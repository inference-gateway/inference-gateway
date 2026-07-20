package api

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"sync"

	gin "github.com/gin-gonic/gin"

	middlewares "github.com/inference-gateway/inference-gateway/api/middlewares"
	config "github.com/inference-gateway/inference-gateway/config"
	mcp "github.com/inference-gateway/inference-gateway/internal/mcp"
	proxymodifier "github.com/inference-gateway/inference-gateway/internal/proxy"
	l "github.com/inference-gateway/inference-gateway/logger"
	otel "github.com/inference-gateway/inference-gateway/otel"
	client "github.com/inference-gateway/inference-gateway/providers/client"
	constants "github.com/inference-gateway/inference-gateway/providers/constants"
	core "github.com/inference-gateway/inference-gateway/providers/core"
	registry "github.com/inference-gateway/inference-gateway/providers/registry"
	routing "github.com/inference-gateway/inference-gateway/providers/routing"
	types "github.com/inference-gateway/inference-gateway/providers/types"
)

//go:generate mockgen -source=routes.go -destination=../tests/mocks/routes.go -package=mocks
type Router interface {
	ListModelsHandler(c *gin.Context)
	ChatCompletionsHandler(c *gin.Context)
	ListToolsHandler(c *gin.Context)
	MetricsIngestionHandler(c *gin.Context)
	ProxyHandler(c *gin.Context)
	HealthcheckHandler(c *gin.Context)
	NotFoundHandler(c *gin.Context)
}

type RouterImpl struct {
	cfg       config.Config
	logger    l.Logger
	registry  registry.ProviderRegistry
	client    client.Client
	mcpClient mcp.MCPClientInterface
	telemetry otel.OpenTelemetry
}

type ErrorResponse struct {
	Error string `json:"error"`
}

type ResponseJSON struct {
	Message string `json:"message"`
}

func NewRouter(
	cfg config.Config,
	logger l.Logger,
	providerRegistry registry.ProviderRegistry,
	httpClient client.Client,
	mcpClient mcp.MCPClientInterface,
	telemetry otel.OpenTelemetry,
) Router {
	return &RouterImpl{
		cfg,
		logger,
		providerRegistry,
		httpClient,
		mcpClient,
		telemetry,
	}
}

func (router *RouterImpl) NotFoundHandler(c *gin.Context) {
	router.logger.Warn("route not found", "path", c.Request.URL.Path, "method", c.Request.Method)
	c.JSON(http.StatusNotFound, ErrorResponse{Error: "Requested route is not found"})
}

func (router *RouterImpl) ProxyHandler(c *gin.Context) {
	p := types.Provider(c.Param("provider"))
	provider, err := router.registry.BuildProvider(p, router.client)
	if err != nil {
		if strings.Contains(err.Error(), "token not configured") {
			router.logger.Error("provider authentication required but api key not configured", err, "provider", p)
			c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Provider requires an API key. Please configure the provider's API key."})
			return
		}
		router.logger.Error("provider not found or not supported", err, "provider", p)
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Provider not found. Please check the list of supported providers."})
		return
	}

	// Setup authentication headers or query params
	token := provider.GetToken()
	switch provider.GetAuthType() {
	case constants.AuthTypeBearer:
		c.Request.Header.Set("Authorization", "Bearer "+token)
	case constants.AuthTypeXheader:
		c.Request.Header.Set("x-api-key", token)
	case constants.AuthTypeQuery:
		query := c.Request.URL.Query()
		query.Set("key", token)
		c.Request.URL.RawQuery = query.Encode()
	case constants.AuthTypeNone:
		// Do Nothing
	default:
		c.JSON(http.StatusUnprocessableEntity, ErrorResponse{Error: "Unsupported auth type"})
		return
	}

	// Add extra headers
	for key, values := range provider.GetExtraHeaders() {
		for _, value := range values {
			c.Request.Header.Add(key, value)
		}
	}

	// Check if streaming is requested
	isStreaming := c.Request.Header.Get("Accept") == "text/event-stream" || c.Request.Header.Get("Content-Type") == "text/event-stream"

	if isStreaming {
		handleStreamingRequest(c, provider, router)
		return
	}

	// Non-streaming case: Setup reverse proxy
	handleProxyRequest(c, provider, router)
}

