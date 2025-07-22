package tools

import (
	"context"
	"fmt"

	"github.com/inference-gateway/a2a/adk/server"
)

// NewAddTool creates a new addition tool that adds two numbers together
func NewAddTool() server.Tool {
	return server.NewBasicTool(
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
		addHandler,
	)
}

// addHandler handles the addition operation
func addHandler(ctx context.Context, args map[string]interface{}) (string, error) {
	a, _ := args["a"].(float64)
	b, _ := args["b"].(float64)
	result := a + b
	return fmt.Sprintf(`{"result": %f, "operation": "addition"}`, result), nil
}
