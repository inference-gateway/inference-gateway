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

	mockMCPClient.EXPECT().GetToolsCatalog("", []string(nil)).Return([]mcp.ToolCatalogEntry{
		{Name: "mcp_server_a_read_file", Description: "Read a file", Server: "server_a"},
	}).Times(1)

	results, err := agent.ExecuteTools(ctx, []types.ChatCompletionMessageToolCall{
		toolCall("call_get", mcp.SelectorToolGet, ""),
	})
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Contains(t, toolResultContent(t, results[0]), "mcp_server_a_read_file")

	mockMCPClient.EXPECT().GetToolsCatalog("", []string{"mcp_server_a_read_file"}).Return([]mcp.ToolCatalogEntry{
		{Name: "mcp_server_a_read_file", Description: "Read a file", Server: "server_a", InputSchema: map[string]any{"type": "object"}},
	}).Times(1)

	results, err = agent.ExecuteTools(ctx, []types.ChatCompletionMessageToolCall{
		toolCall("call_schema", mcp.SelectorToolGet, `{"names":["mcp_server_a_read_file"]}`),
	})
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Contains(t, toolResultContent(t, results[0]), "input_schema")

	mockMCPClient.EXPECT().ResolveTool("mcp_server_a_read_file").Return("server_a", "read_file", nil).Times(1)
	mockMCPClient.EXPECT().ExecuteTool(
		gomock.Any(),
		mcp.Request{
			Method: "tools/call",
			Params: map[string]any{
				"name":      "read_file",
				"arguments": map[string]any{"path": "/tmp/x"},
			},
		},
		"server_a",
	).Return(&mcp.CallToolResult{
		Content: []mcp.ContentBlock{mcp.TextContent{Type: "text", Text: "file contents"}},
	}, nil).Times(1)

	results, err = agent.ExecuteTools(ctx, []types.ChatCompletionMessageToolCall{
		toolCall("call_exec", mcp.SelectorToolExecute, `{"name":"mcp_server_a_read_file","arguments":{"path":"/tmp/x"}}`),
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

// TestAgent_SameToolNameOnTwoServers verifies that a tool name exposed by two
// servers routes to the server named in the alias, not to whichever one a scan
// happened to find first.
func TestAgent_SameToolNameOnTwoServers(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockMCPClient := mcpmocks.NewMockMCPClientInterface(ctrl)
	agent := mcp.NewAgent(logger.NewNoopLogger(), mockMCPClient)

	for _, alias := range []string{"server_a", "server_b"} {
		mockMCPClient.EXPECT().ResolveTool("mcp_"+alias+"_read_file").Return(alias, "read_file", nil).Times(1)
		mockMCPClient.EXPECT().ExecuteTool(gomock.Any(), gomock.Any(), alias).
			Return(&mcp.CallToolResult{Content: []mcp.ContentBlock{mcp.TextContent{Type: "text", Text: alias + " contents"}}}, nil).Times(1)
	}

	results, err := agent.ExecuteTools(context.Background(), []types.ChatCompletionMessageToolCall{
		toolCall("call_a", "mcp_server_a_read_file", `{}`),
		toolCall("call_b", mcp.SelectorToolExecute, `{"name":"mcp_server_b_read_file","arguments":{}}`),
	})
	require.NoError(t, err)
	require.Len(t, results, 2)
	assert.Contains(t, toolResultContent(t, results[0]), "server_a contents")
	assert.Contains(t, toolResultContent(t, results[1]), "server_b contents")

	_, err = json.Marshal(mcp.ToolCatalogEntry{Name: "x"})
	require.NoError(t, err)
}
