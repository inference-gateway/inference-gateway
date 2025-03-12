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

	proxymodifier "github.com/inference-gateway/inference-gateway/internal/proxy"

	gin "github.com/gin-gonic/gin"
	config "github.com/inference-gateway/inference-gateway/config"
	l "github.com/inference-gateway/inference-gateway/logger"
	providers "github.com/inference-gateway/inference-gateway/providers"
)

//go:generate mockgen -source=routes.go -destination=../tests/mocks/routes.go -package=mocks
type Router interface {
	ProxyHandler(c *gin.Context)
	ListAllModelsHandler(c *gin.Context)
	ListModelsHandler(c *gin.Context)
	GenerateProvidersTokenHandler(c *gin.Context)
	HealthcheckHandler(c *gin.Context)
	NotFoundHandler(c *gin.Context)

	ListModelsOpenAICompatibleHandler(c *gin.Context)
	ChatCompletionsOpenAICompatibleHandler(c *gin.Context)
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

// OpenAI Compatible API Handlers
// It returns a list of models from the provider if the provider is specified
// If no provider is specified, it returns a list of models from all providers
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

func (router *RouterImpl) ChatCompletionsOpenAICompatibleHandler(c *gin.Context) {

}

func (router *RouterImpl) ListModelsHandler(c *gin.Context) {
	provider, err := router.registry.BuildProvider(c.Param("provider"), router.client)
	if err != nil {
		if strings.Contains(err.Error(), "token not configured") {
			router.logger.Error("provider requires authentication but no API key was configured", err, "provider", provider.GetName())
			c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Provider requires an API key. Please configure the provider's API key."})
			return
		}
		router.logger.Error("provider not found or not supported", err, "provider", provider.GetName())
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
}

func (router *RouterImpl) ListAllModelsHandler(c *gin.Context) {
	var wg sync.WaitGroup
	providersCfg := router.cfg.Providers

	ch := make(chan providers.ListModelsResponse, len(providersCfg))

	ctx, cancel := context.WithTimeout(context.Background(), router.cfg.Server.ReadTimeout*time.Millisecond)
	defer cancel()

	for providerID := range providersCfg {
		wg.Add(1)
		go func(id string) {
			defer wg.Done()

			provider, err := router.registry.BuildProvider(providerID, router.client)
			if err != nil {
				router.logger.Error("failed to create provider", err)
				ch <- providers.ListModelsResponse{
					Provider: id,
					Data:     make([]providers.Model, 0),
				}
				return
			}

			response, err := provider.ListModels(ctx)
			if err != nil {
				if ctx.Err() == context.DeadlineExceeded {
					router.logger.Error("request timed out", err, "provider", id)
					ch <- providers.ListModelsResponse{
						Provider: id,
						Data:     make([]providers.Model, 0),
					}
					return
				}
				router.logger.Error("failed to list models", err, "provider", id)
				ch <- providers.ListModelsResponse{
					Provider: id,
					Data:     make([]providers.Model, 0),
				}
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

	var allModels []providers.ListModelsResponse
	for model := range ch {
		allModels = append(allModels, model)
	}

	c.JSON(http.StatusOK, allModels)
}

func (router *RouterImpl) GenerateProvidersTokenHandler(c *gin.Context) {
	var req providers.GenerateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		router.logger.Error("failed to decode request", err)
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Failed to decode request"})
		return
	}

	if req.Model == "" {
		router.logger.Error("model is required", nil)
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Model is required"})
		return
	}

	provider, err := router.registry.BuildProvider(c.Param("provider"), router.client)
	if err != nil {
		if strings.Contains(err.Error(), "token not configured") {
			router.logger.Error("provider requires authentication but no API key was configured", err, "provider", c.Param("provider"))
			c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Provider requires an API key. Please configure the provider's API key."})
			return
		}
		router.logger.Error("provider not found or not supported", err, "provider", c.Param("provider"))
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Provider not found. Please check the list of supported providers."})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), router.cfg.Server.ReadTimeout*time.Millisecond)
	defer cancel()

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
					return false
				}

				jsonData, err := json.Marshal(resp.Response)
				if err != nil {
					router.logger.Error("failed to marshal token", err)
					return false
				}

				if req.SSEvents {
					switch resp.EventType {
					case providers.EventMessageStart:
						c.SSEvent(string(providers.EventMessageStart), string(providers.EventMessageStartValue))

					case providers.EventStreamStart:
						c.SSEvent(string(providers.EventStreamStart), string(providers.EventStreamStartValue))

					case providers.EventContentStart:
						c.SSEvent(string(providers.EventContentStart), string(providers.EventContentStartValue))

					case providers.EventContentDelta:
						c.SSEvent(string(providers.EventContentDelta), string(jsonData))

					case providers.EventContentEnd:
						c.SSEvent(string(providers.EventContentEnd), string(providers.EventContentEndValue))

					case providers.EventMessageEnd:
						c.SSEvent(string(providers.EventMessageEnd), string(providers.EventMessageEndValue))

					case providers.EventStreamEnd:
						c.SSEvent(string(providers.EventStreamEnd), string(providers.EventStreamEndValue))

					}
					return true
				}

				// Write Raw JSON chunk
				if _, err := c.Writer.Write(jsonData); err != nil {
					router.logger.Error("failed to write response chunk", err)
					return false
				}
				if _, err := c.Writer.Write([]byte("\n")); err != nil {
					router.logger.Error("failed to write newline", err)
					return false
				}
				return true
			case <-ctx.Done():
				return false
			}
		})
		return
	}

	response, err := provider.GenerateTokens(ctx, req.Model, req.Messages, req.Tools, req.MaxTokens)
	if err != nil {
		if err == context.DeadlineExceeded || ctx.Err() == context.DeadlineExceeded {
			router.logger.Error("request timed out", err, "provider", c.Param("provider"))
			c.JSON(http.StatusGatewayTimeout, ErrorResponse{Error: "Request timed out"})
			return
		}
		router.logger.Error("failed to generate tokens", err, "provider", c.Param("provider"))
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Failed to generate tokens"})
		return
	}

	c.JSON(http.StatusOK, response)
}
