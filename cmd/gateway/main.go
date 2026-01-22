package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	gin "github.com/gin-gonic/gin"
	api "github.com/inference-gateway/inference-gateway/api"
	middlewares "github.com/inference-gateway/inference-gateway/api/middlewares"
	config "github.com/inference-gateway/inference-gateway/config"
	l "github.com/inference-gateway/inference-gateway/logger"
	mcp "github.com/inference-gateway/inference-gateway/mcp"
	otel "github.com/inference-gateway/inference-gateway/otel"
	providers "github.com/inference-gateway/inference-gateway/providers"
	promhttp "github.com/prometheus/client_golang/prometheus/promhttp"
	envconfig "github.com/sethvargo/go-envconfig"
)

var (
	version = "dev"
)

func main() {
	versionFlag := flag.Bool("version", false, "Print version information")
	helpFlag := flag.Bool("help", false, "Print help information")
	flag.Parse()

	if *versionFlag {
		fmt.Println(version)
		os.Exit(0)
	}

	if *helpFlag {
		fmt.Println("Inference Gateway - Unified API gateway for multiple LLM providers")
		fmt.Println()
		fmt.Println("Usage:")
		fmt.Println("  inference-gateway [flags]")
		fmt.Println()
		fmt.Println("Flags:")
		fmt.Println("  --version    Print version information")
		fmt.Println("  --help       Print help information")
		fmt.Println()
		fmt.Println("Configuration:")
		fmt.Println("  The gateway is configured via environment variables.")
		fmt.Println("  See https://github.com/inference-gateway/inference-gateway/blob/main/Configurations.md")
		fmt.Println()
		fmt.Println("Examples:")
		fmt.Println("  # Start the gateway with default configuration")
		fmt.Println("  inference-gateway")
		fmt.Println()
		fmt.Println("  # Start with specific provider configured")
		fmt.Println("  export OPENAI_API_KEY=your-key")
		fmt.Println("  inference-gateway")
		os.Exit(0)
	}
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

	// Log registered providers
	var providerNames []string
	for providerID := range cfg.Providers {
		providerNames = append(providerNames, string(providerID))
	}
	logger.Info("provider registry initialized", "count", len(providerNames), "providers", strings.Join(providerNames, ", "))

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

			mcpClient.StartStatusPolling(context.Background())
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

	// Set GIN mode based on environment
	if cfg.Environment != "development" {
		gin.SetMode(gin.ReleaseMode)
	}

	api := api.NewRouter(cfg, logger, providerRegistry, client, mcpClient)
	r := gin.New()
	r.Use(loggerMiddleware.Middleware())
	if cfg.Telemetry.Enable {
		r.Use(telemetry.Middleware())
	}
	r.Use(oidcAuthenticator.Middleware())

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

	// Validate provider connectivity after server starts
	go func() {
		// Wait a moment for the server to be ready
		time.Sleep(2 * time.Second)

		totalModels := 0
		availableProviders := 0

		for providerID := range cfg.Providers {
			provider, err := providerRegistry.BuildProvider(providerID, client)
			if err != nil {
				logger.Warn("failed to build provider", "provider", providerID, "error", err.Error())
				continue
			}

			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			response, err := provider.ListModels(ctx)
			cancel()

			if err != nil {
				logger.Warn("provider unavailable or authentication failed", "provider", providerID, "error", err.Error())
			} else {
				modelCount := len(response.Data)
				totalModels += modelCount
				availableProviders++
				logger.Info("provider ready", "provider", providerID, "models", modelCount)
			}
		}

		logger.Info("provider validation complete", "total_providers", len(cfg.Providers), "available_providers", availableProviders, "total_models", totalModels)
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit
	logger.Info("shutting down server...")

	if cfg.MCP.Enable && mcpClient != nil {
		mcpClient.StopStatusPolling()
	}

	ctxShutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(ctxShutdown); err != nil {
		logger.Error("server shutdown error", err)
	} else {
		logger.Info("server gracefully stopped")
	}
}
