package api

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"sync"

	gin "github.com/gin-gonic/gin"
	config "github.com/inference-gateway/inference-gateway/config"
	l "github.com/inference-gateway/inference-gateway/logger"
	otel "github.com/inference-gateway/inference-gateway/otel"
	providers "github.com/inference-gateway/inference-gateway/providers"
	semconv "go.opentelemetry.io/otel/semconv/v1.17.0"
	trace "go.opentelemetry.io/otel/trace"
)

type Router interface {
	NotFoundHandler(c *gin.Context)
	ProxyHandler(c *gin.Context)
	HealthcheckHandler(c *gin.Context)
	FetchAllModelsHandler(c *gin.Context)
	GenerateProvidersTokenHandler(c *gin.Context)
	ValidateProvider(provider string) (*providers.Provider, bool)
}

type RouterImpl struct {
	cfg    config.Config
	logger l.Logger
	tp     otel.TracerProvider
}

type ErrorResponse struct {
	Error string `json:"error"`
}

type ResponseJSON struct {
	Message string `json:"message"`
}

func NewRouter(cfg config.Config, logger l.Logger, tp otel.TracerProvider) Router {
	return &RouterImpl{
		cfg,
		logger,
		tp,
	}
}

func (router *RouterImpl) ValidateProvider(provider string) (*providers.Provider, bool) {
	p, ok := router.cfg.Providers()[provider]
	if !ok {
		return nil, false
	}

	return &p, ok
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

	if router.cfg.EnableTelemetry {
		ctx := c.Request.Context()
		_, span := router.tp.Tracer("inference-gateway").Start(ctx, "proxy-request")
		defer span.End()
		span.AddEvent("Proxying request", trace.WithAttributes(
			semconv.HTTPMethodKey.String(c.Request.Method),
			semconv.HTTPTargetKey.String(c.Request.URL.String()),
			semconv.HTTPRequestContentLengthKey.Int64(c.Request.ContentLength),
		))
	}

	c.Request.URL.Path = strings.TrimPrefix(c.Request.URL.Path, "/proxy/"+p)

	if provider.Token == "" && provider.Name != "Ollama" {
		router.logger.Error("provider token is missing", nil, "provider", provider)
		c.JSON(http.StatusUnprocessableEntity, ErrorResponse{Error: "Provider token is missing"})
		return
	} else if provider.Name != "Google" && provider.Name != "Anthropic" {
		c.Request.Header.Set("Authorization", "Bearer "+provider.Token)
	}

	if provider.Name == "Google" {
		query := c.Request.URL.Query()
		query.Set("key", provider.Token)
		c.Request.URL.RawQuery = query.Encode()
	}

	if provider.Name == "Anthropic" {
		c.Request.Header.Set("x-api-key", provider.Token)
		c.Request.Header.Set("anthropic-version", "2023-06-01")
	}

	c.Request.Header.Set("Content-Type", "application/json")
	c.Request.Header.Set("Accept", "application/json")

	remote, _ := url.Parse(provider.URL + c.Request.URL.Path)
	proxy := httputil.NewSingleHostReverseProxy(remote)

	if router.cfg.Environment == "development" {
		proxy.ModifyResponse = func(resp *http.Response) error {
			bodyBytes, err := io.ReadAll(resp.Body)
			if err != nil {
				router.logger.Error("Failed to read response from proxy", err)
				return err
			}

			// Always restore the body
			defer func() {
				resp.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
			}()

			// Only attempt to parse JSON responses
			if strings.Contains(resp.Header.Get("Content-Type"), "application/json") {
				contentBody := bodyBytes

				// Handle gzipped content only if we have actual content
				if resp.Header.Get("Content-Encoding") == "gzip" && len(bodyBytes) > 0 {
					reader, err := gzip.NewReader(bytes.NewReader(bodyBytes))
					if err != nil {
						router.logger.Error("Invalid gzip content", err)
					} else {
						defer reader.Close()
						if decompressed, err := io.ReadAll(reader); err == nil {
							contentBody = decompressed
						} else {
							router.logger.Error("Failed to read gzipped content", err)
						}
					}
				}

				// Try to parse as JSON regardless of gzip success/failure
				var body interface{}
				if err := json.Unmarshal(contentBody, &body); err != nil {
					router.logger.Error("Failed to unmarshal JSON response",
						err,
						"status", resp.StatusCode,
						"content-type", resp.Header.Get("Content-Type"),
						"content-encoding", resp.Header.Get("Content-Encoding"),
						"content-length", len(contentBody))
				} else {
					router.logger.Debug("Proxy response", "body", body)
				}
			}

			return nil
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
	modelProviders := router.cfg.GetEndpointsListModels()

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

	provider, ok := router.ValidateProvider(c.Param("provider"))
	if !ok {
		router.logger.Error("requested unsupported provider", nil, "provider", provider)
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Requested unsupported provider"})
		return
	}

	providerGenTokensURL := router.cfg.GetEndpointsGenerateTokens(provider.ID)

	if provider.Name == "Google" || provider.Name == "Cloudflare" {
		providerGenTokensURL = strings.Replace(providerGenTokensURL, "{model}", req.Model, 1)
	}

	provider.URL = provider.ProxyURL + providerGenTokensURL
	var response providers.GenerateResponse

	response, err := generateTokens(provider, req.Model, req.Messages)
	if err != nil {
		router.logger.Error("failed to generate tokens", err, "provider", provider)
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "Failed to generate tokens"})
		return
	}

	c.JSON(http.StatusOK, response)
}

func generateTokens(provider *providers.Provider, model string, messages []providers.GenerateMessage) (providers.GenerateResponse, error) {
	payload := provider.BuildGenTokensRequest(model, messages)

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return providers.GenerateResponse{}, err
	}

	resp, err := http.Post(provider.URL, "application/json", strings.NewReader(string(payloadBytes)))
	if err != nil {
		return providers.GenerateResponse{}, err
	}
	defer resp.Body.Close()

	var response interface{}
	err = json.NewDecoder(resp.Body).Decode(response)
	if err != nil {
		return providers.GenerateResponse{}, err
	}

	r, err := provider.BuildGenTokensResponse(model, response)
	if err != nil {
		return providers.GenerateResponse{}, err
	}

	return r, nil
}