func handleStreamingRequest(c *gin.Context, provider core.IProvider, router *RouterImpl) {
	middlewares.SetSSEHeaders(c)

	fullURL, err := constructProviderURL(provider, c.Param("path"), c.Request.URL.RawQuery)
	if err != nil {
		router.logger.Error("failed to construct provider url", err, "provider", provider.GetName())
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Failed to construct URL"})
		return
	}

	// Read request body with a 10MB size limit for now, to prevent abuse
	// Will make it configurable later perhaps as a middleware
	const maxBodySize = 10 << 20
	body, err := io.ReadAll(io.LimitReader(c.Request.Body, maxBodySize))
	if err != nil {
		router.logger.Error("failed to read request body", err, "maxBodySize", maxBodySize)
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Failed to read request"})
		return
	}
	if len(body) >= int(maxBodySize) {
		c.JSON(http.StatusRequestEntityTooLarge, ErrorResponse{Error: "Request body too large"})
		return
	}

	ctx := c.Request.Context()
	upstreamReq, err := http.NewRequestWithContext(ctx, c.Request.Method, fullURL.String(), bytes.NewReader(body))
	if err != nil {
		router.logger.Error("failed to create upstream request", err, "method", c.Request.Method, "url", fullURL.String())
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "Failed to create upstream request"})
		return
	}

	upstreamReq.Header = c.Request.Header.Clone()

	resp, err := router.client.Do(upstreamReq)
	if err != nil {
		router.logger.Error("failed to make upstream request", err, "url", fullURL.String())
		c.JSON(http.StatusBadGateway, ErrorResponse{Error: "Failed to reach upstream server"})
		return
	}
	defer resp.Body.Close()

	reader := bufio.NewReaderSize(resp.Body, 4096)

	c.Stream(func(w io.Writer) bool {
		select {
		case <-ctx.Done():
			return false
		default:
		}

		middlewares.ResetWriteDeadline(c, router.cfg.Server.WriteTimeout)

		line, err := reader.ReadBytes('\n')
		if err != nil {
			if err != io.EOF {
				router.logger.Error("failed to read stream", err,
					"url", fullURL.String(),
					"method", c.Request.Method)
			}
			return false
		}

		if len(line) == 0 {
			return true
		}

		if router.cfg.Environment == "development" {
			shouldLog := len(line) > 512 ||
				(c.Param("provider") != "" && len(line) > 0 && (len(line)%10 == 0))

			if shouldLog {
				router.logger.Debug("stream chunk",
					"provider", c.Param("provider"),
					"bytes", len(line),
					"data_preview", func() string {
						preview := string(bytes.TrimSpace(line))
						if len(preview) > 200 {
							return preview[:200] + "... (truncated)"
						}
						return preview
					}(),
				)
			}
		}

		if _, err := w.Write(line); err != nil {
			router.logger.Error("failed to write response", err,
				"bytes", len(line))
			return false
		}

		if flusher, ok := w.(http.Flusher); ok {
			flusher.Flush()
		}

		return true
	})
}

func handleProxyRequest(c *gin.Context, provider core.IProvider, router *RouterImpl) {
	fullURL, err := constructProviderURL(provider, c.Param("path"), c.Request.URL.RawQuery)
	if err != nil {
		router.logger.Error("failed to construct provider url", err, "provider", provider.GetName())
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Failed to construct URL"})
		return
	}
	proxy := &httputil.ReverseProxy{}

	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		router.logger.Error("proxy request failed", err, "url", fullURL.String())
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadGateway)
		err = json.NewEncoder(w).Encode(ErrorResponse{
			Error: fmt.Sprintf("Failed to reach upstream server: %v", err),
		})
		if err != nil {
			router.logger.Error("failed to write error response", err)
		}
	}

	proxy.Rewrite = func(pr *httputil.ProxyRequest) {
		pr.SetURL(fullURL)
		pr.Out.URL.Path = fullURL.Path
		pr.Out.URL.RawQuery = fullURL.RawQuery
		pr.Out.Header = pr.In.Header.Clone()
		pr.Out.Header.Set("Content-Type", "application/json")
		pr.Out.Header.Set("Accept", "application/json")

		if router.cfg.Environment == "development" {
			reqModifier := proxymodifier.NewDevRequestModifier(router.logger, &router.cfg)
			if err := reqModifier.Modify(pr.Out); err != nil {
				router.logger.Error("failed to modify request", err)
				return
			}
		}
	}

	if router.cfg.Environment == "development" {
		devModifier := proxymodifier.NewDevResponseModifier(router.logger)
		proxy.ModifyResponse = devModifier.Modify
	}

	proxy.ServeHTTP(&middlewares.DeadlineResetWriter{ResponseWriter: c.Writer, Timeout: router.cfg.Server.WriteTimeout}, c.Request)
}

