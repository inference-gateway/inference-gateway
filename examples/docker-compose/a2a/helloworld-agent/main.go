package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/inference-gateway/a2a/adk/server"
	"github.com/inference-gateway/a2a/adk/server/config"
	"github.com/sethvargo/go-envconfig"
	"go.uber.org/zap"
)

func main() {
	// Load configuration from environment first
	cfg := config.Config{
		AgentName:        "helloworld-agent",
		AgentDescription: "A simple greeting agent",
		Port:             "8080",
	}

	ctx := context.Background()
	if err := envconfig.Process(ctx, &cfg); err != nil {
		log.Fatal("failed to load config:", err)
	}

	// Initialize logger based on DEBUG environment variable
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
	defer logger.Sync()

	// Create toolbox with greeting tool
	toolBox := server.NewDefaultToolBox()

	// Add a greeting tool
	greetingTool := server.NewBasicTool(
		"greet_user",
		"Generate a personalized greeting",
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"name": map[string]interface{}{
					"type":        "string",
					"description": "The name of the person to greet",
				},
				"language": map[string]interface{}{
					"type":        "string",
					"description": "The language for the greeting (e.g., 'english', 'spanish', 'french')",
				},
			},
			"required": []string{"name"},
		},
		func(ctx context.Context, args map[string]interface{}) (string, error) {
			name := args["name"].(string)
			language := "english"
			if lang, ok := args["language"].(string); ok {
				language = lang
			}

			var greeting string
			switch language {
			case "spanish":
				greeting = fmt.Sprintf("¡Hola, %s! ¿Cómo estás?", name)
			case "french":
				greeting = fmt.Sprintf("Bonjour, %s! Comment allez-vous?", name)
			default:
				greeting = fmt.Sprintf("Hello, %s! Nice to meet you!", name)
			}

			return fmt.Sprintf(`{"greeting": "%s", "language": "%s"}`, greeting, language), nil
		},
	)
	toolBox.AddTool(greetingTool)

	// Create A2A server with agent
	var a2aServer server.A2AServer
	if cfg.AgentConfig.APIKey != "" {
		// With LLM agent
		agent, err := server.NewAgentBuilder(logger).
			WithConfig(&cfg.AgentConfig).
			WithToolBox(toolBox).
			WithSystemPrompt("You are a friendly greeting assistant. Use the greet_user tool to create personalized greetings for users.").
			Build()
		if err != nil {
			log.Fatal("failed to create agent:", err)
		}

		a2aServer = server.NewA2AServerBuilder(cfg, logger).
			WithAgent(agent).
			Build()
	} else {
		// Mock mode without LLM
		agent, err := server.NewAgentBuilder(logger).
			WithToolBox(toolBox).
			Build()
		if err != nil {
			log.Fatal("failed to create agent:", err)
		}

		a2aServer = server.NewA2AServerBuilder(cfg, logger).
			WithAgent(agent).
			Build()
	}

	// Start server
	go func() {
		if err := a2aServer.Start(ctx); err != nil {
			log.Fatal("server failed to start:", err)
		}
	}()

	logger.Info("helloworld agent running", zap.String("port", cfg.Port))

	// Wait for shutdown signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("shutting down server...")
	a2aServer.Stop(ctx)
}
