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
	"time"

	"github.com/google/uuid"

	proxymodifier "github.com/inference-gateway/inference-gateway/internal/proxy"

	gin "github.com/gin-gonic/gin"
	config "github.com/inference-gateway/inference-gateway/config"
	l "github.com/inference-gateway/inference-gateway/logger"
	providers "github.com/inference-gateway/inference-gateway/providers"
)

//go:generate mockgen -source=routes.go -destination=../tests/mocks/routes.go -package=mocks
type Router interface {
	ListModelsOpenAICompatibleHandler(c *gin.Context)
	ChatCompletionsOpenAICompatibleHandler(c *gin.Context)
	ProxyHandler(c *gin.Context)
	HealthcheckHandler(c *gin.Context)
	NotFoundHandler(c *gin.Context)
}

type RouterImpl struct {
	cfg      config.Config
	logger   l.Logger
	registry providers.ProviderRegistry
	client   providers.Client
}

type ErrorResponse struct {
	Error string `json:"error"`
}

type ResponseJSON struct {
	Message string `json:"message"`
}

func NewRouter(cfg config.Config, logger l.Logger, registry providers.ProviderRegistry, client providers.Client) Router {
	return &RouterImpl{
		cfg,
		logger,
		registry,
		client,
	}
}

func (router *RouterImpl) NotFoundHandler(c *gin.Context) {
	router.logger.Error("requested route is not found", nil)
	c.JSON(http.StatusNotFound, ErrorResponse{Error: "Requested route is not found"})
}

func (router *RouterImpl) ProxyHandler(c *gin.Context) {
	p := c.Param("provider")
	provider, err := router.registry.BuildProvider(p, router.client)
	if err != nil {
		if strings.Contains(err.Error(), "token not configured") {
			router.logger.Error("provider requires authentication but no API key was configured", err, "provider", p)
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

func handleStreamingRequest(c *gin.Context, provider providers.Provider, router *RouterImpl) {
	for k, v := range map[string]string{
		"Content-Type":      "text/event-stream",
		"Cache-Control":     "no-cache",
		"Connection":        "keep-alive",
		"Transfer-Encoding": "chunked",
	} {
		c.Header(k, v)
	}

	providerURL := provider.GetURL()
	fullURL := providerURL + strings.TrimPrefix(c.Request.URL.Path, "/proxy/"+c.Param("provider"))

	// Read request body with a 10MB size limit for now, to prevent abuse
	// Will make it configurable later perhaps as a middleware
	const maxBodySize = 10 << 20
	body, err := io.ReadAll(io.LimitReader(c.Request.Body, maxBodySize))
	if err != nil {
		router.logger.Error("failed to read request body", err)
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Failed to read request"})
		return
	}
	if len(body) >= int(maxBodySize) {
		c.JSON(http.StatusRequestEntityTooLarge, ErrorResponse{Error: "Request body too large"})
		return
	}

	ctx := c.Request.Context()
	upstreamReq, err := http.NewRequestWithContext(ctx, c.Request.Method, fullURL, bytes.NewReader(body))
	if err != nil {
		router.logger.Error("failed to create upstream request", err)
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "Failed to create upstream request"})
		return
	}

	upstreamReq.Header = c.Request.Header.Clone()

	resp, err := router.client.Do(upstreamReq)
	if err != nil {
		router.logger.Error("failed to make upstream request", err)
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
					"url", fullURL,
					"method", c.Request.Method)
			}
			return false
		}

		if len(line) == 0 {
			return true
		}

		router.logger.Debug("stream chunk",
			"provider", c.Param("provider"),
			"bytes", len(line),
			"data", string(bytes.TrimSpace(line)))

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

