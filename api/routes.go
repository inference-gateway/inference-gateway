package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
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

func (router *RouterImpl) NotFoundHandler(c *gin.Context) {
	router.logger.Error("requested route is not found", nil)
	c.JSON(http.StatusNotFound, ErrorResponse{Error: "Requested route is not found"})
}

func (router *RouterImpl) ProxyHandler(c *gin.Context) {
	p := c.Param("provider")
	provider, err := providers.GetProvider(router.cfg.Providers, p)
	if err != nil {
		router.logger.Error("requested unsupported provider", err, "provider", p)
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
	case "query":
		query := c.Request.URL.Query()
		query.Set("key", token)
		c.Request.URL.RawQuery = query.Encode()
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

	// Add error handler for proxy failures
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
	p := providers.GetProviders(router.cfg.Providers)

	ch := make(chan providers.ModelsResponse, len(p))
	for _, provider := range p {
		wg.Add(1)
		go func(provider providers.Provider) {
			defer wg.Done()
			ch <- provider.ListModels()
		}(provider)
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

	provider, err := providers.GetProvider(router.cfg.Providers, c.Param("provider"))
	if err != nil {
		router.logger.Error("requested unsupported provider", err, "provider", c.Param("provider"))
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Requested unsupported provider"})
		return
	}

	var response providers.GenerateResponse

	response, err = provider.GenerateTokens(req.Model, req.Messages, router.client)
	if err != nil {
		router.logger.Error("failed to generate tokens", err, "provider", provider)
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Failed to generate tokens"})
		return
	}

	c.JSON(http.StatusOK, response)
}
