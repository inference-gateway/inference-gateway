package api

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httputil"
	"net/textproto"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"sync"

	gin "github.com/gin-gonic/gin"
	otelhttp "go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	otelapi "go.opentelemetry.io/otel"
	codes "go.opentelemetry.io/otel/codes"
	propagation "go.opentelemetry.io/otel/propagation"
	semconv "go.opentelemetry.io/otel/semconv/v1.41.0"
	trace "go.opentelemetry.io/otel/trace"

	middlewares "github.com/inference-gateway/inference-gateway/api/middlewares"
	config "github.com/inference-gateway/inference-gateway/config"
	mcp "github.com/inference-gateway/inference-gateway/internal/mcp"
	proxymodifier "github.com/inference-gateway/inference-gateway/internal/proxy"
	tts "github.com/inference-gateway/inference-gateway/internal/tts"
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
	MessagesHandler(c *gin.Context)
	ResponsesHandler(c *gin.Context)
	ImagesHandler(c *gin.Context)
	ImagesEditsHandler(c *gin.Context)
	ImagesVariationsHandler(c *gin.Context)
	SpeechHandler(c *gin.Context)
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
	selector  *routing.Selector
	tts       *tts.Engine
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
	selector *routing.Selector,
	localTTS ...*tts.Engine,
) Router {
	var ttsEngine *tts.Engine
	if len(localTTS) > 0 {
		ttsEngine = localTTS[0]
	}
	return &RouterImpl{
		cfg,
		logger,
		providerRegistry,
		httpClient,
		mcpClient,
		telemetry,
		selector,
		ttsEngine,
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

	if err := applyProviderAuth(c.Request, provider); err != nil {
		c.JSON(http.StatusUnprocessableEntity, ErrorResponse{Error: "Unsupported auth type"})
		return
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

	body, tooLarge, err := router.readBoundedBody(c)
	if err != nil {
		router.logger.Error("failed to read request body", err)
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Failed to read request"})
		return
	}
	if tooLarge {
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
	otelapi.GetTextMapPropagator().Inject(ctx, propagation.HeaderCarrier(upstreamReq.Header))

	resp, err := router.client.Do(upstreamReq)
	if err != nil {
		router.logger.Error("failed to make upstream request", err, "url", fullURL.String())
		c.JSON(http.StatusBadGateway, ErrorResponse{Error: "Failed to reach upstream server"})
		return
	}
	defer resp.Body.Close()

	reader := bufio.NewReaderSize(resp.Body, 4096)

	c.Stream(func(w io.Writer) bool {
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
	proxy := &httputil.ReverseProxy{
		Transport: proxyTransport,
	}

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
		otelapi.GetTextMapPropagator().Inject(pr.Out.Context(), propagation.HeaderCarrier(pr.Out.Header))

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

// applyProviderAuth sets the provider's auth credential (header or query
// param) and extra headers on req. An unrecognized auth type is returned as an
// error so misconfigured providers fail loudly instead of sending
// unauthenticated requests upstream.
//
// The caller's inbound Authorization header is always removed first so it is
// never forwarded to the upstream provider: the client authenticates to the
// gateway (and the gateway's self-proxy hop carries that token so OIDC can
// re-verify /proxy), but only the provider's own credential must leave the
// gateway. Bearer providers overwrite the header below; the others (x-api-key,
// query key, none) authenticate elsewhere, so without this removal the caller's
// bearer/OIDC token would leak to third-party providers.
func applyProviderAuth(req *http.Request, provider core.IProvider) error {
	req.Header.Del("Authorization")

	token := provider.GetToken()
	switch provider.GetAuthType() {
	case constants.AuthTypeBearer:
		req.Header.Set("Authorization", "Bearer "+token)
	case constants.AuthTypeXheader:
		req.Header.Set("x-api-key", token)
	case constants.AuthTypeQuery:
		query := req.URL.Query()
		query.Set("key", token)
		req.URL.RawQuery = query.Encode()
	case constants.AuthTypeNone:
		// Do Nothing
	default:
		return fmt.Errorf("unsupported auth type %q", provider.GetAuthType())
	}

	for key, values := range provider.GetExtraHeaders() {
		for _, value := range values {
			req.Header.Add(key, value)
		}
	}
	return nil
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
	if !slices.Contains(includeKeys, string(types.ListModelsParamsIncludeContextWindow)) {
		for i := range resp.Data {
			resp.Data[i].ContextWindow = nil
		}
	}
	if !slices.Contains(includeKeys, string(types.ListModelsParamsIncludePricing)) {
		for i := range resp.Data {
			resp.Data[i].Pricing = nil
		}
	}
	if !slices.Contains(includeKeys, string(types.ListModelsParamsIncludeModalities)) {
		for i := range resp.Data {
			resp.Data[i].Modalities = nil
		}
	}

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

		if slices.Contains(includeKeys, string(types.ListModelsParamsIncludeContextWindow)) {
			router.resolveContextWindows(ctx, response.Data)
		}

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

		if slices.Contains(includeKeys, string(types.ListModelsParamsIncludeContextWindow)) {
			router.resolveContextWindows(ctx, allModels)
		}

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
// modelDenied reports whether model is blocked by ALLOWED_MODELS /
// DISALLOWED_MODELS, returning the client-facing reason ("" when permitted).
// ALLOWED_MODELS takes precedence: when it is set, DISALLOWED_MODELS is ignored.
// readBoundedBody reads the request body up to the configured max request
// body size, reporting tooLarge when the body exceeds the limit. A body of
// exactly the limit is accepted.
func (router *RouterImpl) readBoundedBody(c *gin.Context) (body []byte, tooLarge bool, err error) {
	maxBodySize := router.cfg.Server.ResolveMaxRequestBodySize()
	body, err = io.ReadAll(io.LimitReader(c.Request.Body, int64(maxBodySize)+1))
	if err != nil {
		return nil, false, err
	}
	return body, len(body) > maxBodySize, nil
}

func (router *RouterImpl) modelDenied(model string) string {
	if allowed := routing.ParseModelSet(router.cfg.AllowedModels); len(allowed) > 0 {
		if !routing.ModelMatches(allowed, model) {
			router.logger.Error("model not in allowed list", nil, "model", model, "allowed_models", router.cfg.AllowedModels)
			return "Model not allowed. Please check the list of allowed models."
		}
		return ""
	}
	if disallowed := routing.ParseModelSet(router.cfg.DisallowedModels); len(disallowed) > 0 && routing.ModelMatches(disallowed, model) {
		router.logger.Error("model is disallowed", nil, "model", model, "disallowed_models", router.cfg.DisallowedModels)
		return "Model is disallowed. Please use a different model."
	}
	return ""
}

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
		maxBodySize := router.cfg.Server.ResolveMaxRequestBodySize()
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, int64(maxBodySize))
		if err := c.ShouldBindJSON(&req); err != nil {
			var maxBytesErr *http.MaxBytesError
			if errors.As(err, &maxBytesErr) {
				router.logger.Error("request body too large", err)
				c.JSON(http.StatusRequestEntityTooLarge, ErrorResponse{Error: "Request body too large"})
				return
			}
			router.logger.Error("failed to decode request", err)
			c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Failed to decode request"})
			return
		}
	}

	model := req.Model
	originalModel := req.Model
	providerID := types.Provider(c.Query("provider"))

	var routedProvider, routedModel string
	if router.selector != nil && providerID == "" {
		if dep, ok := router.selector.Select(model); ok {
			providerID = types.Provider(dep.Provider)
			model = dep.Model
			routedProvider, routedModel = dep.Provider, dep.Model
			router.logger.Debug("routed logical model", "alias", originalModel, "provider", dep.Provider, "model", dep.Model)
		}
	}

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

	if reason := router.modelDenied(originalModel); reason != "" {
		c.JSON(http.StatusForbidden, ErrorResponse{Error: reason})
		return
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
			if !core.ModelAcceptsImages(providerID, req.Model) {
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

	if routedProvider != "" {
		c.Header("X-Selected-Provider", routedProvider)
		c.Header("X-Selected-Model", routedModel)
	}

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

// messagesError writes a gateway-generated error in the Anthropic error
// envelope ({"type": "error", "error": {"type": ..., "message": ...}}), which
// is what native Messages API clients expect to parse.
func messagesError(c *gin.Context, status int, errType, message string) {
	resp := types.MessagesError{Type: types.MessagesErrorTypeError}
	resp.Error.Type = errType
	resp.Error.Message = message
	c.JSON(status, resp)
}

// MessagesHandler implements an Anthropic-compatible POST /v1/messages
// endpoint: https://docs.anthropic.com/en/api/messages
//
// The request body is forwarded to the upstream provider byte-for-byte (only
// the `model` field is rewritten when the provider prefix is stripped), so
// `cache_control` breakpoints and any future Anthropic request fields pass
// through untouched, and the upstream response - including
// `cache_creation_input_tokens` / `cache_read_input_tokens` usage and the
// Anthropic SSE event envelope when streaming - is relayed verbatim.
//
// Only providers that natively implement the Messages API are supported
// (currently Anthropic); other providers receive a 400 in the Anthropic error
// envelope, mirroring the schema's MessagesNotSupported response.
func (router *RouterImpl) MessagesHandler(c *gin.Context) {
	body, tooLarge, err := router.readBoundedBody(c)
	if err != nil {
		router.logger.Error("failed to read request body", err)
		messagesError(c, http.StatusBadRequest, "invalid_request_error", "Failed to read request")
		return
	}
	if tooLarge {
		messagesError(c, http.StatusRequestEntityTooLarge, "invalid_request_error", "Request body too large")
		return
	}

	var req struct {
		Model  string `json:"model"`
		Stream *bool  `json:"stream"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		router.logger.Error("failed to decode request", err)
		messagesError(c, http.StatusBadRequest, "invalid_request_error", "Failed to decode request")
		return
	}

	originalModel := req.Model
	model := req.Model
	providerID := types.Provider(c.Query("provider"))
	if providerID == "" {
		var providerPtr *types.Provider
		providerPtr, model = routing.DetermineProviderAndModelName(model)
		if providerPtr == nil {
			router.logger.Error("unable to determine provider for model", nil, "model", originalModel)
			messagesError(c, http.StatusBadRequest, "invalid_request_error", "Unable to determine provider for model. Please specify a provider using the ?provider= query parameter or use the provider/model format (e.g., anthropic/claude-sonnet-4-5).")
			return
		}
		providerID = *providerPtr
	}

	span := trace.SpanFromContext(c.Request.Context())
	span.SetAttributes(
		semconv.GenAIProviderNameKey.String(string(providerID)),
		semconv.GenAIRequestModel(originalModel),
	)

	if reason := router.modelDenied(originalModel); reason != "" {
		messagesError(c, http.StatusForbidden, "invalid_request_error", reason)
		return
	}

	if providerID != constants.AnthropicID {
		router.logger.Error("messages api not supported by provider", nil, "provider", providerID)
		messagesError(c, http.StatusBadRequest, "not_supported_error", "The Messages API is not supported by this provider yet.")
		return
	}

	provider, err := router.registry.BuildProvider(providerID, router.client)
	if err != nil {
		if strings.Contains(err.Error(), "token not configured") {
			router.logger.Error("provider requires authentication but no api key was configured", err, "provider", providerID)
			messagesError(c, http.StatusBadRequest, "invalid_request_error", "Provider requires an API key. Please configure the provider's API key.")
			return
		}
		router.logger.Error("provider not found or not supported", err, "provider", providerID)
		messagesError(c, http.StatusBadRequest, "invalid_request_error", "Provider not found. Please check the list of supported providers.")
		return
	}

	if model != originalModel {
		dec := json.NewDecoder(bytes.NewReader(body))
		dec.UseNumber()
		var payload map[string]any
		if err := dec.Decode(&payload); err != nil {
			router.logger.Error("failed to decode request", err)
			messagesError(c, http.StatusBadRequest, "invalid_request_error", "Failed to decode request")
			return
		}
		payload["model"] = model
		if body, err = json.Marshal(payload); err != nil {
			router.logger.Error("failed to encode request", err)
			messagesError(c, http.StatusInternalServerError, "api_error", "Failed to encode request")
			return
		}
	}

	isStreaming := req.Stream != nil && *req.Stream

	ctx := c.Request.Context()
	if !isStreaming {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, router.cfg.Server.ReadTimeout)
		defer cancel()
	}

	upstreamURL := strings.TrimSuffix(provider.GetURL(), "/") + "/messages"
	upstreamReq, err := http.NewRequestWithContext(ctx, http.MethodPost, upstreamURL, bytes.NewReader(body))
	if err != nil {
		router.logger.Error("failed to create upstream request", err, "url", upstreamURL)
		messagesError(c, http.StatusInternalServerError, "api_error", "Failed to create upstream request")
		return
	}
	upstreamReq.Header.Set("Content-Type", "application/json")
	if isStreaming {
		upstreamReq.Header.Set("Accept", "text/event-stream")
	} else {
		upstreamReq.Header.Set("Accept", "application/json")
	}

	if err := applyProviderAuth(upstreamReq, provider); err != nil {
		router.logger.Error("unsupported auth type", err, "provider", providerID)
		messagesError(c, http.StatusUnprocessableEntity, "api_error", "Unsupported auth type")
		return
	}

	otelapi.GetTextMapPropagator().Inject(ctx, propagation.HeaderCarrier(upstreamReq.Header))

	resp, err := router.client.Do(upstreamReq)
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			router.logger.Error("request timed out", err, "provider", providerID)
			messagesError(c, http.StatusGatewayTimeout, "api_error", "Request timed out")
			return
		}
		router.logger.Error("failed to reach upstream server", err, "url", upstreamURL)
		messagesError(c, http.StatusBadGateway, "api_error", "Failed to reach upstream server")
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		span.SetStatus(codes.Error, resp.Status)
		span.SetAttributes(semconv.ErrorTypeKey.String(strconv.Itoa(resp.StatusCode)))
	}

	contentType := resp.Header.Get("Content-Type")
	if !strings.HasPrefix(contentType, "text/event-stream") {
		c.DataFromReader(resp.StatusCode, resp.ContentLength, contentType, resp.Body, nil)
		return
	}

	middlewares.SetSSEHeaders(c)
	reader := bufio.NewReaderSize(resp.Body, 4096)
	c.Stream(func(w io.Writer) bool {
		middlewares.ResetWriteDeadline(c, router.cfg.Server.WriteTimeout)

		// The upstream request carries the client's context, so cancellation
		// surfaces here as a read error - no separate ctx.Done() check needed.
		line, err := reader.ReadBytes('\n')
		if len(line) > 0 {
			if _, werr := w.Write(line); werr != nil {
				router.logger.Error("failed to write chunk", werr)
				return false
			}
			if flusher, ok := w.(http.Flusher); ok {
				flusher.Flush()
			}
		}
		if err != nil {
			if err != io.EOF {
				router.logger.Error("failed to read stream", err, "url", upstreamURL)
			}
			return false
		}
		return true
	})
}

// ResponsesHandler implements an OpenAI-compatible POST /v1/responses
// endpoint: https://platform.openai.com/docs/api-reference/responses
//
// The request body is forwarded to the upstream provider byte-for-byte (only
// the `model` field is rewritten when the provider prefix is stripped), so
// all Responses API fields pass through untouched, and the upstream response
// - including ResponseStreamEvent frames when streaming - is relayed verbatim.
//
// Only providers that natively implement the Responses API are supported
// (currently OpenAI); other providers receive a 400, mirroring the schema's
// ResponsesNotSupported response.
func (router *RouterImpl) ResponsesHandler(c *gin.Context) {
	body, tooLarge, err := router.readBoundedBody(c)
	if err != nil {
		router.logger.Error("failed to read request body", err)
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Failed to read request"})
		return
	}
	if tooLarge {
		c.JSON(http.StatusRequestEntityTooLarge, ErrorResponse{Error: "Request body too large"})
		return
	}

	var req struct {
		Model  string `json:"model"`
		Stream *bool  `json:"stream"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		router.logger.Error("failed to decode request", err)
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Failed to decode request"})
		return
	}

	originalModel := req.Model
	model := req.Model
	providerID := types.Provider(c.Query("provider"))
	if providerID == "" {
		var providerPtr *types.Provider
		providerPtr, model = routing.DetermineProviderAndModelName(model)
		if providerPtr == nil {
			router.logger.Error("unable to determine provider for model", nil, "model", originalModel)
			c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Unable to determine provider for model. Please specify a provider using the ?provider= query parameter or use the provider/model format (e.g., openai/gpt-4o)."})
			return
		}
		providerID = *providerPtr
	}

	span := trace.SpanFromContext(c.Request.Context())
	span.SetAttributes(
		semconv.GenAIProviderNameKey.String(string(providerID)),
		semconv.GenAIRequestModel(originalModel),
	)

	if reason := router.modelDenied(originalModel); reason != "" {
		c.JSON(http.StatusForbidden, ErrorResponse{Error: reason})
		return
	}

	if providerID != constants.OpenaiID {
		router.logger.Error("responses api not supported by provider", nil, "provider", providerID)
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "The Responses API is not supported by this provider yet. Use /v1/chat/completions instead."})
		return
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

	if model != originalModel {
		dec := json.NewDecoder(bytes.NewReader(body))
		dec.UseNumber()
		var payload map[string]any
		if err := dec.Decode(&payload); err != nil {
			router.logger.Error("failed to decode request", err)
			c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Failed to decode request"})
			return
		}
		payload["model"] = model
		if body, err = json.Marshal(payload); err != nil {
			router.logger.Error("failed to encode request", err)
			c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "Failed to encode request"})
			return
		}
	}

	isStreaming := req.Stream != nil && *req.Stream

	ctx := c.Request.Context()
	if !isStreaming {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, router.cfg.Server.ReadTimeout)
		defer cancel()
	}

	upstreamURL := strings.TrimSuffix(provider.GetURL(), "/") + "/responses"
	upstreamReq, err := http.NewRequestWithContext(ctx, http.MethodPost, upstreamURL, bytes.NewReader(body))
	if err != nil {
		router.logger.Error("failed to create upstream request", err, "url", upstreamURL)
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "Failed to create upstream request"})
		return
	}
	upstreamReq.Header.Set("Content-Type", "application/json")
	if isStreaming {
		upstreamReq.Header.Set("Accept", "text/event-stream")
	} else {
		upstreamReq.Header.Set("Accept", "application/json")
	}

	if err := applyProviderAuth(upstreamReq, provider); err != nil {
		router.logger.Error("unsupported auth type", err, "provider", providerID)
		c.JSON(http.StatusUnprocessableEntity, ErrorResponse{Error: "Unsupported auth type"})
		return
	}

	otelapi.GetTextMapPropagator().Inject(ctx, propagation.HeaderCarrier(upstreamReq.Header))

	resp, err := router.client.Do(upstreamReq)
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			router.logger.Error("request timed out", err, "provider", providerID)
			c.JSON(http.StatusGatewayTimeout, ErrorResponse{Error: "Request timed out"})
			return
		}
		router.logger.Error("failed to reach upstream server", err, "url", upstreamURL)
		c.JSON(http.StatusBadGateway, ErrorResponse{Error: "Failed to reach upstream server"})
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		span.SetStatus(codes.Error, resp.Status)
		span.SetAttributes(semconv.ErrorTypeKey.String(strconv.Itoa(resp.StatusCode)))
	}

	contentType := resp.Header.Get("Content-Type")
	if !strings.HasPrefix(contentType, "text/event-stream") {
		c.DataFromReader(resp.StatusCode, resp.ContentLength, contentType, resp.Body, nil)
		return
	}

	middlewares.SetSSEHeaders(c)
	reader := bufio.NewReaderSize(resp.Body, 4096)
	c.Stream(func(w io.Writer) bool {
		middlewares.ResetWriteDeadline(c, router.cfg.Server.WriteTimeout)

		line, err := reader.ReadBytes('\n')
		if len(line) > 0 {
			if _, werr := w.Write(line); werr != nil {
				router.logger.Error("failed to write chunk", werr)
				return false
			}
			if flusher, ok := w.(http.Flusher); ok {
				flusher.Flush()
			}
		}
		if err != nil {
			if err != io.EOF {
				router.logger.Error("failed to read stream", err, "url", upstreamURL)
			}
			return false
		}
		return true
	})
}

