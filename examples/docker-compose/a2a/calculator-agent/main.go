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
		AgentName:        "calculator-agent",
		AgentDescription: "A mathematical calculation agent",
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

	// Create toolbox with calculator tools
	toolBox := server.NewDefaultToolBox()

	// Add calculation tools
	addTool := server.NewBasicTool(
		"add",
		"Add two numbers together",
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"a": map[string]interface{}{
					"type":        "number",
					"description": "First number to add",
				},
				"b": map[string]interface{}{
					"type":        "number",
					"description": "Second number to add",
				},
			},
			"required": []string{"a", "b"},
		},
		func(ctx context.Context, args map[string]interface{}) (string, error) {
			a, _ := args["a"].(float64)
			b, _ := args["b"].(float64)
			result := a + b
			return fmt.Sprintf(`{"result": %f, "operation": "addition"}`, result), nil
		},
	)
	toolBox.AddTool(addTool)

	subtractTool := server.NewBasicTool(
		"subtract",
		"Subtract one number from another",
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"a": map[string]interface{}{
					"type":        "number",
					"description": "Number to subtract from",
				},
				"b": map[string]interface{}{
					"type":        "number",
					"description": "Number to subtract",
				},
			},
			"required": []string{"a", "b"},
		},
		func(ctx context.Context, args map[string]interface{}) (string, error) {
			a, _ := args["a"].(float64)
			b, _ := args["b"].(float64)
			result := a - b
			return fmt.Sprintf(`{"result": %f, "operation": "subtraction"}`, result), nil
		},
	)
	toolBox.AddTool(subtractTool)

	multiplyTool := server.NewBasicTool(
		"multiply",
		"Multiply two numbers together",
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"a": map[string]interface{}{
					"type":        "number",
					"description": "First number to multiply",
				},
				"b": map[string]interface{}{
					"type":        "number",
					"description": "Second number to multiply",
				},
			},
			"required": []string{"a", "b"},
		},
		func(ctx context.Context, args map[string]interface{}) (string, error) {
			a, _ := args["a"].(float64)
			b, _ := args["b"].(float64)
			result := a * b
			return fmt.Sprintf(`{"result": %f, "operation": "multiplication"}`, result), nil
		},
	)
	toolBox.AddTool(multiplyTool)

	divideTool := server.NewBasicTool(
		"divide",
		"Divide one number by another",
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"a": map[string]interface{}{
					"type":        "number",
					"description": "Number to divide (dividend)",
				},
				"b": map[string]interface{}{
					"type":        "number",
					"description": "Number to divide by (divisor)",
				},
			},
			"required": []string{"a", "b"},
		},
		func(ctx context.Context, args map[string]interface{}) (string, error) {
			a, _ := args["a"].(float64)
			b, _ := args["b"].(float64)
			if b == 0 {
				return `{"error": "Division by zero is not allowed"}`, fmt.Errorf("division by zero")
			}
			result := a / b
			return fmt.Sprintf(`{"result": %f, "operation": "division"}`, result), nil
		},
	)
	toolBox.AddTool(divideTool)

	// Create A2A server with agent
	var a2aServer server.A2AServer
	if cfg.AgentConfig.APIKey != "" {
		// With LLM agent
		agent, err := server.NewAgentBuilder(logger).
			WithConfig(&cfg.AgentConfig).
			WithToolBox(toolBox).
			WithSystemPrompt("You are a mathematical calculation assistant. Use the available math tools (add, subtract, multiply, divide) to help users perform calculations. Always show your work and explain the results.").
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

	logger.Info("calculator agent running", zap.String("port", cfg.Port))

	// Wait for shutdown signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("shutting down server...")
	a2aServer.Stop(ctx)
}