func handleProxyRequest(c *gin.Context, provider providers.Provider, router *RouterImpl) {
	remote, _ := url.Parse(provider.GetURL() + c.Request.URL.Path)
	proxy := httputil.NewSingleHostReverseProxy(remote)

	c.Request.Header.Set("Content-Type", "application/json")
	c.Request.Header.Set("Accept", "application/json")

	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		router.logger.Error("proxy request failed", err)
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
		req.Host = remote.Host
		req.URL.Host = remote.Host
		req.URL.Scheme = remote.Scheme
		req.URL.Path = c.Param("path")

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

func (router *RouterImpl) HealthcheckHandler(c *gin.Context) {
	router.logger.Debug("healthcheck")
	c.JSON(http.StatusOK, ResponseJSON{Message: "OK"})
}

// ListModelsOpenAICompatibleHandler implements an OpenAI-compatible API endpoint
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
//	    {
//	      "id": "model-id",
//	      "object": "model",
//	      "created": 1686935002,
//	      "owned_by": "provider-name"
//	    },
//	    ...
//	  ]
//	}
//
// This endpoint allows applications built for OpenAI's API to work seamlessly
// with the Inference Gateway's multi-provider architecture.
func (router *RouterImpl) ListModelsOpenAICompatibleHandler(c *gin.Context) {
	providerID := c.Query("provider")
	if providerID != "" {
		provider, err := router.registry.BuildProvider(c.Query("provider"), router.client)
		if err != nil {
			if strings.Contains(err.Error(), "token not configured") {
				router.logger.Error("provider requires authentication but no API key was configured", err, "provider", providerID)
				c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Provider requires an API key. Please configure the provider's API key."})
				return
			}
			router.logger.Error("provider not found or not supported", err, "provider", providerID)
			c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Provider not found. Please check the list of supported providers."})
			return
		}

		ctx, cancel := context.WithTimeout(context.Background(), router.cfg.Server.ReadTimeout*time.Millisecond)
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

		c.JSON(http.StatusOK, response)
	} else {
		var wg sync.WaitGroup
		providersCfg := router.cfg.Providers

		ch := make(chan providers.ListModelsResponse, len(providersCfg))

		ctx, cancel := context.WithTimeout(context.Background(), router.cfg.Server.ReadTimeout*time.Millisecond)
		defer cancel()

		for providerID := range providersCfg {
			wg.Add(1)
			go func(id string) {
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

		unifiedResponse := providers.ListModelsResponse{
			Object: "list",
			Data:   allModels,
		}

		c.JSON(http.StatusOK, unifiedResponse)
	}
}

// ChatCompletionsOpenAICompatibleHandler implements an OpenAI-compatible API endpoint
// that generates text completions in the standard OpenAI format.
//
// It returns token completions as chat in the standard OpenAI format, allowing applications
// built for OpenAI's API to work seamlessly with the Inference Gateway's multi-provider
// architecture.
func (router *RouterImpl) ChatCompletionsOpenAICompatibleHandler(c *gin.Context) {
	var req providers.ChatCompletionsRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		router.logger.Error("failed to decode request", err)
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Failed to decode request"})
		return
	}

	providerID := c.Query("provider")
	if providerID == "" {
		providerID = determineProviderFromModel(req.Model)
		if providerID == "" {
			router.logger.Error("unable to determine provider for model", nil, "model", req.Model)
			c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Unable to determine provider for model. Please specify a provider."})
			return
		}
	}

	provider, err := router.registry.BuildProvider(providerID, router.client)
	if err != nil {
		if strings.Contains(err.Error(), "token not configured") {
			router.logger.Error("provider requires authentication but no API key was configured", err, "provider", providerID)
			c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Provider requires an API key. Please configure the provider's API key."})
			return
		}
		router.logger.Error("provider not found or not supported", err, "provider", providerID)
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Provider not found. Please check the list of supported providers."})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), router.cfg.Server.ReadTimeout*time.Millisecond)
	defer cancel()

	// Streaming response
	if req.Stream {
		c.Header("Content-Type", "text/event-stream")
		c.Header("Cache-Control", "no-cache")
		c.Header("Connection", "keep-alive")
		c.Header("Transfer-Encoding", "chunked")

		streamCh, err := provider.StreamTokens(ctx, req.Model, req.Messages)
		if err != nil {
			router.logger.Error("failed to start streaming", err)
			c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Failed to start streaming"})
			return
		}

		c.Stream(func(w io.Writer) bool {
			select {
			case resp, ok := <-streamCh:
				if !ok {
					if _, err := c.Writer.Write([]byte("data: [DONE]\n\n")); err != nil {
						router.logger.Error("failed to write [DONE] marker", err)
					}
					return false
				}

				chunk := providers.ChunkResponse{
					ID:      "chatcmpl-" + uuid.New().String(),
					Object:  "chat.completion.chunk",
					Created: time.Now().Unix(),
					Model:   resp.Response.Model,
					Choices: []providers.ChunkChoice{
						{
							Index: 0,
							Delta: providers.ChunkDelta{
								Role:    resp.Response.Role,
								Content: resp.Response.Content,
							},
							FinishReason: nil,
						},
					},
				}

				data, err := json.Marshal(chunk)
				if err != nil {
					router.logger.Error("failed to marshal chunk", err)
					return false
				}

				if _, err := c.Writer.Write([]byte("data: " + string(data) + "\n\n")); err != nil {
					router.logger.Error("failed to write chunk", err)
					return false
				}
				c.Writer.Flush()
				return true
			case <-ctx.Done():
				return false
			}
		})
		return
	}

	// Non-streaming response
	response, err := provider.GenerateTokens(ctx, req.Model, req.Messages, req.Tools, req.MaxTokens)
	if err != nil {
		if err == context.DeadlineExceeded || ctx.Err() == context.DeadlineExceeded {
			router.logger.Error("request timed out", err, "provider", providerID)
			c.JSON(http.StatusGatewayTimeout, ErrorResponse{Error: "Request timed out"})
			return
		}
		router.logger.Error("failed to generate tokens", err, "provider", providerID)
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Failed to generate tokens"})
		return
	}

	openaiResponse := providers.CompletionResponse{
		ID:      "chatcmpl-" + uuid.New().String(),
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Model:   response.Response.Model,
		Choices: []providers.Choice{
			{
				Index: 0,
				Message: providers.Message{
					Role:    response.Response.Role,
					Content: response.Response.Content,
				},
				FinishReason: "stop",
			},
		},
		// TODO - need to implement the usage details correctly
		Usage: providers.Usage{
			PromptTokens:     0,
			CompletionTokens: 0,
			TotalTokens:      0,
			// Optional fields
			QueueTime:      0.0,
			PromptTime:     0.0,
			CompletionTime: 0.0,
			TotalTime:      0.0,
		},
	}

	c.JSON(http.StatusOK, openaiResponse)
}

func determineProviderFromModel(model string) string {
	modelLower := strings.ToLower(model)

	prefixMapping := map[string]string{
		"gpt-":      providers.OpenaiID,
		"claude-":   providers.AnthropicID,
		"llama-":    providers.GroqID,
		"command-":  providers.CohereID,
		"deepseek-": providers.GroqID,
	}

	for prefix, provider := range prefixMapping {
		if strings.HasPrefix(modelLower, prefix) {
			return provider
		}
	}

	return ""
}