// ImagesHandler implements an OpenAI-compatible POST /v1/images/generations
// endpoint: https://platform.openai.com/docs/api-reference/images/create
//
// The request body is forwarded to the upstream provider byte-for-byte (only
// the `model` field is rewritten when the provider prefix is stripped), so
// all Images API fields pass through untouched.
//
// Only providers that natively implement the Images API are supported
// (currently OpenAI); other providers receive a 400, mirroring the schema's
// ImagesNotSupported response.
//
// The endpoint is opt-in via IMAGES_ENABLED (default off). When disabled, the
// handler returns 404.
func (router *RouterImpl) ImagesHandler(c *gin.Context) {
	if !router.cfg.ImagesEnabled {
		router.logger.Error("images api not enabled", nil)
		c.JSON(http.StatusNotFound, ErrorResponse{Error: "The Images API is not enabled. Set IMAGES_ENABLED=true to enable it."})
		return
	}

	router.proxyJSONBody(c, "Images API", "openai/gpt-image-2", "application/json",
		func(e types.Endpoints) *string { return e.Images })
}

// proxyJSONBody forwards an OpenAI-style JSON request byte-for-byte to the
// provider endpoint selected by endpointOf (only the `model` field is
// rewritten when the provider prefix is stripped) and relays the upstream
// response with its Content-Type. apiName appears in client-facing errors,
// exampleModel in routing hints, and accept sets the Accept header when
// non-empty.
func (router *RouterImpl) proxyJSONBody(c *gin.Context, apiName, exampleModel, accept string, endpointOf func(types.Endpoints) *string) {
	body, tooLarge, err := router.readBoundedBody(c)
	if err != nil {
		router.logger.Error("failed to read request body", err)
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Failed to read request"})
		return
	}
	if tooLarge {
		c.JSON(http.StatusRequestEntityTooLarge, ErrorResponse{Error: "Request body too large"})
		return
	}

	var req struct {
		Model *string `json:"model,omitempty"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		router.logger.Error("failed to decode request", err)
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Failed to decode request"})
		return
	}

	providerID := types.Provider(c.Query("provider"))
	model := ""
	if req.Model != nil {
		model = *req.Model
	}
	originalModel := model

	providerHint := "Please specify a provider using the ?provider= query parameter or use the provider/model format (e.g., " + exampleModel + ")."
	if providerID == "" && model != "" {
		var providerPtr *types.Provider
		providerPtr, model = routing.DetermineProviderAndModelName(model)
		if providerPtr == nil {
			router.logger.Error("unable to determine provider for model", nil, "model", originalModel)
			c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Unable to determine provider for model. " + providerHint})
			return
		}
		providerID = *providerPtr
	}

	if providerID == "" {
		router.logger.Error("no provider specified", nil, "api", apiName)
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "No provider specified. " + providerHint})
		return
	}

	if reason := router.modelDenied(originalModel); reason != "" {
		c.JSON(http.StatusForbidden, ErrorResponse{Error: reason})
		return
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

	endpoint := endpointOf(provider.GetEndpoints())
	if endpoint == nil || *endpoint == "" {
		router.logger.Error("api not supported by provider", nil, "api", apiName, "provider", providerID)
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "The " + apiName + " is not supported by this provider yet."})
		return
	}

	if model != originalModel {
		dec := json.NewDecoder(bytes.NewReader(body))
		dec.UseNumber()
		var payload map[string]any
		if err := dec.Decode(&payload); err != nil {
			router.logger.Error("failed to decode request", err)
			c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Failed to decode request"})
			return
		}
		payload["model"] = model
		if body, err = json.Marshal(payload); err != nil {
			router.logger.Error("failed to encode request", err)
			c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "Failed to encode request"})
			return
		}
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), router.cfg.Server.ReadTimeout)
	defer cancel()

	upstreamURL := strings.TrimSuffix(provider.GetURL(), "/") + *endpoint
	upstreamReq, err := http.NewRequestWithContext(ctx, http.MethodPost, upstreamURL, bytes.NewReader(body))
	if err != nil {
		router.logger.Error("failed to create upstream request", err, "url", upstreamURL)
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "Failed to create upstream request"})
		return
	}
	upstreamReq.Header.Set("Content-Type", "application/json")
	if accept != "" {
		upstreamReq.Header.Set("Accept", accept)
	}

	if err := applyProviderAuth(upstreamReq, provider); err != nil {
		router.logger.Error("unsupported auth type", err, "provider", providerID)
		c.JSON(http.StatusUnprocessableEntity, ErrorResponse{Error: "Unsupported auth type"})
		return
	}

	otelapi.GetTextMapPropagator().Inject(ctx, propagation.HeaderCarrier(upstreamReq.Header))

	resp, err := router.client.Do(upstreamReq)
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			router.logger.Error("request timed out", err, "provider", providerID)
			c.JSON(http.StatusGatewayTimeout, ErrorResponse{Error: "Request timed out"})
			return
		}
		router.logger.Error("failed to reach upstream server", err, "url", upstreamURL)
		c.JSON(http.StatusBadGateway, ErrorResponse{Error: "Failed to reach upstream server"})
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		span := trace.SpanFromContext(c.Request.Context())
		span.SetStatus(codes.Error, resp.Status)
		span.SetAttributes(semconv.ErrorTypeKey.String(strconv.Itoa(resp.StatusCode)))
	}

	c.DataFromReader(resp.StatusCode, resp.ContentLength, resp.Header.Get("Content-Type"), resp.Body, nil)
}

// SpeechHandler implements an OpenAI-compatible POST /v1/audio/speech
// endpoint: https://platform.openai.com/docs/api-reference/audio/createSpeech
//
// The request body is forwarded to the upstream provider byte-for-byte (only
// the `model` field is rewritten when the provider prefix is stripped), so
// all Audio API fields pass through untouched. The synthesized audio comes
// back as raw binary and is streamed to the caller with the upstream's
// Content-Type (e.g. audio/mpeg for response_format mp3).
//
// Only providers that natively implement the Audio API are supported
// (currently openai); other providers receive a 400, mirroring the schema's
// SpeechNotSupported response. The llamacpp registry entry also carries a
// Speech endpoint, but that path is a work in progress and not supported yet.
//
// The endpoint is opt-in via AUDIO_ENABLED (default off). When disabled, the
// handler returns 404.
func (router *RouterImpl) SpeechHandler(c *gin.Context) {
	if !router.cfg.AudioEnabled {
		router.logger.Error("audio api not enabled", nil)
		c.JSON(http.StatusNotFound, ErrorResponse{Error: "The Audio API is not enabled. Set AUDIO_ENABLED=true to enable it."})
		return
	}

	// The reserved local/ prefix is served by the built-in llama-tts engine
	// instead of a provider; everything else is proxied byte-for-byte as before.
	if c.Query("provider") == "" && router.serveLocalSpeech(c) {
		return
	}

	router.proxyJSONBody(c, "Audio API", "openai/tts-1", "",
		func(e types.Endpoints) *string { return e.Speech })
}

// serveLocalSpeech handles the reserved local/ model prefix with the built-in
// llama-tts engine and reports whether the request was handled. The body is
// read once and rewound so the provider proxy path proceeds unchanged.
func (router *RouterImpl) serveLocalSpeech(c *gin.Context) bool {
	if router.tts == nil {
		return false
	}
	body, tooLarge, err := router.readBoundedBody(c)
	if err != nil {
		router.logger.Error("failed to read request body", err)
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Failed to read request"})
		return true
	}
	if tooLarge {
		c.JSON(http.StatusRequestEntityTooLarge, ErrorResponse{Error: "Request body too large"})
		return true
	}

	var probe struct {
		Model string `json:"model"`
	}
	if err := json.Unmarshal(body, &probe); err != nil {
		c.Request.Body = io.NopCloser(bytes.NewReader(body))
		return false // let the provider path report the malformed body
	}
	if !strings.HasPrefix(probe.Model, tts.ModelPrefix) {
		c.Request.Body = io.NopCloser(bytes.NewReader(body))
		return false
	}
	if probe.Model != tts.ReservedModelID {
		router.logger.Error("unknown local speech model", nil, "model", probe.Model)
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: fmt.Sprintf("Unknown local speech model %q. %q is the only local speech model.", probe.Model, tts.ReservedModelID)})
		return true
	}
	if reason := router.modelDenied(probe.Model); reason != "" {
		c.JSON(http.StatusForbidden, ErrorResponse{Error: reason})
		return true
	}

	var req struct {
		Input          string `json:"input"`
		ResponseFormat string `json:"response_format"`
		ReferenceAudio []byte `json:"reference_audio"`
		Language       string `json:"language"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		router.logger.Error("failed to decode request", err)
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Failed to decode request"})
		return true
	}
	if strings.TrimSpace(req.Input) == "" {
		router.logger.Error("local speech request missing input", nil)
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "The 'input' field is required."})
		return true
	}
	if req.ResponseFormat != "" && req.ResponseFormat != "wav" {
		router.logger.Error("unsupported response_format for local engine", nil, "response_format", req.ResponseFormat)
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: `The local speech engine only supports response_format "wav".`})
		return true
	}
	if req.Language != "" && !slices.Contains(tts.SupportedLanguages, req.Language) {
		router.logger.Error("unsupported language for local engine", nil, "language", req.Language)
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: fmt.Sprintf("The local speech engine does not support language %q. Supported languages: %s.", req.Language, strings.Join(tts.SupportedLanguages, ", "))})
		return true
	}

	audio, err := router.tts.Synthesize(c.Request.Context(), tts.Request{
		Input:          req.Input,
		ReferenceAudio: req.ReferenceAudio,
		Language:       req.Language,
	})
	if err != nil {
		var notReady *tts.NotReadyError
		switch {
		case errors.As(err, &notReady):
			router.logger.Warn("local speech assets not ready", nil, "detail", notReady.Error())
			c.Header("Retry-After", strconv.Itoa(tts.RetryAfterSeconds))
			c.JSON(http.StatusServiceUnavailable, ErrorResponse{Error: notReady.Error()})
		case errors.Is(err, context.DeadlineExceeded):
			router.logger.Error("local speech synthesis timed out", err)
			c.JSON(http.StatusGatewayTimeout, ErrorResponse{Error: "Local speech synthesis timed out"})
		default:
			router.logger.Error("local speech synthesis failed", err)
			c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "Failed to synthesize local speech"})
		}
		return true
	}

	c.Data(http.StatusOK, "audio/wav", audio)
	return true
}

