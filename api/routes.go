package api

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"sync"
	"time"

	proxymodifier "github.com/inference-gateway/inference-gateway/internal/proxy"

	"github.com/inference-gateway/a2a/adk"
	"github.com/inference-gateway/inference-gateway/a2a"
	"github.com/inference-gateway/inference-gateway/mcp"

	gin "github.com/gin-gonic/gin"
	config "github.com/inference-gateway/inference-gateway/config"
	l "github.com/inference-gateway/inference-gateway/logger"
	providers "github.com/inference-gateway/inference-gateway/providers"
)

//go:generate mockgen -source=routes.go -destination=../tests/mocks/routes.go -package=mocks
type Router interface {
	ListModelsHandler(c *gin.Context)
	ChatCompletionsHandler(c *gin.Context)
	ListToolsHandler(c *gin.Context)
	ListAgentsHandler(c *gin.Context)
	GetAgentHandler(c *gin.Context)
	GetAgentStatusHandler(c *gin.Context)
	GetAllAgentStatusesHandler(c *gin.Context)
	ProxyHandler(c *gin.Context)
	HealthcheckHandler(c *gin.Context)
	NotFoundHandler(c *gin.Context)
}

type RouterImpl struct {
	cfg       config.Config
	logger    l.Logger
	registry  providers.ProviderRegistry
	client    providers.Client
	a2aClient a2a.A2AClientInterface
	mcpClient mcp.MCPClientInterface
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
	registry providers.ProviderRegistry,
	client providers.Client,
	mcpClient mcp.MCPClientInterface,
	a2aClient a2a.A2AClientInterface,
) Router {
	return &RouterImpl{
		cfg,
		logger,
		registry,
		client,
		a2aClient,
		mcpClient,
	}
}

func (router *RouterImpl) NotFoundHandler(c *gin.Context) {
	router.logger.Warn("route not found", "path", c.Request.URL.Path, "method", c.Request.Method)
	c.JSON(http.StatusNotFound, ErrorResponse{Error: "Requested route is not found"})
}

