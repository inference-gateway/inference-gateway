package main

import (
	"context"
	"log"
	"net/http"

	adk "github.com/inference-gateway/a2a/adk"
	sdk "github.com/inference-gateway/sdk"
	envconfig "github.com/sethvargo/go-envconfig"
	zap "go.uber.org/zap"
)

func main() {
	ctx := context.Background()

	var cfg adk.Config
	if err := envconfig.Process(ctx, &cfg); err != nil {
		log.Fatal("failed to process configuration:", err)
	}

	var logger *zap.Logger
	var err error
	if cfg.Debug {
		logger, err = zap.NewDevelopment()
	} else {
		logger, err = zap.NewProduction()
	}
	if err != nil {
		log.Fatal("failed to initialize logger:", err)
	}
	defer func() {
		if syncErr := logger.Sync(); syncErr != nil {
			log.Printf("failed to sync logger: %v", syncErr)
		}
	}()

	client := sdk.NewClient(&sdk.ClientOptions{
		BaseURL: cfg.InferenceGatewayURL,
	})

	weatherService := NewMockWeatherService(logger)
	weatherToolHandler := NewWeatherToolHandler(weatherService, logger)
	weatherToolProvider := NewWeatherToolProvider(weatherToolHandler)

	toolsHandler := adk.NewToolsHandler(logger, weatherToolProvider)

	agent := adk.NewA2AAgent(cfg, logger, client, toolsHandler)

	weatherTaskProcessor := NewWeatherTaskResultProcessor(logger)
	weatherInfoProvider := NewWeatherAgentInfoProvider(logger)

	agent.SetTaskResultProcessor(weatherTaskProcessor)
	agent.SetAgentInfoProvider(weatherInfoProvider)

	oidcAuthenticator, err := adk.NewOIDCAuthenticatorMiddleware(logger, cfg)
	if err != nil {
		logger.Fatal("failed to initialize oidc authenticator", zap.Error(err))
	}

	logger.Info("starting agent",
		zap.String("name", cfg.AgentName),
		zap.String("version", cfg.AgentVersion),
		zap.String("port", cfg.Port),
		zap.String("inference_gateway_url", cfg.InferenceGatewayURL),
		zap.String("llm_provider", cfg.LLMProvider),
		zap.String("llm_model", cfg.LLMModel),
		zap.Bool("debug_mode", cfg.Debug),
		zap.Bool("enable_auth", cfg.AuthConfig.Enable),
		zap.Bool("tls_enabled", cfg.TLSConfig.Enable),
		zap.Duration("cleanup_completed_task_interval", cfg.QueueConfig.CleanupInterval),
		zap.Int("max_queue_size", cfg.QueueConfig.MaxSize),
		zap.Duration("streaming_status_update_interval", cfg.StreamingStatusUpdateInterval),
		zap.Duration("server_read_timeout", cfg.ServerConfig.ReadTimeout),
		zap.Duration("server_write_timeout", cfg.ServerConfig.WriteTimeout),
		zap.Duration("server_idle_timeout", cfg.ServerConfig.IdleTimeout))

	go agent.StartTaskProcessor(ctx)

	router := agent.SetupRouter(oidcAuthenticator)

	server := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      router,
		ReadTimeout:  cfg.ServerConfig.ReadTimeout,
		WriteTimeout: cfg.ServerConfig.WriteTimeout,
		IdleTimeout:  cfg.ServerConfig.IdleTimeout,
	}

	if cfg.TLSConfig.Enable {
		logger.Info("agent starting with tls", zap.String("agent", cfg.AgentName), zap.String("port", cfg.Port))
		if err := server.ListenAndServeTLS(cfg.TLSConfig.CertPath, cfg.TLSConfig.KeyPath); err != nil {
			logger.Fatal("failed to start server with tls", zap.Error(err))
		}
	} else {
		logger.Info("agent starting", zap.String("agent", cfg.AgentName), zap.String("port", cfg.Port))
		if err := server.ListenAndServe(); err != nil {
			logger.Fatal("failed to start server", zap.Error(err))
		}
	}
}