// constructProviderURL builds the provider URL consistently to avoid path duplication.
// It ensures that the path from the provider URL is handled correctly with the path parameter.
func constructProviderURL(provider core.IProvider, pathParam, rawQuery string) (*url.URL, error) {
	providerURL, err := url.Parse(provider.GetURL())
	if err != nil {
		return nil, err
	}

	url := &url.URL{
		Scheme:   providerURL.Scheme,
		Host:     providerURL.Host,
		Path:     strings.TrimSuffix(providerURL.Path, "/") + "/" + strings.TrimPrefix(pathParam, "/"),
		RawQuery: rawQuery,
	}

	return url, nil
}

func (router *RouterImpl) HealthcheckHandler(c *gin.Context) {
	router.logger.Debug("healthcheck")
	c.JSON(http.StatusOK, ResponseJSON{Message: "OK"})
}

// parseIncludeParam splits the comma-separated `include` value into a
// de-duplicated list of known metadata keys, preserving first-seen order. An
// empty value yields no keys; an unrecognized key is rejected so typos fail
// loudly instead of silently returning less data. Validity is checked against
// the generated ListModelsParamsInclude enum, keeping the accepted set in sync
// with openapi.yaml as new metadata fields are added.
func parseIncludeParam(raw string) ([]string, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, nil
	}

	seen := make(map[string]struct{})
	keys := make([]string, 0)
	for _, part := range strings.Split(raw, ",") {
		key := strings.TrimSpace(part)
		if key == "" {
			continue
		}
		if !types.ListModelsParamsInclude(key).Valid() {
			return nil, fmt.Errorf("unknown include value %q", key)
		}
		if _, dup := seen[key]; dup {
			continue
		}
		seen[key] = struct{}{}
		keys = append(keys, key)
	}
	return keys, nil
}

// renderModelsResponse writes the models list as JSON. When include keys are
// present it injects each requested key as an explicit null on every model
// unless already populated, keeping the requested-but-unavailable state
// distinguishable from an absent field. With no include keys the typed response
// is written unchanged so the default payload stays byte-for-byte
// OpenAI-compatible.
func (router *RouterImpl) renderModelsResponse(c *gin.Context, resp types.ListModelsResponse, includeKeys []string) {
	if len(includeKeys) == 0 {
		c.JSON(http.StatusOK, resp)
		return
	}

	raw, err := json.Marshal(resp)
	if err != nil {
		router.logger.Error("failed to marshal models response", err)
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "Failed to encode response"})
		return
	}

	var envelope map[string]any
	if err := json.Unmarshal(raw, &envelope); err != nil {
		router.logger.Error("failed to decode models response", err)
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "Failed to encode response"})
		return
	}

	if data, ok := envelope["data"].([]any); ok {
		for _, item := range data {
			model, ok := item.(map[string]any)
			if !ok {
				continue
			}
			for _, key := range includeKeys {
				if _, exists := model[key]; !exists {
					model[key] = nil
				}
			}
		}
	}

	c.JSON(http.StatusOK, envelope)
}

