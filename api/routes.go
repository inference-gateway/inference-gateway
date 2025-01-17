package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"sync"

	"github.com/edenreich/inference-gateway/api/providers"
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

type Provider struct {
	Name     string `json:"name"`
	URL      string `json:"url"`
	ProxyURL string `json:"proxy_url"`
	Token    string `json:"token"`
}

func (router *RouterImpl) ValidateProvider(provider string) (*Provider, bool) {
	cfg := router.cfg
	providers := map[string]Provider{
		"ollama":     {Name: "Ollama", URL: cfg.OllamaAPIURL, ProxyURL: "http://localhost:8080/proxy/ollama", Token: ""},
		"groq":       {Name: "Groq", URL: cfg.GroqAPIURL, ProxyURL: "http://localhost:8080/proxy/groq", Token: cfg.GroqAPIKey},
		"openai":     {Name: "OpenAI", URL: cfg.OpenaiAPIURL, ProxyURL: "http://localhost:8080/proxy/openai", Token: cfg.OpenaiAPIKey},
		"google":     {Name: "Google", URL: cfg.GoogleAIStudioURL, ProxyURL: "http://localhost:8080/proxy/google", Token: cfg.GoogleAIStudioKey},
		"cloudflare": {Name: "Cloudflare", URL: cfg.CloudflareAPIURL, ProxyURL: "http://localhost:8080/proxy/cloudflare", Token: cfg.CloudflareAPIKey},
	}

	p, ok := providers[provider]
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
	} else if provider.Name != "Google" {
		c.Request.Header.Set("Authorization", "Bearer "+provider.Token)
	}

	if provider.Name == "Google" {
		query := c.Request.URL.Query()
		query.Set("key", provider.Token)
		c.Request.URL.RawQuery = query.Encode()
	}

	c.Request.Header.Set("Content-Type", "application/json")
	c.Request.Header.Set("Accept", "application/json")

	remote, _ := url.Parse(provider.URL + c.Request.URL.Path)
	proxy := httputil.NewSingleHostReverseProxy(remote)
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

type ResponseTokens struct {
	Role    string `json:"role"`
	Model   string `json:"model"`
	Content string `json:"content"`
}

type GenerateResponse struct {
	Provider string         `json:"provider"`
	Response ResponseTokens `json:"response"`
}

func (router *RouterImpl) GenerateProvidersTokenHandler(c *gin.Context) {
	var req GenerateRequest
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

	providersEndpoints := map[string]string{
		"Ollama":     "/api/generate",
		"Groq":       "/openai/v1/chat/completions",
		"OpenAI":     "/v1/completions",
		"Google":     "/v1beta/models/{model}:generateContent",
		"Cloudflare": "/ai/run/@cf/meta/{model}",
	}

	url, ok := providersEndpoints[provider.Name]
	if !ok {
		router.logger.Error("requested unsupported provider", nil, "provider", provider)
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Requested unsupported provider"})
		return
	}

	if provider.Name == "Google" || provider.Name == "Cloudflare" {
		url = strings.Replace(url, "{model}", req.Model, 1)
	}

	provider.URL = provider.ProxyURL + url
	var response GenerateResponse

	response, err := generateToken(provider, req.Model, req.Prompt)
	if err != nil {
		router.logger.Error("failed to generate tokens", err, "provider", provider)
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "Failed to generate tokens"})
		return
	}

	c.JSON(http.StatusOK, response)
}

func generateToken(provider *Provider, model string, prompt string) (GenerateResponse, error) {
	var payload interface{}
	var response interface{}
	var role, content string

	switch provider.Name {
	case "Ollama":
		payload = providers.GenerateRequestOllama{
			Model:  model,
			Prompt: prompt,
			Stream: false,
		}
		response = &providers.GenerateResponseOllama{}
	case "Groq":
		payload = providers.GenerateRequestGroq{
			Model: model,
			Messages: []providers.GenerateRequestGroqMessage{
				{
					Role:    "user",
					Content: prompt,
				},
			},
		}
		response = &providers.GenerateResponseGroq{}
	case "OpenAI":
		payload = providers.GenerateRequestOpenAI{
			Model: model,
			Messages: []providers.GenerateRequestOpenAIMessage{
				{
					Role:    "user",
					Content: prompt,
				},
			},
		}
		response = &providers.GenerateResponseOpenAI{}
	case "Google":
		payload = providers.GenerateRequestGoogle{
			Contents: providers.GenerateRequestGoogleContents{
				Parts: []providers.GenerateRequestGoogleParts{
					{
						Text: prompt,
					},
				},
			},
		}
		response = &providers.GenerateResponseGoogle{}
	case "Cloudflare":
		payload = providers.GenerateRequestCloudflare{
			Prompt: prompt,
		}
		response = &providers.GenerateResponseCloudflare{}
	default:
		return GenerateResponse{}, errors.New("provider not implemented")
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return GenerateResponse{}, err
	}

	resp, err := http.Post(provider.URL, "application/json", strings.NewReader(string(payloadBytes)))
	if err != nil {
		return GenerateResponse{}, err
	}
	defer resp.Body.Close()

	err = json.NewDecoder(resp.Body).Decode(response)
	if err != nil {
		return GenerateResponse{}, err
	}

	switch provider.Name {
	case "Ollama":
		ollamaResponse := response.(*providers.GenerateResponseOllama)
		if ollamaResponse.Response != "" {
			role = "assistant" // It's not provided by Ollama so we set it to assistant
			content = ollamaResponse.Response
		} else {
			return GenerateResponse{}, errors.New("invalid response from Ollama")
		}
	case "Groq":
		groqResponse := response.(*providers.GenerateResponseGroq)
		if len(groqResponse.Choices) > 0 && len(groqResponse.Choices[0].Message.Content) > 0 {
			role = groqResponse.Choices[0].Message.Role
			content = groqResponse.Choices[0].Message.Content
		} else {
			return GenerateResponse{}, errors.New("invalid response from Groq")
		}
	case "OpenAI":
		openAIResponse := response.(*providers.GenerateResponseOpenAI)
		if len(openAIResponse.Choices) > 0 && len(openAIResponse.Choices[0].Message.Content) > 0 {
			role = openAIResponse.Choices[0].Message.Role
			content = openAIResponse.Choices[0].Message.Content
		} else {
			return GenerateResponse{}, errors.New("invalid response from OpenAI")
		}
	case "Google":
		googleResponse := response.(*providers.GenerateResponseGoogle)
		if len(googleResponse.Candidates) > 0 && len(googleResponse.Candidates[0].Content.Parts) > 0 {
			role = googleResponse.Candidates[0].Content.Role
			content = googleResponse.Candidates[0].Content.Parts[0].Text
		} else {
			return GenerateResponse{}, errors.New("invalid response from Google")
		}
	case "Cloudflare":
		cloudflareResponse := response.(*providers.GenerateResponseCloudflare)
		if cloudflareResponse.Result.Response != "" {
			role = "assistant" // It's not provided by Cloudflare so we set it to assistant
			content = cloudflareResponse.Result.Response
		} else {
			return GenerateResponse{}, errors.New("invalid response from Cloudflare")
		}
	}

	return GenerateResponse{Provider: provider.Name, Response: ResponseTokens{
		Role:    role,
		Model:   model,
		Content: content,
	}}, nil
}
