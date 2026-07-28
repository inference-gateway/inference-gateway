package tests

import (
	"encoding/json"
	"fmt"
	"testing"

	config "github.com/inference-gateway/inference-gateway/config"
	mcp "github.com/inference-gateway/inference-gateway/internal/mcp"
	logger "github.com/inference-gateway/inference-gateway/logger"
	types "github.com/inference-gateway/inference-gateway/providers/types"
)

// buildMCPTools fabricates n MCP tools with a representative input schema so the
// benchmark reflects real per-tool schema weight.
func buildMCPTools(n int) []mcp.Tool {
	tools := make([]mcp.Tool, 0, n)
	for i := 0; i < n; i++ {
		desc := fmt.Sprintf("Tool number %d that performs a useful operation on the given inputs", i)
		tools = append(tools, mcp.Tool{
			Name:        fmt.Sprintf("tool_%d", i),
			Description: &desc,
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"path":      map[string]any{"type": "string", "description": "The filesystem path to operate on"},
					"recursive": map[string]any{"type": "boolean", "description": "Whether to recurse into subdirectories"},
					"limit":     map[string]any{"type": "integer", "description": "Maximum number of results to return"},
				},
				"required": []any{"path"},
			},
		})
	}
	return tools
}

func requestBytes(tools []types.ChatCompletionTool) int {
	req := types.CreateChatCompletionRequest{
		Model: "openai/gpt-4o",
		Messages: []types.Message{
			{Role: types.User},
		},
		Tools: &tools,
	}
	b, _ := json.Marshal(req)
	return len(b)
}

// BenchmarkMCPToolInjection compares the serialized request size of direct
// injection (all tool schemas) against selector mode (two meta-tools). It
// reports bytes/op per mode plus the reduction factor for a 50-tool setup.
func BenchmarkMCPToolInjection(b *testing.B) {
	client := mcp.NewMCPClient(nil, logger.NewNoopLogger(), config.Config{MCP: &config.MCPConfig{}})
	directTools := client.ConvertMCPToolsToChatCompletionTools(buildMCPTools(50))
	selectorTools := mcp.SelectorToolDefinitions()

	directSize := requestBytes(directTools)
	selectorSize := requestBytes(selectorTools)

	if selectorSize >= directSize {
		b.Fatalf("selector mode (%d bytes) should be smaller than direct mode (%d bytes)", selectorSize, directSize)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = requestBytes(selectorTools)
	}
	b.StopTimer()

	b.ReportMetric(float64(directSize), "direct_bytes")
	b.ReportMetric(float64(selectorSize), "selector_bytes")
	b.ReportMetric(float64(directSize)/float64(selectorSize), "reduction_x")
}