// ListModelsHandler implements an OpenAI-compatible API endpoint
// that returns model information in the standard OpenAI format.
//
// This handler supports the OpenAI GET /v1/models endpoint specification:
// https://platform.openai.com/docs/api-reference/models/list
//
// Parameters:
//   - provider (query): Optional. When specified, returns models from only that provider.
//     If not specified, returns models from all configured providers.
//   - include (query): Optional. Comma-separated list of extra per-model metadata
//     fields to include (context_window, pricing). Keys are trimmed and
//     de-duplicated; an unknown key returns 400. Requested-but-unresolved keys are
//     returned as explicit null. When omitted, no metadata fields are added.
//
// Response format:
//
//	{
//	  "object": "list",
//	  "data": [
//	   {
//	      "id": "model-id",
//	      "object": "model",
//	      "created": 1686935002,
//	      "owned_by": "provider-name",
//	      "served_by": "provider-name"
//	   },
//	   ...
//	  ]
//	}
//
// This endpoint allows applications built for OpenAI's API to work seamlessly
// with the Inference Gateway's multi-provider architecture.
func (router *RouterImpl) ListModelsHandler(c *gin.Context) {
	includeKeys, err := parseIncludeParam(c.Query("include"))
	if err != nil {
		router.logger.Error("invalid include parameter", err)
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		return
	}

	providerID := types.Provider(c.Query("provider"))
	if providerID != "" {
		provider, err := router.registry.BuildProvider(providerID, router.client)
		if err != nil {
			if strings.Contains(err.Error(), "token not configured") {
				router.logger.Error("provider authentication required but api key not configured", err, "provider", providerID)
				c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Provider requires an API key. Please configure the provider's API key."})
				return
			}
			router.logger.Error("provider not found or not supported", err, "provider", providerID)
			c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Provider not found. Please check the list of supported providers."})
			return
		}

		ctx, cancel := context.WithTimeout(c.Request.Context(), router.cfg.Server.ReadTimeout)
		defer cancel()

		response, err := provider.ListModels(ctx)
		if err != nil {
			if ctx.Err() == context.DeadlineExceeded {
				router.logger.Error("request timed out", err, "provider", provider.GetName())
				c.JSON(http.StatusGatewayTimeout, ErrorResponse{Error: "Request timed out"})
				return
			}
			router.logger.Error("failed to list models", err, "provider", provider.GetName())
			c.JSON(http.StatusBadGateway, ErrorResponse{Error: "Failed to list models"})
			return
		}

		response.Data = routing.FilterModels(response.Data, router.cfg.AllowedModels, router.cfg.DisallowedModels)

		router.renderModelsResponse(c, response, includeKeys)
	} else {
		var wg sync.WaitGroup
		providersCfg := router.cfg.Providers

		ch := make(chan types.ListModelsResponse, len(providersCfg))

		ctx, cancel := context.WithTimeout(c.Request.Context(), router.cfg.Server.ReadTimeout)
		defer cancel()

		for providerID := range providersCfg {
			wg.Add(1)
			go func(id types.Provider) {
				defer wg.Done()

				provider, err := router.registry.BuildProvider(id, router.client)
				if err != nil {
					router.logger.Error("failed to create provider", err, "provider", id)
					return
				}

				response, err := provider.ListModels(ctx)
				if err != nil {
					if ctx.Err() == context.DeadlineExceeded {
						router.logger.Error("request timed out", err, "provider", id)
						return
					}
					router.logger.Error("failed to list models", err, "provider", id)
					return
				}

				if response.Data == nil {
					response.Data = make([]types.Model, 0)
				}
				ch <- response
			}(providerID)
		}

		wg.Wait()
		close(ch)

		var allModels []types.Model
		for response := range ch {
			allModels = append(allModels, response.Data...)
		}

		if allModels == nil {
			allModels = make([]types.Model, 0)
		}

		allModels = routing.FilterModels(allModels, router.cfg.AllowedModels, router.cfg.DisallowedModels)

		unifiedResponse := types.ListModelsResponse{
			Object: "list",
			Data:   allModels,
		}

		router.renderModelsResponse(c, unifiedResponse, includeKeys)
	}
}

