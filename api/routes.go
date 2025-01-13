package api

import (
	"encoding/json"
	"net/http"
	"sync"

	l "github.com/edenreich/inference-gateway/logger"
)

type Router interface {
	FetchAllModelsHandler(w http.ResponseWriter, r *http.Request)
}

type RouterImpl struct {
	Logger l.Logger
}

func (router *RouterImpl) Healthcheck(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]string{"status": "ok"}); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

type ModelResponse struct {
	Provider string        `json:"provider"`
	Models   []interface{} `json:"models"`
}

func (router *RouterImpl) FetchAllModelsHandler(w http.ResponseWriter, r *http.Request) {
	var wg sync.WaitGroup
	modelProviders := map[string]string{
		"Ollama":     "http://localhost:8080/llms/ollama/v1/models",
		"Groq":       "http://localhost:8080/llms/groq/openai/v1/models",
		"OpenAI":     "http://localhost:8080/llms/openai/v1/models",
		"Google":     "http://localhost:8080/llms/google/v1beta/models",
		"Cloudflare": "http://localhost:8080/llms/cloudflare/ai/finetunes/public",
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

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(allModels); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func fetchModels(url string, provider string, wg *sync.WaitGroup, ch chan<- ModelResponse) {
	defer wg.Done()
	resp, err := http.Get(url)
	if err != nil {
		ch <- ModelResponse{Provider: provider, Models: []interface{}{}}
		return
	}
	defer resp.Body.Close()

	if provider == "Google" {
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

	if provider == "Cloudflare" {
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
