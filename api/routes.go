package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"sync"

	proxymodifier "github.com/inference-gateway/inference-gateway/internal/proxy"

	gin "github.com/gin-gonic/gin"
	config "github.com/inference-gateway/inference-gateway/config"
	l "github.com/inference-gateway/inference-gateway/logger"
	providers "github.com/inference-gateway/inference-gateway/providers"
)

//go:generate mockgen -source=routes.go -destination=../tests/mocks/routes.go -package=mocks
type Router interface {
	GetClient() http.Client
	NotFoundHandler(c *gin.Context)
	ProxyHandler(c *gin.Context)
	HealthcheckHandler(c *gin.Context)
	FetchAllModelsHandler(c *gin.Context)
	GenerateProvidersTokenHandler(c *gin.Context)
	ValidateProvider(provider string) (providers.Provider, bool)
}

type RouterImpl struct {
	cfg    config.Config
	logger l.Logger
	client http.Client
}

type ErrorResponse struct {
	Error string `json:"error"`
}

type ResponseJSON struct {
	Message string `json:"message"`
}

func NewRouter(cfg config.Config, logger *l.Logger, client *http.Client) Router {
	return &RouterImpl{
		cfg,
		*logger,
		*client,
	}
}

func (router *RouterImpl) GetClient() http.Client {
	return router.client
}

func (router *RouterImpl) ValidateProvider(provider string) (providers.Provider, bool) {
	p, ok := router.cfg.Providers()[provider]
	if !ok {
		return nil, false
	}

	return p, ok
}

func (router *RouterImpl) NotFoundHandler(c *gin.Context) {
	router.logger.Error("requested route is not found", nil)
	c.JSON(http.StatusNotFound, ErrorResponse{Error: "Requested route is not found"})
}

func (router *RouterImpl) ProxyHandler(c *gin.Context) {
	p := c.Param("provider")
	provider, ok := router.ValidateProvider(p)
	if !ok {
		router.logger.Error("requested unsupported provider", nil, "provider", provider)
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Requested unsupported provider"})
		return
	}

	// Setup authentication headers or query params
	token := provider.GetToken()
	switch provider.GetAuthType() {
	case "bearer":
		c.Request.Header.Set("Authorization", "Bearer "+token)
	case "xheader":
		c.Request.Header.Set("x-api-key", token)
		for k, v := range provider.GetExtraXHeaders() {
			c.Request.Header.Set(k, v)
		}
	case "query":
		query := c.Request.URL.Query()
		query.Set("key", token)
		c.Request.URL.RawQuery = query.Encode()
	default:
		c.JSON(http.StatusUnprocessableEntity, ErrorResponse{Error: "Unsupported auth type"})
		return
	}

	// Setup common headers
	c.Request.Header.Set("Content-Type", "application/json")
	c.Request.Header.Set("Accept", "application/json")

	// Create and configure proxy
	remote, _ := url.Parse(provider.GetURL() + c.Request.URL.Path)
	proxy := httputil.NewSingleHostReverseProxy(remote)

	// Log proxy responses in development mode only
	if router.cfg.Environment == "development" {
		devModifier := proxymodifier.NewDevResponseModifier(router.logger)
		proxy.ModifyResponse = devModifier.Modify
	}

	proxy.Director = func(req *http.Request) {
		req.Header = c.Request.Header
		req.Host = remote.Host
		req.URL.Host = remote.Host
		req.URL.Scheme = remote.Scheme
		req.URL.Path = remote.Path
	}
	proxy.ServeHTTP(c.Writer, c.Request)
}

func (router *RouterImpl) HealthcheckHandler(c *gin.Context) {
	router.logger.Debug("healthcheck")
	c.JSON(http.StatusOK, ResponseJSON{Message: "OK"})
}

type ModelResponse struct {
	Provider string        `json:"provider"`
	Models   []interface{} `json:"models"`
}

func (router *RouterImpl) FetchAllModelsHandler(c *gin.Context) {
	var wg sync.WaitGroup
	modelProviders := router.cfg.ListLLMsEndpoints()

	ch := make(chan providers.ModelsResponse, len(modelProviders))
	for provider, url := range modelProviders {
		wg.Add(1)
		go func(url, provider string) {
			defer wg.Done()
			ch <- providers.FetchModels(url, provider)
		}(url, provider)
	}

	wg.Wait()
	close(ch)

	var allModels []providers.ModelsResponse
	for model := range ch {
		allModels = append(allModels, model)
	}

	c.JSON(http.StatusOK, allModels)
}

func (router *RouterImpl) GenerateProvidersTokenHandler(c *gin.Context) {
	var req providers.GenerateRequest
	if err := json.NewDecoder(c.Request.Body).Decode(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Failed to decode request"})
		return
	}

	if req.Model == "" {
		router.logger.Error("model is required", nil)
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Model is required"})
		return
	}

	provider, ok := router.ValidateProvider(c.Param("provider"))
	if !ok {
		router.logger.Error("requested unsupported provider", nil, "provider", provider)
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Requested unsupported provider"})
		return
	}

	providerGenTokensURL := router.cfg.GenTokensEndpoint(provider.GetID())
	providerName := provider.GetName()
	if providerName == "Google" || providerName == "Cloudflare" {
		providerGenTokensURL = strings.Replace(providerGenTokensURL, "{model}", req.Model, 1)
	}

	url := provider.GetProxyURL() + providerGenTokensURL
	var response providers.GenerateResponse

	response, err := generateTokens(provider, url, req.Model, req.Messages, router.client)
	if err != nil {
		router.logger.Error("failed to generate tokens", err, "provider", provider)
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Failed to generate tokens"})
		return
	}

	c.JSON(http.StatusOK, response)
}

func generateTokens(provider providers.Provider, url string, model string, messages []providers.GenerateMessage, client http.Client) (providers.GenerateResponse, error) {
	if provider == nil {
		return providers.GenerateResponse{}, errors.New("provider cannot be nil")
	}

	if url == "" {
		return providers.GenerateResponse{}, errors.New("url cannot be empty")
	}

	payload := provider.BuildGenTokensRequest(model, messages)
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return providers.GenerateResponse{}, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(payloadBytes))
	if err != nil {
		return providers.GenerateResponse{}, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return providers.GenerateResponse{}, fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return providers.GenerateResponse{}, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var response interface{}
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return providers.GenerateResponse{}, fmt.Errorf("failed to decode response: %w", err)
	}

	result, err := provider.BuildGenTokensResponse(model, response)
	if err != nil {
		return providers.GenerateResponse{}, fmt.Errorf("failed to build response: %w", err)
	}

	return result, nil
}
