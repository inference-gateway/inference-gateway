package api

import (
	"encoding/json"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"sync"

	config "github.com/edenreich/inference-gateway/config"
	l "github.com/edenreich/inference-gateway/logger"
	"github.com/edenreich/inference-gateway/otel"
	"github.com/gin-gonic/gin"
	semconv "go.opentelemetry.io/otel/semconv/v1.17.0"
	"go.opentelemetry.io/otel/trace"
)

type Router interface {
	NotFoundHandler(c *gin.Context)
	ProxyHandler(c *gin.Context)
	HealthcheckHandler(c *gin.Context)
	FetchAllModelsHandler(c *gin.Context)
	GenerateProvidersTokenHandler(c *gin.Context)
	ValidateProvider(provider string) (*Provider, bool)
}

type RouterImpl struct {
	cfg    config.Config
	logger l.Logger
	tp     otel.TracerProvider
}

type ErrorResponse struct {
	Error string `json:"error"`
}

type Response struct {
	Message string `json:"message"`
}

func NewRouter(cfg config.Config, logger l.Logger, tp otel.TracerProvider) Router {
	return &RouterImpl{
		cfg,
		logger,
		tp,
	}
}

type Provider struct {
	Name  string `json:"name"`
	URL   string `json:"url"`
	Token string `json:"token"`
}

func (router *RouterImpl) ValidateProvider(provider string) (*Provider, bool) {
	cfg := router.cfg
	providers := map[string]Provider{
		"ollama":     {Name: "Ollama", URL: cfg.OllamaAPIURL, Token: ""},
		"groq":       {Name: "Groq", URL: cfg.GroqAPIURL, Token: cfg.GroqAPIKey},
		"openai":     {Name: "OpenAI", URL: cfg.OpenaiAPIURL, Token: cfg.OpenaiAPIKey},
		"google":     {Name: "Google", URL: cfg.GoogleAIStudioURL, Token: cfg.GoogleAIStudioKey},
		"cloudflare": {Name: "Cloudflare", URL: cfg.CloudflareAPIURL, Token: cfg.CloudflareAPIKey},
	}

	p, ok := providers[provider]
	if !ok {
		return nil, false
	}

	return &p, ok
}

func (router *RouterImpl) NotFoundHandler(c *gin.Context) {
	router.logger.Error("Requested route is not found", nil)
	c.JSON(http.StatusNotFound, ErrorResponse{Error: "Requested route is not found"})
}

func (router *RouterImpl) ProxyHandler(c *gin.Context) {
	p := c.Param("provider")
	provider, ok := router.ValidateProvider(p)
	if !ok {
		router.logger.Error("Requested unsupported provider", nil, "provider", provider)
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
		c.JSON(http.StatusUnprocessableEntity, ErrorResponse{Error: "Provider token is missing"})
		return
	} else if provider.Name != "Google" {
		c.Request.Header.Set("Authorization", "Bearer "+provider.Token)
	}

	if provider.Name == "Google" {
		query := c.Request.URL.Query()
		query.Set("key", provider.Token)
		c.Request.URL.RawQuery = query.Encode()
	}

	remote, _ := url.Parse(provider.URL)
	proxy := httputil.NewSingleHostReverseProxy(remote)
	proxy.Director = func(req *http.Request) {
		req.Header = c.Request.Header
		req.Host = remote.Host
		req.URL.Host = remote.Host
		req.URL.Scheme = remote.Scheme
	}
	proxy.ServeHTTP(c.Writer, c.Request)
}

func (router *RouterImpl) HealthcheckHandler(c *gin.Context) {
	router.logger.Debug("Healthcheck")
	c.JSON(http.StatusOK, Response{Message: "OK"})
}

type ModelResponse struct {
	Provider string        `json:"provider"`
	Models   []interface{} `json:"models"`
}

