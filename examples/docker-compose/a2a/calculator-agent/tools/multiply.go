package tools

import (
	"context"
	"fmt"

	"github.com/inference-gateway/a2a/adk/server"
)

// NewMultiplyTool creates a new multiplication tool that multiplies two numbers together
func NewMultiplyTool() server.Tool {
	return server.NewBasicTool(
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
		multiplyHandler,
	)
}

// multiplyHandler handles the multiplication operation
func multiplyHandler(ctx context.Context, args map[string]interface{}) (string, error) {
	a, _ := args["a"].(float64)
	b, _ := args["b"].(float64)
	result := a * b
	return fmt.Sprintf(`{"result": %f, "operation": "multiplication"}`, result), nil
}