// Multipart form field names shared by the /images/edits and
// /images/variations endpoints.
const (
	imageFormFieldImage = "image"
	// imageFormFieldImageArray is the multi-image variant accepted by
	// gpt-image-1 edits.
	imageFormFieldImageArray = "image[]"
	imageFormFieldPrompt     = "prompt"
	imageFormFieldModel      = "model"

	// imagesMultipartMaxMemory caps how much of a multipart upload is kept in
	// memory; parts above it spill to temp files instead.
	imagesMultipartMaxMemory = 1 << 20
)

// imagesMultipartTarget selects which multipart Images endpoint a request is
// forwarded to and whether a prompt is required (edits require one, variations
// do not).
type imagesMultipartTarget struct {
	endpoint      func(types.Endpoints) *string
	requirePrompt bool
}

// proxyTransport wraps http.DefaultTransport with OpenTelemetry instrumentation
// so that every non-streaming reverse proxy call emits a distinct client span.
var proxyTransport = otelhttp.NewTransport(http.DefaultTransport, client.SpanNameFormatter())

var (
	imagesEditsTarget = imagesMultipartTarget{
		endpoint:      func(e types.Endpoints) *string { return e.ImagesEdits },
		requirePrompt: true,
	}
	imagesVariationsTarget = imagesMultipartTarget{
		endpoint:      func(e types.Endpoints) *string { return e.ImagesVariations },
		requirePrompt: false,
	}
)

