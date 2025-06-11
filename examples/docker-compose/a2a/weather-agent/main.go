package main

import (
	"context"
	"log"
	"net/http"
	"time"

	sdk "github.com/inference-gateway/sdk"
	envconfig "github.com/sethvargo/go-envconfig"
	zap "go.uber.org/zap"
)

var logger *zap.Logger
var cfg Config

func main() {
	ctx := context.Background()

	if err := envconfig.Process(ctx, &cfg); err != nil {
		log.Fatal("failed to process configuration:", err)
	}

	var err error
	if cfg.Debug {
		logger, err = zap.NewDevelopment()
	} else {
		logger, err = zap.NewProduction()
	}
	if err != nil {
		log.Fatal("failed to initialize logger:", err)
	}
	defer logger.Sync()

	client := sdk.NewClient(&sdk.ClientOptions{
		BaseURL: cfg.InferenceGatewayURL,
	})

	weatherService := NewMockWeatherService(logger)
	weatherToolHandler := NewWeatherToolHandler(weatherService, logger)

	agent := NewA2AAgent(cfg, logger, client, weatherToolHandler)

	oidcAuthenticator, err := NewOIDCAuthenticatorMiddleware(logger, cfg)
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
		zap.Duration("streaming_status_update_interval", cfg.StreamingStatusUpdateInterval))

	go agent.StartTaskProcessor(ctx)

	router := agent.SetupRouter(oidcAuthenticator)

	server := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      router,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
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