func (router *RouterImpl) ProxyHandler(c *gin.Context) {
	p := providers.Provider(c.Param("provider"))
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
	case providers.AuthTypeBearer:
		c.Request.Header.Set("Authorization", "Bearer "+token)
	case providers.AuthTypeXheader:
		c.Request.Header.Set("x-api-key", token)
	case providers.AuthTypeQuery:
		query := c.Request.URL.Query()
		query.Set("key", token)
		c.Request.URL.RawQuery = query.Encode()
	case providers.AuthTypeNone:
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

func handleStreamingRequest(c *gin.Context, provider providers.IProvider, router *RouterImpl) {
	for k, v := range map[string]string{
		"Content-Type":      "text/event-stream",
		"Cache-Control":     "no-cache",
		"Connection":        "keep-alive",
		"Transfer-Encoding": "chunked",
	} {
		c.Header(k, v)
	}

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

func handleProxyRequest(c *gin.Context, provider providers.IProvider, router *RouterImpl) {
	fullURL, err := constructProviderURL(provider, c.Param("path"), c.Request.URL.RawQuery)
	if err != nil {
		router.logger.Error("failed to construct provider url", err, "provider", provider.GetName())
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Failed to construct URL"})
		return
	}
	proxy := httputil.NewSingleHostReverseProxy(fullURL)

	c.Request.Header.Set("Content-Type", "application/json")
	c.Request.Header.Set("Accept", "application/json")

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

	proxy.Director = func(req *http.Request) {
		req.Header = c.Request.Header
		req.Host = fullURL.Host
		req.URL.Host = fullURL.Host
		req.URL.Scheme = fullURL.Scheme
		req.URL.Path = fullURL.Path
		req.URL.RawQuery = fullURL.RawQuery

		if router.cfg.Environment == "development" {
			reqModifier := proxymodifier.NewDevRequestModifier(router.logger)
			if err := reqModifier.Modify(req); err != nil {
				router.logger.Error("failed to modify request", err)
				return
			}
		}
	}

	if router.cfg.Environment == "development" {
		devModifier := proxymodifier.NewDevResponseModifier(router.logger)
		proxy.ModifyResponse = devModifier.Modify
	}

	proxy.ServeHTTP(c.Writer, c.Request)
}

// constructProviderURL builds the provider URL consistently to avoid path duplication.
// It ensures that the path from the provider URL is handled correctly with the path parameter.
func constructProviderURL(provider providers.IProvider, pathParam, rawQuery string) (*url.URL, error) {
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

// ListModelsHandler implements an OpenAI-compatible API endpoint
// that returns model information in the standard OpenAI format.
//
// This handler supports the OpenAI GET /v1/models endpoint specification:
// https://platform.openai.com/docs/api-reference/models/list
//
// Parameters:
//   - provider (query): Optional. When specified, returns models from only that provider.
//     If not specified, returns models from all configured providers.
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
	providerID := providers.Provider(c.Query("provider"))
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

		ctx, cancel := context.WithTimeout(c, router.cfg.Server.ReadTimeout)
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

		response.Data = router.filterModelsByAllowList(response.Data, router.cfg.AllowedModels)

		c.JSON(http.StatusOK, response)
	} else {
		var wg sync.WaitGroup
		providersCfg := router.cfg.Providers

		ch := make(chan providers.ListModelsResponse, len(providersCfg))

		ctx, cancel := context.WithTimeout(c, router.cfg.Server.ReadTimeout*time.Millisecond)
		defer cancel()

		for providerID := range providersCfg {
			wg.Add(1)
			go func(id providers.Provider) {
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
					response.Data = make([]providers.Model, 0)
				}
				ch <- response
			}(providerID)
		}

		wg.Wait()
		close(ch)

		var allModels []providers.Model
		for response := range ch {
			allModels = append(allModels, response.Data...)
		}

		if allModels == nil {
			allModels = make([]providers.Model, 0)
		}

		allModels = router.filterModelsByAllowList(allModels, router.cfg.AllowedModels)

		unifiedResponse := providers.ListModelsResponse{
			Object: "list",
			Data:   allModels,
		}

		c.JSON(http.StatusOK, unifiedResponse)
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
	var req providers.CreateChatCompletionRequest

	if mcpRequest, exists := c.Get("X-MCP-Bypass"); exists {
		if parsedRequest, ok := mcpRequest.(*providers.CreateChatCompletionRequest); ok {
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
	providerID := providers.Provider(c.Query("provider"))
	if providerID == "" {
		var providerPtr *providers.Provider
		providerPtr, model = providers.DetermineProviderAndModelName(model)
		if providerPtr == nil {
			router.logger.Error("unable to determine provider for model", nil, "model", req.Model)
			c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Unable to determine provider for model. Please specify a provider using the ?provider= query parameter or use the provider/model format (e.g., openai/gpt-4)."})
			return
		}
		providerID = *providerPtr
	}
	req.Model = model

	if router.cfg.AllowedModels != "" {
		if !router.isModelAllowed(originalModel, router.cfg.AllowedModels) {
			router.logger.Error("model not in allowed list", nil, "model", originalModel, "allowed_models", router.cfg.AllowedModels)
			c.JSON(http.StatusForbidden, ErrorResponse{Error: "Model not allowed. Please check the list of allowed models."})
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

	router.logger.Debug("server read timeout", "timeout", router.cfg.Server.ReadTimeout)

	ctx, cancel := context.WithTimeout(c, router.cfg.Server.ReadTimeout)
	defer cancel()

	// Streaming response
	if req.Stream != nil && *req.Stream {
		c.Header("Content-Type", "text/event-stream")
		c.Header("Cache-Control", "no-cache")
		c.Header("Connection", "keep-alive")
		c.Header("Transfer-Encoding", "chunked")
		c.Header("X-Accel-Buffering", "no")

		streamCh, err := provider.StreamChatCompletions(ctx, req)
		if err != nil {
			router.logger.Error("failed to start streaming", err, "provider", providerID)
			c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Failed to start streaming"})
			return
		}

		c.Stream(func(w io.Writer) bool {
			select {
			case line, ok := <-streamCh:
				if !ok {
					router.logger.Debug("stream closed", "provider", providerID)
					return false
				}

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
			case <-ctx.Done():
				return false
			}
		})
		return
	}

	// Non-streaming response
	c.Header("Content-Type", "application/json")
	response, err := provider.ChatCompletions(ctx, req)
	if err != nil {
		if err == context.DeadlineExceeded || ctx.Err() == context.DeadlineExceeded {
			router.logger.Error("request timed out", err, "provider", providerID)
			c.JSON(http.StatusGatewayTimeout, ErrorResponse{Error: "Request timed out"})
			return
		}
		router.logger.Error("failed to generate tokens", err, "provider", providerID)
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: fmt.Sprintf("Failed to generate tokens: %s", err)})
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

	var allTools []providers.MCPTool

	switch {
	case router.mcpClient == nil:
		router.logger.Debug("mcp client is nil, returning empty tools list")
		allTools = make([]providers.MCPTool, 0)
	case !router.mcpClient.IsInitialized():
		router.logger.Info("mcp client not initialized, no tools available")
		allTools = make([]providers.MCPTool, 0)
	default:
		servers := router.mcpClient.GetServers()

		for _, serverURL := range servers {
			tools, err := router.mcpClient.GetServerTools(serverURL)
			if err != nil {
				router.logger.Error("failed to get tools from mcp server", err, "server", serverURL)
				continue
			}

			for _, tool := range tools {
				mcpTool := providers.MCPTool{
					Name:        tool.Name,
					Description: *tool.Description,
					Server:      serverURL,
					InputSchema: &tool.InputSchema,
				}
				allTools = append(allTools, mcpTool)
			}
		}

		if allTools == nil {
			allTools = make([]providers.MCPTool, 0)
		}
	}

	response := providers.ListToolsResponse{
		Object: "list",
		Data:   allTools,
	}

	c.JSON(http.StatusOK, response)
}

// ListAgentsHandler implements an endpoint that returns complete A2A agent cards with all detailed information
// when A2A_EXPOSE environment variable is enabled.
//
// This handler supports the Agent-to-Agent (A2A) protocol by exposing a list of
// connected agents along with their complete agent cards containing comprehensive
// metadata such as name, description, URL, capabilities, skills, security schemes,
// and supported input/output modes.
//
// The endpoint follows the OpenAI-compatible API format pattern used throughout
// the gateway for consistency.
//
// Request:
//   - Method: GET
//   - Path: /a2a/agents
//   - Authentication: Required (Bearer token)
//   - Query Parameters: None
//
// Response format when A2A is exposed and agents are available:
//
//	{
//	  "object": "list",
//	  "data": [
//	    {
//	      "name": "Calculator Agent",
//	      "description": "An agent that can perform mathematical calculations",
//	      "url": "https://agent1.example.com",
//	      "version": "1.0.0",
//	      "capabilities": {
//	        "streaming": true,
//	        "pushNotifications": false,
//	        "stateTransitionHistory": false,
//	        "extensions": []
//	      },
//	      "skills": [
//	        {
//	          "id": "calculate",
//	          "name": "Mathematical Calculation",
//	          "description": "Perform basic and advanced mathematical operations",
//	          "tags": ["math", "calculation"],
//	          "examples": ["2 + 2", "sqrt(16)", "sin(pi/2)"]
//	        }
//	      ],
//	      "defaultInputModes": ["text/plain"],
//	      "defaultOutputModes": ["text/plain", "application/json"],
//	      "provider": {
//	        "organization": "Example Corp",
//	        "url": "https://example.com"
//	      },
//	      "security": [],
//	      "securitySchemes": {}
//	    }
//	  ]
//	}
//
// Response when A2A is not exposed:
//
//	{
//	  "error": "A2A agents endpoint is not exposed. Set A2A_EXPOSE=true to enable."
//	}
//
// Response when no agents are available:
//
//	{
//	  "object": "list",
//	  "data": []
//	}
//
// Error Handling:
//   - Returns 403 Forbidden if A2A_EXPOSE is not enabled
//   - Returns empty list if A2A client is not initialized
//   - Continues processing other agents if individual agent card retrieval fails
//   - Logs errors for failed agent card retrievals but doesn't fail the entire request
//
// The handler gracefully handles various states:
//   - A2A client is nil (returns empty list)
//   - A2A client is not initialized (returns empty list)
//   - Individual agent card retrieval failures (skips failed agents, continues with others)
//   - No agents configured (returns empty list)
//
// Security:
//   - Requires authentication via Bearer token
//   - Only exposes agents when explicitly configured via A2A_EXPOSE=true
//   - Does not expose internal errors to clients
//
// Note: This endpoint now returns complete AgentCard objects instead of simplified
// A2AItem objects, providing clients with full agent metadata including capabilities,
// skills, security requirements, and supported modalities.
func (router *RouterImpl) ListAgentsHandler(c *gin.Context) {
	if !router.cfg.A2A.Expose {
		router.logger.Error("a2a agents endpoint access attempted but not exposed", nil)
		c.JSON(http.StatusForbidden, ErrorResponse{Error: "A2A agents endpoint is not exposed. Set A2A_EXPOSE=true to enable."})
		return
	}

	var allAgents []providers.A2AAgentCard

	switch {
	case router.a2aClient == nil:
		router.logger.Debug("a2a client is nil, returning empty agents list")
		allAgents = make([]providers.A2AAgentCard, 0)
	case !router.a2aClient.IsInitialized():
		router.logger.Info("a2a client not initialized, no agents available")
		allAgents = make([]providers.A2AAgentCard, 0)
	default:
		agentURLs := router.a2aClient.GetAgents()

		for _, agentURL := range agentURLs {
			agentCard, err := router.a2aClient.GetAgentCard(c.Request.Context(), agentURL)
			if err != nil {
				router.logger.Error("failed to get agent card from a2a agent", err, "agent", agentURL)
				continue
			}

			// TODO: refactor providers package to be able to use a2a.AgentCard directly, instead of copying fields
			convertedCard := convertA2AAgentCard(*agentCard, agentURL)
			allAgents = append(allAgents, convertedCard)
		}

		if allAgents == nil {
			allAgents = make([]providers.A2AAgentCard, 0)
		}
	}

	response := providers.ListAgentsResponse{
		Object: "list",
		Data:   allAgents,
	}

	c.JSON(http.StatusOK, response)
}

// GetAgentHandler implements an endpoint that returns a specific A2A agent by ID
//
// This endpoint provides detailed information about a single A2A agent identified by its unique ID.
// The ID is generated as a base64-encoded SHA256 hash of the agent's URL to ensure uniqueness
// and deterministic identification across restarts.
//
// Request:
//   - Method: GET
//   - Path: /a2a/agents/{id}
//   - Parameters:
//   - id (path): The unique identifier of the agent (base64-encoded SHA256 hash of the agent URL)
//
// Response (200 OK):
//   - Content-Type: application/json
//   - Body: A2AAgentCard object containing complete agent information
//
// Response (404 Not Found):
//   - When the specified agent ID does not exist
//
// Response (403 Forbidden):
//   - When A2A_EXPOSE is not enabled
//
// Security:
//   - Requires authentication via Bearer token
//   - Only accessible when explicitly configured via A2A_EXPOSE=true
//   - Does not expose internal errors to clients
func (router *RouterImpl) GetAgentHandler(c *gin.Context) {
	if !router.cfg.A2A.Expose {
		router.logger.Error("a2a agent endpoint access attempted but not exposed", nil)
		c.JSON(http.StatusForbidden, ErrorResponse{Error: "A2A agents endpoint is not exposed. Set A2A_EXPOSE=true to enable."})
		return
	}

	agentID := c.Param("id")
	if agentID == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Agent ID is required"})
		return
	}

	switch {
	case router.a2aClient == nil:
		router.logger.Debug("a2a client is nil")
		c.JSON(http.StatusNotFound, ErrorResponse{Error: "Agent not found"})
		return
	case !router.a2aClient.IsInitialized():
		router.logger.Info("a2a client not initialized, no agents available")
		c.JSON(http.StatusNotFound, ErrorResponse{Error: "Agent not found"})
		return
	}

	agentURLs := router.a2aClient.GetAgents()

	for _, agentURL := range agentURLs {
		if generateAgentID(agentURL) == agentID {
			agentCard, err := router.a2aClient.GetAgentCard(c.Request.Context(), agentURL)
			if err != nil {
				router.logger.Error("failed to get agent card from a2a agent", err, "agent", agentURL)
				c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "Failed to retrieve agent information"})
				return
			}

			convertedCard := convertA2AAgentCard(*agentCard, agentURL)
			c.JSON(http.StatusOK, convertedCard)
			return
		}
	}

	c.JSON(http.StatusNotFound, ErrorResponse{Error: "Agent not found"})
}

// convertA2AAgentCard converts an adk.AgentCard to providers.A2AAgentCard
func convertA2AAgentCard(agentCard adk.AgentCard, agentURL string) providers.A2AAgentCard {
	capabilities := make(map[string]interface{})

	if agentCard.Capabilities.Extensions != nil {
		capabilities["extensions"] = agentCard.Capabilities.Extensions
	} else {
		capabilities["extensions"] = []adk.AgentExtension{}
	}

	capabilities["pushNotifications"] = agentCard.Capabilities.PushNotifications
	capabilities["stateTransitionHistory"] = agentCard.Capabilities.StateTransitionHistory
	capabilities["streaming"] = agentCard.Capabilities.Streaming

	var provider *map[string]interface{}
	if agentCard.Provider != nil {
		providerMap := make(map[string]interface{})
		providerMap["organization"] = agentCard.Provider.Organization
		providerMap["url"] = agentCard.Provider.URL
		provider = &providerMap
	}

	var security *[]map[string]interface{}
	if agentCard.Security != nil {
		securitySlice := make([]map[string]interface{}, len(agentCard.Security))
		for i, sec := range agentCard.Security {
			securityMap := make(map[string]interface{})
			for k, v := range sec {
				securityMap[k] = v
			}
			securitySlice[i] = securityMap
		}
		security = &securitySlice
	}

	var securitySchemes *map[string]interface{}
	if agentCard.SecuritySchemes != nil {
		schemes := make(map[string]interface{})
		for k, v := range agentCard.SecuritySchemes {
			schemes[k] = v
		}
		securitySchemes = &schemes
	}

	skills := make([]map[string]interface{}, len(agentCard.Skills))
	for i, skill := range agentCard.Skills {
		skillMap := make(map[string]interface{})
		skillMap["description"] = skill.Description

		if skill.Examples != nil {
			skillMap["examples"] = skill.Examples
		} else {
			skillMap["examples"] = []string{}
		}

		skillMap["id"] = skill.ID

		if skill.InputModes != nil {
			skillMap["inputModes"] = skill.InputModes
		} else {
			skillMap["inputModes"] = []string{}
		}

		skillMap["name"] = skill.Name
		if skill.OutputModes != nil {
			skillMap["outputModes"] = skill.OutputModes
		} else {
			skillMap["outputModes"] = []string{}
		}

		if skill.Tags != nil {
			skillMap["tags"] = skill.Tags
		} else {
			skillMap["tags"] = []string{}
		}

		skills[i] = skillMap
	}

	return providers.A2AAgentCard{
		Capabilities:                      capabilities,
		Defaultinputmodes:                 agentCard.DefaultInputModes,
		Defaultoutputmodes:                agentCard.DefaultOutputModes,
		Description:                       agentCard.Description,
		Documentationurl:                  agentCard.DocumentationURL,
		Iconurl:                           agentCard.IconURL,
		ID:                                generateAgentID(agentURL),
		Name:                              agentCard.Name,
		Provider:                          provider,
		Security:                          security,
		Securityschemes:                   securitySchemes,
		Skills:                            skills,
		Supportsauthenticatedextendedcard: agentCard.SupportsAuthenticatedExtendedCard,
		Url:                               agentCard.URL,
		Version:                           agentCard.Version,
	}
}

// filterModelsByAllowList filters models based on the comma-separated ALLOWED_MODELS configuration.
// If allowedModels is empty, all models are returned. Otherwise, only models matching
// the allowed list are returned. The matching is done using case-insensitive comparison.
func (router *RouterImpl) filterModelsByAllowList(models []providers.Model, allowedModels string) []providers.Model {
	if allowedModels == "" {
		return models
	}

	allowedMap := make(map[string]bool)
	for _, model := range strings.Split(allowedModels, ",") {
		trimmed := strings.TrimSpace(model)
		if trimmed != "" {
			allowedMap[strings.ToLower(trimmed)] = true
		}
	}

	if len(allowedMap) == 0 {
		return models
	}

	filtered := make([]providers.Model, 0)
	for _, model := range models {
		modelID := strings.ToLower(model.ID)

		parts := strings.SplitN(model.ID, "/", 2)
		modelName := ""
		if len(parts) == 2 {
			modelName = strings.ToLower(parts[1])
		}

		if allowedMap[modelID] || (modelName != "" && allowedMap[modelName]) {
			filtered = append(filtered, model)
		}
	}

	return filtered
}

// isModelAllowed checks if a specific model is allowed based on the ALLOWED_MODELS configuration.
// If allowedModels is empty, all models are allowed. Otherwise, only models matching
// the allowed list are permitted. The matching is done using case-insensitive comparison.
func (router *RouterImpl) isModelAllowed(modelID string, allowedModels string) bool {
	if allowedModels == "" {
		return true
	}

	allowedMap := make(map[string]bool)
	for _, model := range strings.Split(allowedModels, ",") {
		trimmed := strings.TrimSpace(model)
		if trimmed != "" {
			allowedMap[strings.ToLower(trimmed)] = true
		}
	}

	if len(allowedMap) == 0 {
		return true
	}

	modelIDLower := strings.ToLower(modelID)
	if allowedMap[modelIDLower] {
		return true
	}

	parts := strings.SplitN(modelID, "/", 2)
	if len(parts) == 2 {
		modelName := strings.ToLower(parts[1])
		if allowedMap[modelName] {
			return true
		}
	}

	return false
}

// generateAgentID creates a unique identifier for an agent based on its URL
// Uses SHA256 hash of the URL encoded as base64 for deterministic, unique IDs
func generateAgentID(agentURL string) string {
	hash := sha256.Sum256([]byte(agentURL))
	return base64.StdEncoding.EncodeToString(hash[:])
}

// GetAgentStatusHandler implements an endpoint that returns the status of a specific A2A agent
//
// This endpoint provides the current status of a single A2A agent identified by its unique ID.
// The status indicates whether the agent is available, unavailable, or in an unknown state.
//
// Request:
//   - Method: GET
//   - Path: /a2a/agents/{id}/status
//   - Parameters:
//   - id (path): The unique identifier of the agent (base64-encoded SHA256 hash of the agent URL)
//
// Response format (200 OK):
//
//	{
//	  "id": "agent-id",
//	  "status": "available|unavailable|unknown",
//	  "url": "https://agent.example.com"
//	}
//
// Response (404 Not Found):
//   - When the specified agent ID does not exist
//
// Response (403 Forbidden):
//   - When A2A_EXPOSE is not enabled
//
// Status values:
//   - "available": Agent is responding to health checks
//   - "unavailable": Agent is not responding to health checks
//   - "unknown": Agent status is unknown or not yet determined
func (router *RouterImpl) GetAgentStatusHandler(c *gin.Context) {
	if !router.cfg.A2A.Expose {
		router.logger.Error("a2a agent status endpoint access attempted but not exposed", nil)
		c.JSON(http.StatusForbidden, ErrorResponse{Error: "A2A agents endpoint is not exposed. Set A2A_EXPOSE=true to enable."})
		return
	}

	agentID := c.Param("id")
	if agentID == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Agent ID is required"})
		return
	}

	switch {
	case router.a2aClient == nil:
		router.logger.Debug("a2a client is nil")
		c.JSON(http.StatusNotFound, ErrorResponse{Error: "Agent not found"})
		return
	case !router.a2aClient.IsInitialized():
		router.logger.Info("a2a client not initialized, no agents available")
		c.JSON(http.StatusNotFound, ErrorResponse{Error: "Agent not found"})
		return
	}

	agentURLs := router.a2aClient.GetAgents()

	for _, agentURL := range agentURLs {
		if generateAgentID(agentURL) == agentID {
			status := router.a2aClient.GetAgentStatus(agentURL)
			response := map[string]interface{}{
				"id":     agentID,
				"status": string(status),
				"url":    agentURL,
			}
			c.JSON(http.StatusOK, response)
			return
		}
	}

	c.JSON(http.StatusNotFound, ErrorResponse{Error: "Agent not found"})
}

