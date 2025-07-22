package tools

import (
	"context"
	"fmt"

	"github.com/inference-gateway/a2a/adk/server"
)

// NewDivideTool creates a new division tool that divides one number by another
func NewDivideTool() server.Tool {
	return server.NewBasicTool(
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
		divideHandler,
	)
}

// divideHandler handles the division operation
func divideHandler(ctx context.Context, args map[string]interface{}) (string, error) {
	a, _ := args["a"].(float64)
	b, _ := args["b"].(float64)
	if b == 0 {
		return `{"error": "Division by zero is not allowed"}`, fmt.Errorf("division by zero")
	}
	result := a / b
	return fmt.Sprintf(`{"result": %f, "operation": "division"}`, result), nil
}