// ChatCompletionsHandler implements an OpenAI-compatible API endpoint
// that generates text completions in the standard OpenAI format.
//
// Regular response format:
//
//	{
//	  "choices": [
//	    {
//	      "finish_reason": "stop",
//	      "message": {
//	        "content": "Hello, how can I help you today?",
//	        "role": "assistant"
//	      }
//	    }
//	  ],
//	  "created": 1742165657,
//	  "id": "chatcmpl-118",
//	  "model": "deepseek-r1:1.5b",
//	  "object": "chat.completion",
//	  "usage": {
//	    "completion_tokens": 139,
//	    "prompt_tokens": 10,
//	    "total_tokens": 149
//	  }
//	}
//
// Streaming response format:
//
//	{
//	  "choices": [
//	    {
//	      "index": 0,
//	      "finish_reason": "stop",
//	      "delta": {
//	        "content": "Hello",
//	        "role": "assistant"
//	      }
//	    }
//	  ],
//	  "created": 1742165657,
//	  "id": "chatcmpl-118",
//	  "model": "deepseek-r1:1.5b",
//	  "object": "chat.completion.chunk",
//	  "usage": {
//	    "completion_tokens": 139,
//	    "prompt_tokens": 10,
//	    "total_tokens": 149
//	  }
//	}
//
// It returns token completions as chat in the standard OpenAI format, allowing applications
// built for OpenAI's API to work seamlessly with the Inference Gateway's multi-provider
// architecture.
func (router *RouterImpl) ChatCompletionsHandler(c *gin.Context) {
	var req types.CreateChatCompletionRequest

	if mcpRequest, exists := c.Get(middlewares.MCPBypassHeader); exists {
		if parsedRequest, ok := mcpRequest.(*types.CreateChatCompletionRequest); ok {
			req = *parsedRequest
		} else {
			router.logger.Error("invalid mcp request type in context", nil)
			c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "Internal server error"})
			return
		}
	} else {
		if err := c.ShouldBindJSON(&req); err != nil {
			router.logger.Error("failed to decode request", err)
			c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Failed to decode request"})
			return
		}
	}

	model := req.Model
	originalModel := req.Model
	providerID := types.Provider(c.Query("provider"))
	if providerID == "" {
		var providerPtr *types.Provider
		providerPtr, model = routing.DetermineProviderAndModelName(model)
		if providerPtr == nil {
			router.logger.Error("unable to determine provider for model", nil, "model", req.Model)
			c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Unable to determine provider for model. Please specify a provider using the ?provider= query parameter or use the provider/model format (e.g., openai/gpt-4)."})
			return
		}
		providerID = *providerPtr
	}
	req.Model = model

	if allowed := routing.ParseModelSet(router.cfg.AllowedModels); len(allowed) > 0 {
		if !routing.ModelMatches(allowed, originalModel) {
			router.logger.Error("model not in allowed list", nil, "model", originalModel, "allowed_models", router.cfg.AllowedModels)
			c.JSON(http.StatusForbidden, ErrorResponse{Error: "Model not allowed. Please check the list of allowed models."})
			return
		}
	} else if disallowed := routing.ParseModelSet(router.cfg.DisallowedModels); len(disallowed) > 0 {
		if routing.ModelMatches(disallowed, originalModel) {
			router.logger.Error("model is disallowed", nil, "model", originalModel, "disallowed_models", router.cfg.DisallowedModels)
			c.JSON(http.StatusForbidden, ErrorResponse{Error: "Model is disallowed. Please use a different model."})
			return
		}
	}

	provider, err := router.registry.BuildProvider(providerID, router.client)
	if err != nil {
		if strings.Contains(err.Error(), "token not configured") {
			router.logger.Error("provider requires authentication but no api key was configured", err, "provider", providerID)
			c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Provider requires an API key. Please configure the provider's API key."})
			return
		}
		router.logger.Error("provider not found or not supported", err, "provider", providerID)
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Provider not found. Please check the list of supported providers."})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), router.cfg.Server.ReadTimeout)
	defer cancel()

	if router.cfg.EnableVision {
		hasImageContent := false
		imageCount := 0
		for _, message := range req.Messages {
			if message.HasImageContent() {
				hasImageContent = true
				imageCount++
			}
		}

		if hasImageContent {
			supportsVision, err := provider.SupportsVision(ctx, req.Model)
			if err != nil {
				router.logger.Error("failed to check vision support", err, "provider", providerID, "model", req.Model)
				c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "Failed to check model capabilities"})
				return
			}
			if !supportsVision {
				router.logger.Info("filtering images from non-vision model request",
					"provider", providerID,
					"model", req.Model,
					"messagesWithImages", imageCount)

				for i := range req.Messages {
					if req.Messages[i].HasImageContent() {
						if err := req.Messages[i].StripImageContent(); err != nil {
							router.logger.Error("failed to strip image content from message", err)
							c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to process message content"})
							return
						}
					}
				}

				router.logger.Debug("images stripped from request, continuing with text-only content")
			}
		}
	}

	router.logger.Debug("server read timeout", "timeout", router.cfg.Server.ReadTimeout)

	if req.Stream != nil && *req.Stream {
		middlewares.SetSSEHeaders(c)

		streamCtx := c.Request.Context()
		streamCh, err := provider.StreamChatCompletions(streamCtx, req)
		if err != nil {
			router.logger.Error("failed to start streaming", err, "provider", providerID)

			statusCode := http.StatusBadRequest
			if httpErr, ok := err.(*core.HTTPError); ok {
				statusCode = httpErr.StatusCode
			}

			c.JSON(statusCode, ErrorResponse{Error: err.Error()})
			return
		}

		c.Stream(func(w io.Writer) bool {
			select {
			case line, ok := <-streamCh:
				if !ok {
					router.logger.Debug("stream closed", "provider", providerID)
					return false
				}

				middlewares.ResetWriteDeadline(c, router.cfg.Server.WriteTimeout)

				router.logger.Debug("stream chunk",
					"provider", providerID,
					"bytes", len(line),
					"line", string(line))

				if _, err := w.Write(line); err != nil {
					router.logger.Error("failed to write chunk", err)
					return false
				}

				if flusher, ok := w.(http.Flusher); ok {
					flusher.Flush()
				}
				return true
			case <-streamCtx.Done():
				return false
			}
		})
		return
	}

	c.Header("Content-Type", "application/json")
	response, err := provider.ChatCompletions(ctx, req)
	if err != nil {
		if err == context.DeadlineExceeded || ctx.Err() == context.DeadlineExceeded {
			router.logger.Error("request timed out", err, "provider", providerID)
			c.JSON(http.StatusGatewayTimeout, ErrorResponse{Error: "Request timed out"})
			return
		}
		router.logger.Error("failed to generate tokens", err, "provider", providerID)

		statusCode := http.StatusBadRequest
		if httpErr, ok := err.(*core.HTTPError); ok {
			statusCode = httpErr.StatusCode
		}

		c.JSON(statusCode, ErrorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response)
}

