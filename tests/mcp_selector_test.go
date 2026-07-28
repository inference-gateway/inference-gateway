package tests

import (
	"context"
	"encoding/json"
	"testing"

	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
	gomock "go.uber.org/mock/gomock"

	mcpmocks "github.com/inference-gateway/inference-gateway/tests/mocks/mcp"

	mcp "github.com/inference-gateway/inference-gateway/internal/mcp"
	logger "github.com/inference-gateway/inference-gateway/logger"
	types "github.com/inference-gateway/inference-gateway/providers/types"
)

func toolCall(id, name, args string) types.ChatCompletionMessageToolCall {
	return types.ChatCompletionMessageToolCall{
		ID:   id,
		Type: types.Function,
		Function: types.ChatCompletionMessageToolCallFunction{
			Name:      name,
			Arguments: args,
		},
	}
}

func toolResultContent(t *testing.T, msg types.Message) string {
	t.Helper()
	content, err := msg.Content.AsMessageContent0()
	require.NoError(t, err)
	return content
}

// TestAgent_Selector_RoundTrip exercises discover -> get schema -> execute.
func TestAgent_Selector_RoundTrip(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockMCPClient := mcpmocks.NewMockMCPClientInterface(ctrl)
	agent := mcp.NewAgent(logger.NewNoopLogger(), mockMCPClient)
	ctx := context.Background()

	// 1. Discover: mcp_tools_get with no arguments -> compact catalog.
	mockMCPClient.EXPECT().GetToolsCatalog("", []string(nil)).Return([]mcp.ToolCatalogEntry{
		{Name: "read_file", Description: "Read a file", Server: "http://server-a"},
	}).Times(1)

	results, err := agent.ExecuteTools(ctx, []types.ChatCompletionMessageToolCall{
		toolCall("call_get", mcp.SelectorToolGet, ""),
	})
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Contains(t, toolResultContent(t, results[0]), "read_file")

	// 2. Get schema for a specific tool.
	mockMCPClient.EXPECT().GetToolsCatalog("", []string{"read_file"}).Return([]mcp.ToolCatalogEntry{
		{Name: "read_file", Description: "Read a file", Server: "http://server-a", InputSchema: map[string]any{"type": "object"}},
	}).Times(1)

	results, err = agent.ExecuteTools(ctx, []types.ChatCompletionMessageToolCall{
		toolCall("call_schema", mcp.SelectorToolGet, `{"names":["read_file"]}`),
	})
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Contains(t, toolResultContent(t, results[0]), "input_schema")

	// 3. Execute: mcp_tools_execute must dispatch the underlying tool.
	mockMCPClient.EXPECT().GetServerForTool("read_file").Return("http://server-a", nil).Times(1)
	mockMCPClient.EXPECT().ExecuteTool(
		gomock.Any(),
		mcp.Request{
			Method: "tools/call",
			Params: map[string]any{
				"name":      "read_file",
				"arguments": map[string]any{"path": "/tmp/x"},
			},
		},
		"http://server-a",
	).Return(&mcp.CallToolResult{
		Content: []mcp.ContentBlock{mcp.TextContent{Type: "text", Text: "file contents"}},
	}, nil).Times(1)

	results, err = agent.ExecuteTools(ctx, []types.ChatCompletionMessageToolCall{
		toolCall("call_exec", mcp.SelectorToolExecute, `{"name":"read_file","arguments":{"path":"/tmp/x"}}`),
	})
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, "call_exec", *results[0].ToolCallID)
	assert.Contains(t, toolResultContent(t, results[0]), "file contents")
}

// TestAgent_Selector_ExecuteMissingName rejects an execute call without a name.
func TestAgent_Selector_ExecuteMissingName(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockMCPClient := mcpmocks.NewMockMCPClientInterface(ctrl)
	agent := mcp.NewAgent(logger.NewNoopLogger(), mockMCPClient)

	results, err := agent.ExecuteTools(context.Background(), []types.ChatCompletionMessageToolCall{
		toolCall("call_bad", mcp.SelectorToolExecute, `{"arguments":{"path":"/tmp/x"}}`),
	})
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Contains(t, toolResultContent(t, results[0]), "requires a 'name'")
}

// TestAgent_Selector_ExecuteStripsPrefix ensures a wrapped mcp_ prefix is stripped
// before dispatch so guardrails and server lookup see the raw tool name.
func TestAgent_Selector_ExecuteStripsPrefix(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockMCPClient := mcpmocks.NewMockMCPClientInterface(ctrl)
	agent := mcp.NewAgent(logger.NewNoopLogger(), mockMCPClient)

	mockMCPClient.EXPECT().GetServerForTool("read_file").Return("http://server-a", nil).Times(1)
	mockMCPClient.EXPECT().ExecuteTool(gomock.Any(), gomock.Any(), "http://server-a").
		Return(&mcp.CallToolResult{Content: []mcp.ContentBlock{mcp.TextContent{Type: "text", Text: "ok"}}}, nil).Times(1)

	results, err := agent.ExecuteTools(context.Background(), []types.ChatCompletionMessageToolCall{
		toolCall("call_exec", mcp.SelectorToolExecute, `{"name":"mcp_read_file","arguments":{}}`),
	})
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Contains(t, toolResultContent(t, results[0]), "ok")

	// Verify JSON is produced from a real catalog entry so the marshaled shape holds.
	_, err = json.Marshal(mcp.ToolCatalogEntry{Name: "x"})
	require.NoError(t, err)
}
