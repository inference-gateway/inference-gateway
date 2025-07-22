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

type Config struct {
	A2A config.Config `prefix:"A2A_"`
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

	// Create toolbox with weather tool
	toolBox := server.NewDefaultToolBox()

	// Add a weather tool
	weatherTool := server.NewBasicTool(
		"get_weather",
		"Get current weather information for a location",
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"location": map[string]interface{}{
					"type":        "string",
					"description": "The city and state, e.g. San Francisco, CA",
				},
				"units": map[string]interface{}{
					"type":        "string",
					"description": "Temperature units: celsius, fahrenheit, or kelvin",
					"enum":        []string{"celsius", "fahrenheit", "kelvin"},
				},
			},
			"required": []string{"location"},
		},
		func(ctx context.Context, args map[string]interface{}) (string, error) {
			location := args["location"].(string)
			units := "celsius"
			if u, ok := args["units"].(string); ok {
				units = u
			}

			// Mock weather data based on location
			var temp string
			var description string
			switch location {
			case "San Francisco, CA":
				switch units {
				case "fahrenheit":
					temp = "65°F"
				case "kelvin":
					temp = "291K"
				default:
					temp = "18°C"
				}
				description = "Partly cloudy with light fog"
			case "New York, NY":
				switch units {
				case "fahrenheit":
					temp = "72°F"
				case "kelvin":
					temp = "295K"
				default:
					temp = "22°C"
				}
				description = "Sunny with scattered clouds"
			default:
				switch units {
				case "fahrenheit":
					temp = "70°F"
				case "kelvin":
					temp = "294K"
				default:
					temp = "21°C"
				}
				description = "Moderate weather conditions"
			}

			result := fmt.Sprintf(`{
				"location": "%s",
				"temperature": "%s",
				"description": "%s",
				"units": "%s",
				"humidity": "60%%",
				"wind_speed": "15 km/h"
			}`, location, temp, description, units)

			return result, nil
		},
	)
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
