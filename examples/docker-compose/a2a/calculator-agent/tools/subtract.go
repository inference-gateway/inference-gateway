package tools

import (
	"context"
	"fmt"

	"github.com/inference-gateway/a2a/adk/server"
)

// NewSubtractTool creates a new subtraction tool that subtracts one number from another
func NewSubtractTool() server.Tool {
	return server.NewBasicTool(
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
		subtractHandler,
	)
}

// subtractHandler handles the subtraction operation
func subtractHandler(ctx context.Context, args map[string]interface{}) (string, error) {
	a, _ := args["a"].(float64)
	b, _ := args["b"].(float64)
	result := a - b
	return fmt.Sprintf(`{"result": %f, "operation": "subtraction"}`, result), nil
}
