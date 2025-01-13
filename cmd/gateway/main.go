package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	config "github.com/edenreich/inference-gateway/config"
	gateway "github.com/edenreich/inference-gateway/gateway"
	l "github.com/edenreich/inference-gateway/logger"
	otel "github.com/edenreich/inference-gateway/otel"
)

func main() {
	var config config.Config
	cfg, err := config.Load()
	if err != nil {
		log.Printf("Config load error: %v", err)
		return
	}

	var tp otel.TracerProvider
	var logger l.Logger

	if cfg.EnableTelemetry {
		otel := &otel.OpenTelemetryImpl{}
		tp, err = otel.Init(cfg)
		if err != nil {
			logger.Error("OpenTelemetry init error", err)
			return
		}
		defer func() {
			if err := tp.Shutdown(context.Background()); err != nil {
				logger.Error("Tracer shutdown error", err)
			}
		}()
		logger.Info("OpenTelemetry initialized")
	} else {
		logger = l.NewLogger(cfg.Environment)
		logger.Info("OpenTelemetry is disabled")
	}

	ctx := context.Background()
	var span otel.TraceSpan
	if cfg.EnableTelemetry {
		_, span = tp.Tracer(cfg.ApplicationName).Start(ctx, "main")
		defer span.End()
	}

	http.HandleFunc("/llms/ollama/", gateway.Create(cfg.OllamaAPIURL, "", "/llms/ollama/", tp, cfg.EnableTelemetry, logger))
	http.HandleFunc("/llms/groq/", gateway.Create(cfg.GroqAPIURL, cfg.GroqAPIKey, "/llms/groq/", tp, cfg.EnableTelemetry, logger))
	http.HandleFunc("/llms/openai/", gateway.Create(cfg.OpenaiAPIURL, cfg.OpenaiAPIKey, "/llms/openai/", tp, cfg.EnableTelemetry, logger))
	http.HandleFunc("/llms/google/", gateway.Create(cfg.GoogleAIStudioURL, cfg.GoogleAIStudioKey, "/llms/google/", tp, cfg.EnableTelemetry, logger))
	http.HandleFunc("/llms/cloudflare/", gateway.Create(cfg.CloudflareAPIURL, cfg.CloudflareAPIKey, "/llms/cloudflare/", tp, cfg.EnableTelemetry, logger))

	http.HandleFunc("/llms", fetchAllModelsHandler)
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	server := &http.Server{
		Addr:         cfg.ServerHost + ":" + cfg.ServerPort,
		ReadTimeout:  cfg.ServerReadTimeout,
		WriteTimeout: cfg.ServerWriteTimeout,
		IdleTimeout:  cfg.ServerIdleTimeout,
	}

	if cfg.ServerTLSCertPath != "" && cfg.ServerTLSKeyPath != "" {
		go func() {
			if cfg.EnableTelemetry {
				span.AddEvent("Starting Inference Gateway with TLS")
			}
			logger.Info("Starting Inference Gateway with TLS", "port", cfg.ServerPort)

			if err := server.ListenAndServeTLS(cfg.ServerTLSCertPath, cfg.ServerTLSKeyPath); err != nil && err != http.ErrServerClosed {
				logger.Error("ListenAndServeTLS error", err)
			}
		}()
	} else {
		go func() {
			if cfg.EnableTelemetry {
				span.AddEvent("Starting Inference Gateway")
			}
			logger.Info("Starting Inference Gateway", "port", cfg.ServerPort)

			if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				logger.Error("ListenAndServe error", err)
			}
		}()
	}

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit
	logger.Info("Shutting down server...")

	ctxShutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(ctxShutdown); err != nil {
		logger.Error("Server Shutdown error", err)
	} else {
		logger.Info("Server gracefully stopped")
	}
}

type ModelResponse struct {
	Provider string        `json:"provider"`
	Models   []interface{} `json:"models"`
}

func fetchModels(url string, provider string, wg *sync.WaitGroup, ch chan<- ModelResponse) {
	defer wg.Done()
	resp, err := http.Get(url)
	if err != nil {
		ch <- ModelResponse{Provider: provider, Models: nil}
		return
	}
	defer resp.Body.Close()

	if provider == "Google" {
		var response struct {
			Models []interface{} `json:"models"`
		}
		if err = json.NewDecoder(resp.Body).Decode(&response); err != nil {
			ch <- ModelResponse{Provider: provider, Models: nil}
			return
		}
		ch <- ModelResponse{Provider: provider, Models: response.Models}
		return
	}

	var response struct {
		Object string        `json:"object"`
		Data   []interface{} `json:"data"`
	}
	if err = json.NewDecoder(resp.Body).Decode(&response); err != nil {
		ch <- ModelResponse{Provider: provider, Models: nil}
		return
	}
	ch <- ModelResponse{Provider: provider, Models: response.Data}
}

func fetchAllModelsHandler(w http.ResponseWriter, r *http.Request) {
	var wg sync.WaitGroup
	modelProviders := map[string]string{
		"Ollama":     "http://localhost:8080/llms/ollama/v1/models",
		"Groq":       "http://localhost:8080/llms/groq/openai/v1/models",
		"OpenAI":     "http://localhost:8080/llms/openai/v1/models",
		"Google":     "http://localhost:8080/llms/google/v1beta/models",
		"Cloudflare": "http://localhost:8080/llms/cloudflare/ai/models",
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
