package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	gin "github.com/gin-gonic/gin"
	api "github.com/inference-gateway/inference-gateway/api"
	middlewares "github.com/inference-gateway/inference-gateway/api/middlewares"
	config "github.com/inference-gateway/inference-gateway/config"
	l "github.com/inference-gateway/inference-gateway/logger"
	otel "github.com/inference-gateway/inference-gateway/otel"
	providers "github.com/inference-gateway/inference-gateway/providers"
	"github.com/sethvargo/go-envconfig"
)

func main() {
	var config config.Config
	cfg, err := config.Load(envconfig.OsLookuper())
	if err != nil {
		log.Printf("Config load error: %v", err)
		return
	}

	var logger l.Logger
	logger, err = l.NewLogger(cfg.Environment)
	if err != nil {
		log.Printf("Logger init error: %v", err)
		return
	}

	// Initialize logger middleware
	loggerMiddleware, err := middlewares.NewLoggerMiddleware(&logger)
	if err != nil {
		logger.Error("Failed to initialize logger middleware: %v", err)
		return
	}

	// Initialize telemetry middleware
	var telemetry middlewares.Telemetry
	if cfg.EnableTelemetry {
		otelImpl := &otel.OpenTelemetryImpl{}
		err = otelImpl.Init(cfg)
		if err != nil {
			logger.Error("OpenTelemetry init error", err)
			return
		}

		telemetry, err = middlewares.NewTelemetryMiddleware(cfg, otelImpl, logger)
		if err != nil {
			logger.Error("Failed to initialize telemetry middleware: %v", err)
			return
		}
	} else {
		telemetry, _ = middlewares.NewTelemetryMiddleware(cfg, nil, logger)
	}

	// Initialize OIDC authenticator middleware
	oidcAuthenticator, err := middlewares.NewOIDCAuthenticatorMiddleware(logger, cfg)
	if err != nil {
		logger.Error("Failed to initialize OIDC authenticator: %v", err)
		return
	}

	scheme := "http"
	if cfg.Server.TlsCertPath != "" && cfg.Server.TlsKeyPath != "" {
		scheme = "https"
	}

	clientConfig, err := providers.NewClientConfig()
	if err != nil {
		log.Printf("fatal: failed to initialize client configuration: %v", err)
		return
	}

	client := providers.NewHTTPClient(clientConfig, scheme, cfg.Server.Host, cfg.Server.Port)
	providerRegistry := providers.NewProviderRegistry(cfg.Providers, logger)

	api := api.NewRouter(cfg, logger, providerRegistry, client)
	r := gin.New()
	r.Use(loggerMiddleware.Middleware())
	if cfg.EnableTelemetry {
		r.Use(telemetry.Middleware())
	}
	r.Use(oidcAuthenticator.Middleware())

	r.GET("/llms", api.ListAllModelsHandler)
	r.GET("/llms/:provider", api.ListModelsHandler)
	r.POST("/llms/:provider/generate", api.GenerateProvidersTokenHandler)
	r.Any("/proxy/:provider/*path", api.ProxyHandler)
	r.GET("/health", api.HealthcheckHandler)
	r.NoRoute(api.NotFoundHandler)

	server := &http.Server{
		Addr:         cfg.Server.Host + ":" + cfg.Server.Port,
		Handler:      r,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
		IdleTimeout:  cfg.Server.IdleTimeout,
	}

	if cfg.Server.TlsCertPath != "" && cfg.Server.TlsKeyPath != "" {
		go func() {
			logger.Info("Starting Inference Gateway with TLS", "port", cfg.Server.Port)

			if err := server.ListenAndServeTLS(cfg.Server.TlsCertPath, cfg.Server.TlsKeyPath); err != nil && err != http.ErrServerClosed {
				logger.Error("ListenAndServeTLS error", err)
			}
		}()
	} else {
		go func() {
			logger.Info("Starting Inference Gateway", "port", cfg.Server.Port)

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
