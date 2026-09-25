package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	gin "github.com/gin-gonic/gin"
	promhttp "github.com/prometheus/client_golang/prometheus/promhttp"
	envconfig "github.com/sethvargo/go-envconfig"
	otelgin "go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"

	api "github.com/inference-gateway/inference-gateway/api"
	middlewares "github.com/inference-gateway/inference-gateway/api/middlewares"
	config "github.com/inference-gateway/inference-gateway/config"
	guardrails "github.com/inference-gateway/inference-gateway/internal/guardrails"
	mcp "github.com/inference-gateway/inference-gateway/internal/mcp"
	logger "github.com/inference-gateway/inference-gateway/internal/platform/logger"
	otel "github.com/inference-gateway/inference-gateway/internal/platform/otel"
	tts "github.com/inference-gateway/inference-gateway/internal/tts"
	client "github.com/inference-gateway/inference-gateway/providers/client"
	constants "github.com/inference-gateway/inference-gateway/providers/constants"
	registry "github.com/inference-gateway/inference-gateway/providers/registry"
	routing "github.com/inference-gateway/inference-gateway/providers/routing"
)

var (
	version = "dev"
)

const (
	metricsServerReadTimeout  = 10 * time.Second
	metricsServerWriteTimeout = 10 * time.Second
	metricsServerIdleTimeout  = 30 * time.Second
	shutdownGracePeriod       = 5 * time.Second
	providerProbeStartDelay   = 2 * time.Second
	providerProbeTimeout      = 10 * time.Second
)

