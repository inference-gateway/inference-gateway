package tools

import (
	"context"
	"fmt"

	"github.com/inference-gateway/adk/server"
)

// NewGreetUserTool creates a new greeting tool that generates personalized greetings
func NewGreetUserTool() server.Tool {
	return server.NewBasicTool(
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
		greetUserHandler,
	)
}

// greetUserHandler handles the greeting tool execution
func greetUserHandler(ctx context.Context, args map[string]interface{}) (string, error) {
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
}
