package api

import (
	"encoding/json"
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
	ProxyHandler(c *gin.Context)
	ListAllModelsHandler(c *gin.Context)
	ListModelsHandler(c *gin.Context)
	GenerateProvidersTokenHandler(c *gin.Context)
	HealthcheckHandler(c *gin.Context)
	NotFoundHandler(c *gin.Context)
}

type RouterImpl struct {
	cfg    config.Config
	logger l.Logger
	client providers.Client
}

type ErrorResponse struct {
	Error string `json:"error"`
}

type ResponseJSON struct {
	Message string `json:"message"`
}

func NewRouter(cfg config.Config, logger *l.Logger, client providers.Client) Router {
	return &RouterImpl{
		cfg,
		*logger,
		client,
	}
}

func (router *RouterImpl) NotFoundHandler(c *gin.Context) {
	router.logger.Error("requested route is not found", nil)
	c.JSON(http.StatusNotFound, ErrorResponse{Error: "Requested route is not found"})
}

func (router *RouterImpl) ProxyHandler(c *gin.Context) {
	p := c.Param("provider")
	provider, err := providers.NewProvider(router.cfg.Providers, p, &router.logger, &router.client)
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

	// Setup common headers
	c.Request.Header.Set("Content-Type", "application/json")
	c.Request.Header.Set("Accept", "application/json")

	// Create and configure proxy
	remote, _ := url.Parse(provider.GetURL() + c.Request.URL.Path)
	proxy := httputil.NewSingleHostReverseProxy(remote)

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

	// Configure proxy director
	proxy.Director = func(req *http.Request) {
		req.Header = c.Request.Header
		req.Host = remote.Host
		req.URL.Host = remote.Host
		req.URL.Scheme = remote.Scheme
		req.URL.Path = c.Param("path")

		if router.cfg.Environment == "development" {
			router.logger.Debug("proxying request",
				"from", c.Request.URL.String(),
				"to", req.URL.String(),
				"method", req.Method,
			)
		}
	}

	// Log proxy responses in development mode only
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

type ModelResponse struct {
	Provider string        `json:"provider"`
	Models   []interface{} `json:"models"`
}

func (router *RouterImpl) ListModelsHandler(c *gin.Context) {
	provider, err := providers.NewProvider(router.cfg.Providers, c.Param("provider"), &router.logger, &router.client)
	if err != nil {
		if strings.Contains(err.Error(), "token not configured") {
			router.logger.Error("provider requires authentication but no API key was configured", err, "provider", provider)
			c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Provider requires an API key. Please configure the provider's API key."})
			return
		}
		router.logger.Error("provider not found or not supported", err, "provider", provider)
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Provider not found. Please check the list of supported providers."})
		return
	}

	response, err := provider.ListModels()
	if err != nil {
		router.logger.Error("failed to list models", err, "provider", provider)
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Failed to list models"})
		return
	}

	c.JSON(http.StatusOK, response)
}

func (router *RouterImpl) ListAllModelsHandler(c *gin.Context) {
	var wg sync.WaitGroup
	providersCfg := router.cfg.Providers

	ch := make(chan providers.ListModelsResponse, len(providersCfg))

	for providerID := range providersCfg {
		wg.Add(1)
		go func(id string) {
			defer wg.Done()

			provider, err := providers.NewProvider(providersCfg, id, &router.logger, &router.client)
			if err != nil {
				router.logger.Error("failed to create provider", err)
				ch <- providers.ListModelsResponse{
					Provider: id,
					Models:   []providers.Model{},
				}
				return
			}

			response, err := provider.ListModels()
			if err != nil {
				router.logger.Error("failed to list models", err, "provider", provider)
				ch <- providers.ListModelsResponse{
					Provider: id,
					Models:   []providers.Model{},
				}
				return
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
	if err := json.NewDecoder(c.Request.Body).Decode(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Failed to decode request"})
		return
	}

	if req.Model == "" {
		router.logger.Error("model is required", nil)
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Model is required"})
		return
	}

	provider, err := providers.NewProvider(router.cfg.Providers, c.Param("provider"), &router.logger, &router.client)
	if err != nil {
		if strings.Contains(err.Error(), "token not configured") {
			router.logger.Error("provider requires authentication but no API key was configured", err, "provider", provider)
			c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Provider requires an API key. Please configure the provider's API key."})
			return
		}
		router.logger.Error("provider not found or not supported", err, "provider", provider)
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Provider not found. Please check the list of supported providers."})
		return
	}

	var response providers.GenerateResponse
	response, err = provider.GenerateTokens(req.Model, req.Messages)
	if err != nil {
		router.logger.Error("failed to generate tokens", err, "provider", provider)
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Failed to generate tokens"})
		return
	}

	c.JSON(http.StatusOK, response)
}