func isLoopbackHost(host string) bool {
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

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
	var appLogger logger.Logger
	appLogger, err = logger.NewLogger(cfg.Environment)
	if err != nil {
		log.Printf("{\"error\": \"logger init error: %v\"}", err)
		return
	}

	// Log config in debug mode
	appLogger.Debug("loaded config", "config", cfg.String())

	if !cfg.Auth.Enabled && !isLoopbackHost(cfg.Server.Host) {
		appLogger.Warn("gateway bound to a non-loopback address with authentication disabled; "+
			"any client that can reach this port can consume your configured provider API keys - "+
			"set AUTH_ENABLED=true or bind SERVER_HOST to loopback",
			"host", cfg.Server.Host)
	}

	// Initialize OpenTelemetry Prometheus exporter Server
	var telemetryImpl otel.OpenTelemetry
	if cfg.Telemetry.Enabled {
		telemetryImpl = &otel.OpenTelemetryImpl{}
		err := telemetryImpl.Init(cfg, appLogger)
		if err != nil {
			appLogger.Error("opentelemetry initialization failed", err)
			return
		}

		metricsMux := http.NewServeMux()
		metricsMux.Handle("/metrics", promhttp.Handler())

		appLogger.Info("telemetry initialized successfully")

		metricsServer := &http.Server{
			Addr:         ":" + cfg.Telemetry.MetricsPort,
			Handler:      metricsMux,
			ReadTimeout:  metricsServerReadTimeout,
			WriteTimeout: metricsServerWriteTimeout,
			IdleTimeout:  metricsServerIdleTimeout,
		}

		go func() {
			appLogger.Info("starting metrics server", "port", cfg.Telemetry.MetricsPort)
			if err := metricsServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				appLogger.Error("metrics server failed", err)
			}
		}()

		defer func() {
			appLogger.Info("shutting down metrics server...")
			ctxMetrics, cancelMetrics := context.WithTimeout(context.Background(), shutdownGracePeriod)
			defer cancelMetrics()

			if err := metricsServer.Shutdown(ctxMetrics); err != nil {
				appLogger.Error("metrics server shutdown error", err)
			} else {
				appLogger.Info("metrics server gracefully stopped")
			}
		}()

		defer func() {
			ctx, cancel := context.WithTimeout(context.Background(), shutdownGracePeriod)
			defer cancel()
			if err := telemetryImpl.ShutDown(ctx); err != nil {
				appLogger.Error("error shutting down telemetry", err)
			}
		}()
	}

	// Initialize logger middleware
	loggerMiddleware := middlewares.NewLoggerMiddleware(&appLogger)

	// Initialize telemetry middleware
	var telemetry *middlewares.TelemetryMiddleware
	if cfg.Telemetry.Enabled {
		telemetry = middlewares.NewTelemetryMiddleware(cfg, telemetryImpl, appLogger)
	}

	// Initialize OIDC authenticator middleware
	oidcAuthenticator, err := middlewares.NewOIDCAuthenticatorMiddleware(appLogger, cfg)
	if err != nil {
		appLogger.Error("failed to initialize oidc authenticator", err)
		return
	}

	scheme := "http"
	if cfg.Server.TlsCertPath != "" && cfg.Server.TlsKeyPath != "" {
		scheme = "https"
	}

	httpClient := client.NewHTTPClient(cfg.Client, scheme, cfg.Server.Host, cfg.Server.Port)
	providerRegistry := registry.NewProviderRegistry(cfg.Providers, appLogger)

	// Log registered providers
	var providerNames []string
	for providerID := range cfg.Providers {
		providerNames = append(providerNames, string(providerID))
	}
	appLogger.Info("provider registry initialized", "count", len(providerNames), "providers", strings.Join(providerNames, ", "))

	// Initialize MCP middleware if enabled
	var mcpClient mcp.MCPClientInterface
	var mcpAgent *mcp.Agent
	var mcpMiddleware middlewares.MCPMiddleware
	if cfg.MCP.Enabled {
		if cfg.MCP.Servers != "" {
			mcpServers, err := mcp.ParseServers(cfg.MCP.Servers)
			if err != nil {
				appLogger.Error("invalid MCP_SERVERS configuration", err)
				return
			}
			mcpClient = mcp.NewMCPClient(mcpServers, appLogger, cfg)

			initCtx, cancel := context.WithTimeout(context.Background(), cfg.MCP.RequestTimeout)
			defer cancel()

			appLogger.Info("starting mcp client initialization", "timeout", cfg.MCP.RequestTimeout.String())
			initErr := mcpClient.InitializeAll(initCtx)
			switch {
			case initErr == nil:
				appLogger.Info("mcp client initialized successfully")
			case errors.Is(initErr, mcp.ErrNoClientsInitialized) && cfg.MCP.EnableReconnect:
				appLogger.Warn("no mcp servers initialized at startup; continuing with background reconnection enabled",
					"error", initErr.Error())
			default:
				appLogger.Error("failed to initialize mcp client", initErr)
				return
			}

			mcpClient.StartStatusPolling(context.Background())
			mcpAgent = mcp.NewAgent(appLogger, mcpClient)
			appLogger.Info("mcp agent created successfully")
		} else {
			appLogger.Info("mcp is enabled but no servers configured, using no-op middleware")
			mcpAgent = mcp.NewAgent(appLogger, mcpClient)
		}
		mcpAgent.SetTelemetry(telemetryImpl)
		mcpMiddleware, err = middlewares.NewMCPMiddleware(providerRegistry, httpClient, mcpClient, mcpAgent, appLogger, cfg)
		if err != nil {
			appLogger.Error("failed to initialize mcp middleware", err)
			return
		}
	}

	// Initialize guardrails middleware if enabled
	var guardrailsMiddleware middlewares.GuardrailsMiddleware
	if cfg.Guardrails != nil && cfg.Guardrails.Enabled {
		ctx := context.Background()
		evaluator, err := guardrails.NewEvaluator(ctx, cfg.Guardrails.PolicyDir)
		if err != nil {
			appLogger.Error("failed to initialize guardrails evaluator", err)
			return
		}

		var externalClient *guardrails.ExternalClient
		if cfg.Guardrails.ExternalUrl != "" {
			externalClient = guardrails.NewExternalClient(cfg.Guardrails.ExternalUrl, cfg.Guardrails.ExternalTimeout)
		}

		detectors := guardrails.DefaultDetectors()
		guardrailsMiddleware = middlewares.NewGuardrailsMiddleware(
			evaluator,
			externalClient,
			detectors,
			appLogger,
			telemetryImpl,
			cfg,
		)
		appLogger.Info("guardrails middleware initialized", "policy_dir", cfg.Guardrails.PolicyDir)

		if mcpAgent != nil {
			mcpAgent.SetGuardrails(evaluator, cfg.Guardrails.FailMode)
		}
	} else {
		guardrailsMiddleware = middlewares.NewGuardrailsMiddleware(nil, nil, nil, appLogger, telemetryImpl, cfg)
	}

	// Build the model routing selector if enabled (opt-in, default off).
	var selector *routing.Selector
	if cfg.Routing != nil && cfg.Routing.Enabled {
		poolsCfg, err := routing.LoadPoolsConfig(cfg.Routing.ConfigPath)
		if err != nil {
			appLogger.Error("failed to load routing config", err, "path", cfg.Routing.ConfigPath)
			return
		}
		selector, err = routing.NewSelector(poolsCfg)
		if err != nil {
			appLogger.Error("invalid routing config", err, "path", cfg.Routing.ConfigPath)
			return
		}
		appLogger.Info("model routing enabled", "aliases", selector.Aliases())
	}

	// Set GIN mode based on environment
	if cfg.Environment != constants.EnvironmentDevelopment {
		gin.SetMode(gin.ReleaseMode)
	}

	var localTTS *tts.Engine
	if cfg.AudioEnabled {
		home, homeErr := os.UserHomeDir()
		if homeErr != nil {
			appLogger.Warn("audio: cannot resolve home dir; local speech cache paths resolve relative to the working directory (.infer/bin, .infer/models/tts)", "error", homeErr.Error())
		}
		localTTS = tts.NewEngine(appLogger, tts.Config{
			AutoDownload:   cfg.AudioLocalAutoDownload,
			MaxConcurrency: cfg.AudioLocalMaxConcurrency,
			Timeout:        time.Duration(cfg.AudioLocalTimeout) * time.Second,
			Home:           home,
		})
		go localTTS.Warmup(context.Background())
	}

	mcp.GatewayInfo.Version = version
	api := api.NewRouter(cfg, appLogger, providerRegistry, httpClient, mcpClient, mcpAgent, telemetryImpl, selector, localTTS)
	r := gin.New()
	if cfg.Telemetry.Enabled && cfg.Telemetry.TracingEnabled {
		r.Use(otelgin.Middleware("inference-gateway", otelgin.WithFilter(func(req *http.Request) bool {
			return req.URL.Path != middlewares.HealthPath && req.URL.Path != middlewares.MetricsIngestPath
		})))
		appLogger.Info("tracing middleware added to request pipeline")
	}
	r.Use(loggerMiddleware.Middleware())
	if cfg.Telemetry.Enabled {
		r.Use(telemetry.Middleware())
	}
	r.Use(oidcAuthenticator.Middleware())

	// Add guardrails middleware (before MCP so it wraps MCP's writer for post_call).
	r.Use(guardrailsMiddleware.Middleware())
	appLogger.Info("guardrails middleware added to request pipeline")

	// Add MCP middleware if enabled
	if cfg.MCP.Enabled {
		r.Use(mcpMiddleware.Middleware())
		appLogger.Info("mcp middleware added to request pipeline")
	}

	r.GET(middlewares.HealthPath, api.HealthcheckHandler)
	r.GET(middlewares.MCPProtectedResourcePath, api.MCPProtectedResourceMetadataHandler)
	r.POST(middlewares.MCPPath, api.MCPJSONRPCHandler)
	r.Match([]string{http.MethodGet, http.MethodDelete}, middlewares.MCPPath, func(c *gin.Context) {
		c.Header("Allow", http.MethodPost)
		c.Status(http.StatusMethodNotAllowed)
	})
	r.POST(middlewares.MetricsIngestPath, api.MetricsIngestionHandler)
	r.POST(middlewares.ChatCompletionsPath, api.ChatCompletionsHandler)
	r.POST(middlewares.ResponsesPath, api.ResponsesHandler)
	r.Any("/proxy/:provider/*path", api.ProxyHandler)
	v1 := r.Group("/v1")
	{
		v1.GET("/models", api.ListModelsHandler)
		v1.POST("/messages", api.MessagesHandler)
		v1.POST("/images/generations", api.ImagesHandler)
		v1.POST("/images/edits", api.ImagesEditsHandler)
		v1.POST("/audio/speech", api.SpeechHandler)
		v1.POST("/audio/sfx", api.SFXHandler)
		v1.POST("/audio/music", api.MusicHandler)
		v1.POST("/videos", api.VideosHandler)
		v1.GET("/videos/:video_id", api.RetrieveVideoHandler)
		v1.GET("/videos/:video_id/content", api.DownloadVideoContentHandler)
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
			appLogger.Info("starting inference gateway with tls", "port", cfg.Server.Port)

			if err := server.ListenAndServeTLS(cfg.Server.TlsCertPath, cfg.Server.TlsKeyPath); err != nil && err != http.ErrServerClosed {
				appLogger.Error("listen and serve tls error", err)
			}
		}()
	} else {
		go func() {
			appLogger.Info("starting inference gateway", "port", cfg.Server.Port)

			if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				appLogger.Error("listen and serve error", err)
			}
		}()
	}

	go func() {
		time.Sleep(providerProbeStartDelay)

		totalModels := 0
		availableProviders := 0

		for providerID := range cfg.Providers {
			provider, err := providerRegistry.BuildProvider(providerID, httpClient)
			if err != nil {
				appLogger.Warn("failed to build provider", "provider", providerID, "error", err.Error())
				continue
			}

			ctx, cancel := context.WithTimeout(context.Background(), providerProbeTimeout)
			response, err := provider.ListModels(ctx)
			cancel()

			if err != nil {
				appLogger.Warn("provider unavailable or authentication failed", "provider", providerID, "error", err.Error())
			} else {
				modelCount := len(response.Data)
				totalModels += modelCount
				availableProviders++
				appLogger.Info("provider ready", "provider", providerID, "models", modelCount)
			}
		}

		appLogger.Info("provider validation complete", "total_providers", len(cfg.Providers), "available_providers", availableProviders, "total_models", totalModels)
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit
	appLogger.Info("shutting down server...")

	if cfg.MCP.Enabled && mcpClient != nil {
		mcpClient.StopStatusPolling()
		mcpClient.StopBackgroundReconnection()
	}

	ctxShutdown, cancel := context.WithTimeout(context.Background(), shutdownGracePeriod)
	defer cancel()

	if err := server.Shutdown(ctxShutdown); err != nil {
		appLogger.Error("server shutdown error", err)
	} else {
		appLogger.Info("server gracefully stopped")
	}
}
