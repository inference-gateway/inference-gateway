package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"

	l "github.com/edenreich/inference-gateway/logger"
)

type Router interface {
	FetchAllModelsHandler(w http.ResponseWriter, r *http.Request)
}

type RouterImpl struct {
	Logger l.Logger
}

type ErrorResponse struct {
	Error string `json:"error"`
}

type Response struct {
	Message string `json:"message"`
}

func (router *RouterImpl) errorResponseJSON(w http.ResponseWriter, err error, status int) {
	var response ErrorResponse
	response.Error = err.Error()
	w.Header().Set("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(&response)
	if err != nil {
		router.Logger.Error("response failed", err)
	}
	http.Error(w, err.Error(), status)
}

func (router *RouterImpl) responseJSON(w http.ResponseWriter, data interface{}, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	err := json.NewEncoder(w).Encode(data)
	if err != nil {
		router.Logger.Error("response failed", err)
	}
}

func (router *RouterImpl) Healthcheck(w http.ResponseWriter, r *http.Request) {
	router.Logger.Debug("Healthcheck")
	router.responseJSON(w, Response{Message: "OK"}, http.StatusOK)
}

type ModelResponse struct {
	Provider string        `json:"provider"`
	Models   []interface{} `json:"models"`
}

func (router *RouterImpl) FetchAllModelsHandler(w http.ResponseWriter, r *http.Request) {
	var wg sync.WaitGroup
	modelProviders := map[string]string{
		"ollama":     "http://localhost:8080/llms/ollama/v1/models",
		"groq":       "http://localhost:8080/llms/groq/openai/v1/models",
		"openai":     "http://localhost:8080/llms/openai/v1/models",
		"google":     "http://localhost:8080/llms/google/v1beta/models",
		"cloudflare": "http://localhost:8080/llms/cloudflare/ai/finetunes/public",
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

	router.responseJSON(w, allModels, http.StatusOK)
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

func (router *RouterImpl) GenerateProvidersTokenHandler(w http.ResponseWriter, r *http.Request) {
	var req GenerateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		router.errorResponseJSON(w, fmt.Errorf("invalid request payload. %w", err), http.StatusBadRequest)
		return
	}

	provider := r.PathValue("provider")
	providers := map[string]string{
		"ollama":     "http://localhost:8080/llms/ollama/api/generate",
		"groq":       "http://localhost:8080/llms/groq/openai/v1/chat/completions",
		"openai":     "http://localhost:8080/llms/openai/v1/completions",
		"google":     "http://localhost:8080/llms/google/v1beta/models/{model}:generateContent",
		"cloudflare": "http://localhost:8080/llms/cloudflare/ai/run/@cf/meta/{model}",
	}

	url, ok := providers[provider]
	if !ok {
		router.errorResponseJSON(w, fmt.Errorf("requested unsupported provider"), http.StatusBadRequest)
		return
	}

	if provider == "google" || provider == "cloudflare" {
		url = strings.Replace(url, "{model}", req.Model, 1)
	}

	response := generateToken(url, provider, req)
	router.responseJSON(w, response, http.StatusOK)
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