// ImagesEditsHandler implements POST /v1/images/edits (multipart/form-data).
func (router *RouterImpl) ImagesEditsHandler(c *gin.Context) {
	router.handleImagesMultipart(c, imagesEditsTarget)
}

// ImagesVariationsHandler implements POST /v1/images/variations
// (multipart/form-data).
func (router *RouterImpl) ImagesVariationsHandler(c *gin.Context) {
	router.handleImagesMultipart(c, imagesVariationsTarget)
}

// handleImagesMultipart proxies a multipart Images upload to the resolved
// provider. It parses the form to validate required fields and resolve the
// provider (via ?provider= or the model prefix), then re-encodes the parts and
// streams them to the upstream through an io.Pipe so the binary image/mask
// files are streamed to the upstream without a second full in-memory copy
// (ParseMultipartForm has already spilled anything over 1 MiB to temp files).
//
// Behaviour mirrors ImagesHandler: opt-in via IMAGES_ENABLED (404 when off) and
// only providers that natively implement the endpoint are supported (others
// receive a 400).
func (router *RouterImpl) handleImagesMultipart(c *gin.Context, target imagesMultipartTarget) {
	if !router.cfg.ImagesEnabled {
		router.logger.Error("images api not enabled", nil)
		c.JSON(http.StatusNotFound, ErrorResponse{Error: "The Images API is not enabled. Set IMAGES_ENABLED=true to enable it."})
		return
	}

	maxBodySize := router.cfg.Server.ResolveMaxRequestBodySize()
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, int64(maxBodySize))
	if err := c.Request.ParseMultipartForm(imagesMultipartMaxMemory); err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			c.JSON(http.StatusRequestEntityTooLarge, ErrorResponse{Error: "Request body too large"})
			return
		}
		router.logger.Error("failed to parse multipart form", err)
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Failed to parse multipart/form-data request"})
		return
	}
	defer func() {
		if c.Request.MultipartForm != nil {
			_ = c.Request.MultipartForm.RemoveAll()
		}
	}()

	form := c.Request.MultipartForm

	if len(form.File[imageFormFieldImage])+len(form.File[imageFormFieldImageArray]) == 0 {
		router.logger.Error("images request missing image file", nil)
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "The 'image' file is required."})
		return
	}
	if target.requirePrompt && strings.TrimSpace(imagesFormValue(form, imageFormFieldPrompt)) == "" {
		router.logger.Error("images edit request missing prompt", nil)
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "The 'prompt' field is required."})
		return
	}

	providerID := types.Provider(c.Query("provider"))
	model := imagesFormValue(form, imageFormFieldModel)
	originalModel := model
	if providerID == "" && model != "" {
		var providerPtr *types.Provider
		providerPtr, model = routing.DetermineProviderAndModelName(model)
		if providerPtr == nil {
			router.logger.Error("unable to determine provider for model", nil, "model", originalModel)
			c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Unable to determine provider for model. Please specify a provider using the ?provider= query parameter or use the provider/model format (e.g., openai/gpt-image-2)."})
			return
		}
		providerID = *providerPtr
	}

	if providerID == "" {
		router.logger.Error("no provider specified for images request", nil)
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "No provider specified. Please specify a provider using the ?provider= query parameter or use the provider/model format (e.g., openai/gpt-image-2)."})
		return
	}

	if reason := router.modelDenied(originalModel); reason != "" {
		c.JSON(http.StatusForbidden, ErrorResponse{Error: reason})
		return
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

	endpoint := target.endpoint(provider.GetEndpoints())
	if endpoint == nil || *endpoint == "" {
		router.logger.Error("images api not supported by provider", nil, "provider", providerID)
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "The Images API is not supported by this provider yet."})
		return
	}

	if model != originalModel {
		form.Value[imageFormFieldModel] = []string{model}
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), router.cfg.Server.ReadTimeout)
	defer cancel()

	pr, pw := io.Pipe()
	mw := multipart.NewWriter(pw)
	contentType := mw.FormDataContentType()
	go func() {
		pw.CloseWithError(writeImagesMultipartForm(mw, form))
	}()

	upstreamURL := strings.TrimSuffix(provider.GetURL(), "/") + *endpoint
	upstreamReq, err := http.NewRequestWithContext(ctx, http.MethodPost, upstreamURL, pr)
	if err != nil {
		_ = pr.CloseWithError(err)
		router.logger.Error("failed to create upstream request", err, "url", upstreamURL)
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "Failed to create upstream request"})
		return
	}
	upstreamReq.Header.Set("Content-Type", contentType)
	upstreamReq.Header.Set("Accept", "application/json")

	if err := applyProviderAuth(upstreamReq, provider); err != nil {
		_ = pr.CloseWithError(err)
		router.logger.Error("unsupported auth type", err, "provider", providerID)
		c.JSON(http.StatusUnprocessableEntity, ErrorResponse{Error: "Unsupported auth type"})
		return
	}

	otelapi.GetTextMapPropagator().Inject(ctx, propagation.HeaderCarrier(upstreamReq.Header))

	resp, err := router.client.Do(upstreamReq)
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			router.logger.Error("request timed out", err, "provider", providerID)
			c.JSON(http.StatusGatewayTimeout, ErrorResponse{Error: "Request timed out"})
			return
		}
		router.logger.Error("failed to reach upstream server", err, "url", upstreamURL)
		c.JSON(http.StatusBadGateway, ErrorResponse{Error: "Failed to reach upstream server"})
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		span := trace.SpanFromContext(c.Request.Context())
		span.SetStatus(codes.Error, resp.Status)
		span.SetAttributes(semconv.ErrorTypeKey.String(strconv.Itoa(resp.StatusCode)))
	}

	c.DataFromReader(resp.StatusCode, resp.ContentLength, resp.Header.Get("Content-Type"), resp.Body, nil)
}