// GetAllAgentStatusesHandler implements an endpoint that returns the status of all A2A agents
//
// This endpoint provides the current status of all configured A2A agents.
// The status indicates whether each agent is available, unavailable, or in an unknown state.
//
// Request:
//   - Method: GET
//   - Path: /a2a/agents/status
//
// Response format (200 OK):
//
//	{
//	  "object": "list",
//	  "data": [
//	    {
//	      "id": "agent-id-1",
//	      "status": "available",
//	      "url": "https://agent1.example.com"
//	    },
//	    {
//	      "id": "agent-id-2",
//	      "status": "unavailable",
//	      "url": "https://agent2.example.com"
//	    }
//	  ]
//	}
//
// Response (403 Forbidden):
//   - When A2A_EXPOSE is not enabled
//
// Response when no agents are configured:
//
//	{
//	  "object": "list",
//	  "data": []
//	}
//
// Status values:
//   - "available": Agent is responding to health checks
//   - "unavailable": Agent is not responding to health checks
//   - "unknown": Agent status is unknown or not yet determined
func (router *RouterImpl) GetAllAgentStatusesHandler(c *gin.Context) {
	if !router.cfg.A2A.Expose {
		router.logger.Error("a2a agent statuses endpoint access attempted but not exposed", nil)
		c.JSON(http.StatusForbidden, ErrorResponse{Error: "A2A agents endpoint is not exposed. Set A2A_EXPOSE=true to enable."})
		return
	}

	var statusList []map[string]interface{}

	switch {
	case router.a2aClient == nil:
		router.logger.Debug("a2a client is nil, returning empty status list")
		statusList = make([]map[string]interface{}, 0)
	case !router.a2aClient.IsInitialized():
		router.logger.Info("a2a client not initialized, no agents available")
		statusList = make([]map[string]interface{}, 0)
	default:
		agentURLs := router.a2aClient.GetAgents()
		allStatuses := router.a2aClient.GetAllAgentStatuses()

		statusList = make([]map[string]interface{}, 0, len(agentURLs))
		for _, agentURL := range agentURLs {
			status := allStatuses[agentURL]
			statusInfo := map[string]interface{}{
				"id":     generateAgentID(agentURL),
				"status": string(status),
				"url":    agentURL,
			}
			statusList = append(statusList, statusInfo)
		}
	}

	response := map[string]interface{}{
		"object": "list",
		"data":   statusList,
	}

	c.JSON(http.StatusOK, response)
}
