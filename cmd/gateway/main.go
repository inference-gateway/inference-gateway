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
	middlewares "github.com/edenreich/inference-gateway/api/middlewares"
	config "github.com/edenreich/inference-gateway/config"
	l "github.com/edenreich/inference-gateway/logger"
	otel "github.com/edenreich/inference-gateway/otel"
	gin "github.com/gin-gonic/gin"
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
	logger, err = l.NewLogger(cfg.Environment)
	if err != nil {
		log.Printf("Logger init error: %v", err)
		return
	}

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
		logger.Info("OpenTelemetry is disabled")
	}

	ctx := context.Background()
	var span otel.TraceSpan
	if cfg.EnableTelemetry {
		_, span = tp.Tracer(cfg.ApplicationName).Start(ctx, "main")
		defer span.End()
	}

	oidcAuthenticator, err := middlewares.NewOIDCAuthenticator(logger, cfg)
	if err != nil {
		logger.Error("Failed to initialize OIDC authenticator: %v", err)
		return
	}

	api := api.NewRouter(cfg, logger, tp)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		logger.Info("Request received", "method", c.Request.Method, "host", c.Request.Host, "path", c.Request.URL.Path)
		c.Next()
	})
	r.Use(oidcAuthenticator.Middleware())

	r.GET("/llms", api.FetchAllModelsHandler)
	r.POST("/llms/:provider/generate", api.GenerateProvidersTokenHandler)
	r.GET("/proxy/:provider/*path", api.ProxyHandler)
	r.POST("/proxy/:provider/*path", api.ProxyHandler)
	r.GET("/health", api.HealthcheckHandler)
	r.NoRoute(api.NotFoundHandler)

	server := &http.Server{
		Addr:         cfg.ServerHost + ":" + cfg.ServerPort,
		Handler:      r,
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
