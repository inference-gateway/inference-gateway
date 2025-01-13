package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	api "github.com/edenreich/inference-gateway/api"
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

	api := &api.RouterImpl{
		Logger: logger,
	}
	http.HandleFunc("/llms", api.FetchAllModelsHandler)
	http.HandleFunc("/health", api.Healthcheck)

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
