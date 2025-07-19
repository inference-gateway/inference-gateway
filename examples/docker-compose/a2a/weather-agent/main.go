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
		AgentName:        "weather-agent",
		AgentDescription: "A weather information agent",
		Port:             "8080",
		AgentConfig: config.AgentConfig{
			Provider: "deepseek",
			Model:    "deepseek-chat",
		},
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
	if cfg.AgentConfig.APIKey != "" {
		// Create agent with LLM capabilities
		agent, err := server.NewAgentBuilder(logger).
			WithConfig(&cfg.AgentConfig).
			WithToolBox(toolBox).
			Build()
		if err != nil {
			log.Fatal("failed to create agent:", err)
		}

		a2aServer = server.NewA2AServerBuilder(cfg, logger).
			WithAgent(agent).
			Build()
	} else {
		// Create tool-only agent without LLM (mock mode)
		logger.Info("creating tool-only agent without LLM")
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

	logger.Info("weather agent running", zap.String("port", cfg.Port))

	// Wait for shutdown signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("shutting down server...")
	a2aServer.Stop(ctx)
}