// ListToolsHandler implements an endpoint that returns available MCP tools
// when EXPOSE_MCP environment variable is enabled.
//
// Response format when MCP is exposed:
//
//	{
//	  "object": "list",
//	  "data": [
//	    {
//	      "name": "read_file",
//	      "description": "Read the contents of a file",
//	      "server": "filesystem-server",
//	      "input_schema": {...}
//	    },
//	    ...
//	  ]
//	}
//
// Response when MCP is not exposed:
//
//	{
//	  "error": "MCP tools endpoint is not exposed"
//	}
func (router *RouterImpl) ListToolsHandler(c *gin.Context) {
	if !router.cfg.MCP.Expose {
		router.logger.Error("mcp tools endpoint access attempted but not exposed", nil)
		c.JSON(http.StatusForbidden, ErrorResponse{Error: "mcp tools endpoint is not exposed"})
		return
	}

	var allTools []types.MCPTool

	switch {
	case router.mcpClient == nil:
		router.logger.Debug("mcp client is nil, returning empty tools list")
		allTools = make([]types.MCPTool, 0)
	case !router.mcpClient.IsInitialized():
		router.logger.Info("mcp client not initialized, no tools available")
		allTools = make([]types.MCPTool, 0)
	default:
		servers := router.mcpClient.GetServers()

		for _, serverURL := range servers {
			tools, err := router.mcpClient.GetServerTools(serverURL)
			if err != nil {
				router.logger.Error("failed to get tools from mcp server", err, "server", serverURL)
				continue
			}

			for _, tool := range tools {
				mcpTool := types.MCPTool{
					Name:        "mcp_" + tool.Name,
					Description: *tool.Description,
					Server:      serverURL,
					InputSchema: &tool.InputSchema,
				}
				allTools = append(allTools, mcpTool)
			}
		}

		if allTools == nil {
			allTools = make([]types.MCPTool, 0)
		}
	}

	response := types.ListToolsResponse{
		Object: "list",
		Data:   allTools,
	}

	c.JSON(http.StatusOK, response)
}