// imagesFormValue returns the first value for key in a parsed multipart form,
// or "" when absent.
func imagesFormValue(form *multipart.Form, key string) string {
	if vs := form.Value[key]; len(vs) > 0 {
		return vs[0]
	}
	return ""
}

// writeImagesMultipartForm re-encodes a parsed multipart form onto mw, copying
// each uploaded file straight through (preserving its Content-Type) so the
// payload streams to the upstream without a second full in-memory copy. It
// runs in its own goroutine writing to the io.Pipe.
func writeImagesMultipartForm(mw *multipart.Writer, form *multipart.Form) error {
	for field, values := range form.Value {
		for _, v := range values {
			if err := mw.WriteField(field, v); err != nil {
				return err
			}
		}
	}
	for field, headers := range form.File {
		for _, fh := range headers {
			if err := copyImagesFormFile(mw, field, fh); err != nil {
				return err
			}
		}
	}
	return mw.Close()
}

func copyImagesFormFile(mw *multipart.Writer, field string, fh *multipart.FileHeader) error {
	src, err := fh.Open()
	if err != nil {
		return err
	}
	defer src.Close()

	h := make(textproto.MIMEHeader)
	h.Set("Content-Disposition", fmt.Sprintf(`form-data; name=%q; filename=%q`, field, fh.Filename))
	if ct := fh.Header.Get("Content-Type"); ct != "" {
		h.Set("Content-Type", ct)
	}
	dst, err := mw.CreatePart(h)
	if err != nil {
		return err
	}
	_, err = io.Copy(dst, src)
	return err
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
