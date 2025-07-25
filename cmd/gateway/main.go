package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	gin "github.com/gin-gonic/gin"
	"github.com/inference-gateway/inference-gateway/a2a"
	api "github.com/inference-gateway/inference-gateway/api"
	middlewares "github.com/inference-gateway/inference-gateway/api/middlewares"
	config "github.com/inference-gateway/inference-gateway/config"
	l "github.com/inference-gateway/inference-gateway/logger"
	"github.com/inference-gateway/inference-gateway/mcp"
	otel "github.com/inference-gateway/inference-gateway/otel"
	providers "github.com/inference-gateway/inference-gateway/providers"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sethvargo/go-envconfig"
)

func main() {
	var config config.Config
	cfg, err := config.Load(envconfig.OsLookuper())
	if err != nil {
		log.Printf("{\"error\": \"config load error: %v\"}", err)
		return
	}

	// Initialize logger
	var logger l.Logger
	logger, err = l.NewLogger(cfg.Environment)
	if err != nil {
		log.Printf("{\"error\": \"logger init error: %v\"}", err)
		return
	}

	// Log config in debug mode
	logger.Debug("loaded config", "config", cfg.String())

	// Initialize OpenTelemetry Prometheus exporter Server
	var telemetryImpl otel.OpenTelemetry
	if cfg.Telemetry.Enable {
		telemetryImpl = &otel.OpenTelemetryImpl{}
		err := telemetryImpl.Init(cfg, logger)
		if err != nil {
			logger.Error("opentelemetry initialization failed", err)
			return
		}

		metricsMux := http.NewServeMux()
		metricsMux.Handle("/metrics", promhttp.Handler())

		logger.Info("telemetry initialized successfully")

		metricsServer := &http.Server{
			Addr:         ":" + cfg.Telemetry.MetricsPort,
			Handler:      metricsMux,
			ReadTimeout:  10 * time.Second,
			WriteTimeout: 10 * time.Second,
			IdleTimeout:  30 * time.Second,
		}

		go func() {
			logger.Info("starting metrics server", "port", cfg.Telemetry.MetricsPort)
			if err := metricsServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				logger.Error("metrics server failed", err)
			}
		}()

		defer func() {
			logger.Info("shutting down metrics server...")
			ctxMetrics, cancelMetrics := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancelMetrics()

			if err := metricsServer.Shutdown(ctxMetrics); err != nil {
				logger.Error("metrics server shutdown error", err)
			} else {
				logger.Info("metrics server gracefully stopped")
			}
		}()

		defer func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := telemetryImpl.ShutDown(ctx); err != nil {
				logger.Error("error shutting down telemetry", err)
			}
		}()
	}

	// Initialize logger middleware
	loggerMiddleware, err := middlewares.NewLoggerMiddleware(&logger)
	if err != nil {
		logger.Error("failed to initialize logger middleware", err)
		return
	}

	// Initialize telemetry middleware
	var telemetry middlewares.Telemetry
	if cfg.Telemetry.Enable {
		telemetry, err = middlewares.NewTelemetryMiddleware(cfg, telemetryImpl, logger)
		if err != nil {
			logger.Error("failed to initialize telemetry middleware", err)
			return
		}
	}

	// Initialize OIDC authenticator middleware
	oidcAuthenticator, err := middlewares.NewOIDCAuthenticatorMiddleware(logger, cfg)
	if err != nil {
		logger.Error("failed to initialize oidc authenticator", err)
		return
	}

	// Initialize provider registry and HTTP client
	clientConfig, err := providers.NewClientConfig()
	if err != nil {
		log.Printf("fatal: failed to initialize client configuration: %v", err)
		return
	}

	scheme := "http"
	if cfg.Server.TlsCertPath != "" && cfg.Server.TlsKeyPath != "" {
		scheme = "https"
	}

	client := providers.NewHTTPClient(clientConfig, scheme, cfg.Server.Host, cfg.Server.Port)
	providerRegistry := providers.NewProviderRegistry(cfg.Providers, logger)

	// Initialize MCP middleware if enabled
	var mcpClient mcp.MCPClientInterface
	var mcpAgent mcp.Agent
	var mcpMiddleware middlewares.MCPMiddleware
	if cfg.MCP.Enable {
		if cfg.MCP.Servers != "" {
			mcpClient = mcp.NewMCPClient(strings.Split(cfg.MCP.Servers, ","), logger, cfg)

			initCtx, cancel := context.WithTimeout(context.Background(), cfg.MCP.RequestTimeout)
			defer cancel()

			logger.Info("starting mcp client initialization", "timeout", cfg.MCP.RequestTimeout.String())
			initErr := mcpClient.InitializeAll(initCtx)
			if initErr != nil {
				logger.Error("failed to initialize mcp client", initErr)
				return
			}
			logger.Info("mcp client initialized successfully")
			// Create MCP agent with the initialized client
			mcpAgent = mcp.NewAgent(logger, mcpClient)
			logger.Info("mcp agent created successfully")
		} else {
			logger.Info("mcp is enabled but no servers configured, using no-op middleware")
			mcpAgent = mcp.NewAgent(logger, mcpClient)
		}
		mcpMiddleware, err = middlewares.NewMCPMiddleware(providerRegistry, client, mcpClient, mcpAgent, logger, cfg)
		if err != nil {
			logger.Error("failed to initialize mcp middleware", err)
			return
		}
	}

	// Initialize A2A client if enabled
	var a2aClient a2a.A2AClientInterface
	var a2aMiddleware middlewares.A2AMiddleware
	if cfg.A2A.Enable {
		if cfg.A2A.Agents != "" {
			a2aClient = a2a.NewA2AClient(cfg, logger)

			initCtx, cancel := context.WithTimeout(context.Background(), cfg.A2A.ClientTimeout)
			defer cancel()

			logger.Info("starting a2a client initialization", "timeout", cfg.A2A.ClientTimeout.String())
			initErr := a2aClient.InitializeAll(initCtx)
			if initErr != nil {
				logger.Error("failed to initialize a2a client", initErr)
				return
			}
			logger.Info("a2a client initialized successfully")

			a2aClient.StartStatusPolling(context.Background())
		} else {
			logger.Info("a2a is enabled but no agents configured")
		}

		a2aAgent := a2a.NewAgent(logger, a2aClient, cfg.A2A)
		a2aMiddleware, err = middlewares.NewA2AMiddleware(providerRegistry, a2aClient, a2aAgent, logger, client, cfg)
		if err != nil {
			logger.Error("failed to initialize a2a middleware", err)
			return
		}
	}

	// Set GIN mode based on environment
	if cfg.Environment != "development" {
		gin.SetMode(gin.ReleaseMode)
	}

	api := api.NewRouter(cfg, logger, providerRegistry, client, mcpClient, a2aClient)
	r := gin.New()
	r.Use(loggerMiddleware.Middleware())
	if cfg.Telemetry.Enable {
		r.Use(telemetry.Middleware())
	}
	r.Use(oidcAuthenticator.Middleware())

	// Add A2A middleware if enabled
	if cfg.A2A.Enable {
		r.Use(a2aMiddleware.Middleware())
		logger.Info("a2a middleware added to request pipeline")
	}

	// Add MCP middleware if enabled
	if cfg.MCP.Enable {
		r.Use(mcpMiddleware.Middleware())
		logger.Info("mcp middleware added to request pipeline")
	}

	r.GET("/health", api.HealthcheckHandler)
	r.Any("/proxy/:provider/*path", api.ProxyHandler)
	v1 := r.Group("/v1")
	{
		v1.GET("/models", api.ListModelsHandler)
		v1.GET("/a2a/agents", api.ListAgentsHandler)
		v1.GET("/a2a/agents/:id", api.GetAgentHandler)
		v1.GET("/a2a/agents/status", api.GetAllAgentStatusesHandler)
		v1.GET("/a2a/agents/:id/status", api.GetAgentStatusHandler)
		v1.GET("/mcp/tools", api.ListToolsHandler)
		v1.POST("/chat/completions", api.ChatCompletionsHandler)
	}
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
			logger.Info("starting inference gateway with tls", "port", cfg.Server.Port)

			if err := server.ListenAndServeTLS(cfg.Server.TlsCertPath, cfg.Server.TlsKeyPath); err != nil && err != http.ErrServerClosed {
				logger.Error("listen and serve tls error", err)
			}
		}()
	} else {
		go func() {
			logger.Info("starting inference gateway", "port", cfg.Server.Port)

			if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				logger.Error("listen and serve error", err)
			}
		}()
	}

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit
	logger.Info("shutting down server...")

	if cfg.A2A.Enable && a2aClient != nil {
		a2aClient.StopStatusPolling()
	}

	ctxShutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(ctxShutdown); err != nil {
		logger.Error("server shutdown error", err)
	} else {
		logger.Info("server gracefully stopped")
	}
}