func (router *RouterImpl) FetchAllModelsHandler(c *gin.Context) {
	var wg sync.WaitGroup
	modelProviders := map[string]string{
		"ollama":     "http://localhost:8080/proxy/ollama/v1/models",
		"groq":       "http://localhost:8080/proxy/groq/openai/v1/models",
		"openai":     "http://localhost:8080/proxy/openai/v1/models",
		"google":     "http://localhost:8080/proxy/google/v1beta/models",
		"cloudflare": "http://localhost:8080/proxy/cloudflare/ai/finetunes/public",
	}

	ch := make(chan ModelResponse, len(modelProviders))
	for provider, url := range modelProviders {
		wg.Add(1)
		go fetchModels(url, provider, &wg, ch)
	}

	wg.Wait()
	close(ch)

	var allModels []ModelResponse
	for model := range ch {
		allModels = append(allModels, model)
	}

	c.JSON(http.StatusOK, allModels)
}

func fetchModels(url string, provider string, wg *sync.WaitGroup, ch chan<- ModelResponse) {
	defer wg.Done()
	resp, err := http.Get(url)
	if err != nil {
		ch <- ModelResponse{Provider: provider, Models: []interface{}{}}
		return
	}
	defer resp.Body.Close()

	if provider == "google" {
		var response struct {
			Models []interface{} `json:"models"`
		}
		if err = json.NewDecoder(resp.Body).Decode(&response); err != nil {
			ch <- ModelResponse{Provider: provider, Models: []interface{}{}}
			return
		}
		ch <- ModelResponse{Provider: provider, Models: response.Models}
		return
	}

	if provider == "cloudflare" {
		var response struct {
			Result []interface{} `json:"result"`
		}
		if err = json.NewDecoder(resp.Body).Decode(&response); err != nil {
			ch <- ModelResponse{Provider: provider, Models: []interface{}{}}
			return
		}
		ch <- ModelResponse{Provider: provider, Models: response.Result}
		return
	}

	var response struct {
		Object string        `json:"object"`
		Data   []interface{} `json:"data"`
	}
	if err = json.NewDecoder(resp.Body).Decode(&response); err != nil {
		ch <- ModelResponse{Provider: provider, Models: []interface{}{}}
		return
	}
	ch <- ModelResponse{Provider: provider, Models: response.Data}
}

type GenerateRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
}

type GenerateResponse struct {
	Provider string `json:"provider"`
	Response string `json:"response"`
}

func (router *RouterImpl) GenerateProvidersTokenHandler(c *gin.Context) {
	var req GenerateRequest
	if err := json.NewDecoder(c.Request.Body).Decode(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Failed to decode request"})
		return
	}

	provider := c.Param("provider")
	providers := map[string]string{
		"ollama":     "http://localhost:8080/proxy/ollama/api/generate",
		"groq":       "http://localhost:8080/proxy/groq/openai/v1/chat/completions",
		"openai":     "http://localhost:8080/proxy/openai/v1/completions",
		"google":     "http://localhost:8080/proxy/google/v1beta/models/{model}:generateContent",
		"cloudflare": "http://localhost:8080/proxy/cloudflare/ai/run/@cf/meta/{model}",
	}

	url, ok := providers[provider]
	if !ok {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Requested unsupported provider"})
		return
	}

	if provider == "google" || provider == "cloudflare" {
		url = strings.Replace(url, "{model}", req.Model, 1)
	}

	response := generateToken(url, provider, req)
	c.JSON(http.StatusOK, response)
}

func generateToken(url string, provider string, req GenerateRequest) GenerateResponse {
	payload, err := json.Marshal(req)
	if err != nil {
		return GenerateResponse{Provider: provider, Response: "Failed to marshal request payload"}
	}

	resp, err := http.Post(url, "application/json", strings.NewReader(string(payload)))
	if err != nil {
		return GenerateResponse{Provider: provider, Response: "Failed to generate token"}
	}
	defer resp.Body.Close()

	var response struct {
		Content string `json:"content"`
	}
	if err = json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return GenerateResponse{Provider: provider, Response: "Failed to decode response"}
	}

	return GenerateResponse{Provider: provider, Response: response.Content}
}
