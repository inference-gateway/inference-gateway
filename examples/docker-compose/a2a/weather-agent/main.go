package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/inference-gateway/a2a/adk/server"
	"github.com/inference-gateway/a2a/adk/server/config"
	"github.com/sethvargo/go-envconfig"
	"go.uber.org/zap"

	"weather-agent/tools"
)

type Config struct {
	A2A config.Config `env:",prefix=A2A_"`
}

var (
	Version          = "unknown"
	AgentName        = "unknown"
	AgentDescription = "unknown"
)

func main() {
	// Load configuration from environment first
	var cfg Config

	ctx := context.Background()
	if err := envconfig.Process(ctx, &cfg); err != nil {
		log.Fatal("failed to load config:", err)
	}

	// Initialize logger based on DEBUG environment variable
	var logger *zap.Logger
	var err error
	if cfg.A2A.Debug {
		logger, err = zap.NewDevelopment()
	} else {
		logger, err = zap.NewProduction()
	}
	if err != nil {
		log.Fatal("failed to initialize logger:", err)
	}
	defer logger.Sync()

	logger.Debug("loaded configuration", zap.Any("config", cfg))

	// Create toolbox with weather tool
	toolBox := server.NewDefaultToolBox()

	// Add weather tool from tools package
	weatherTool := tools.NewGetWeatherTool()
	toolBox.AddTool(weatherTool)

	// Create A2A server with agent
	var a2aServer server.A2AServer

	// Check if we have LLM configuration, otherwise create a tool-only agent
	if cfg.A2A.AgentConfig.APIKey != "" {
		// Create agent with LLM capabilities
		agent, err := server.NewAgentBuilder(logger).
			WithConfig(&cfg.A2A.AgentConfig).
			WithToolBox(toolBox).
			Build()
		if err != nil {
			log.Fatal("failed to create agent:", err)
		}

		a2aServer, err = server.NewA2AServerBuilder(cfg.A2A, logger).
			WithAgent(agent).
			WithAgentCardFromFile("./.well-known/agent.json", map[string]interface{}{
				"name":        AgentName,
				"version":     Version,
				"description": AgentDescription,
				"url":         cfg.A2A.AgentURL,
			}).
			Build()
		if err != nil {
			log.Fatal("failed to create A2A server:", err)
		}
	} else {
		// Create tool-only agent without LLM (mock mode)
		logger.Info("creating tool-only agent without LLM")
		agent, err := server.NewAgentBuilder(logger).
			WithToolBox(toolBox).
			Build()
		if err != nil {
			log.Fatal("failed to create agent:", err)
		}

		a2aServer, err = server.NewA2AServerBuilder(cfg.A2A, logger).
			WithAgent(agent).
			WithAgentCardFromFile("./.well-known/agent.json", map[string]interface{}{
				"name":        AgentName,
				"version":     Version,
				"description": AgentDescription,
				"url":         cfg.A2A.AgentURL,
			}).
			Build()
		if err != nil {
			log.Fatal("failed to create A2A server:", err)
		}
	}

	// Start server
	go func() {
		if err := a2aServer.Start(ctx); err != nil {
			log.Fatal("server failed to start:", err)
		}
	}()

	logger.Info("weather agent running", zap.String("port", cfg.A2A.ServerConfig.Port))

	// Wait for shutdown signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("shutting down server...")
	a2aServer.Stop(ctx)
}
