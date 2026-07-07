package types_test

import (
	"testing"

	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"

	types "github.com/inference-gateway/inference-gateway/providers/types"
)

func TestAccumulateStreamingToolCalls(t *testing.T) {
	tests := []struct {
		name     string
		body     string
		expected []types.ChatCompletionMessageToolCall
	}{
		{
			name: "single tool call assembled across chunks",
			body: `data: {"choices":[{"delta":{"tool_calls":[{"index":0,"id":"call_123","type":"function","function":{"name":"mcp_test_tool"}}]}}]}
data: {"choices":[{"delta":{"tool_calls":[{"index":0,"function":{"arguments":"{\"arg1\""}}]}}]}
data: {"choices":[{"delta":{"tool_calls":[{"index":0,"function":{"arguments":":\"value1\",\"arg2\":42}"}}]}}]}
data: [DONE]`,
			expected: []types.ChatCompletionMessageToolCall{
				{
					ID:   "call_123",
					Type: types.Function,
					Function: types.ChatCompletionMessageToolCallFunction{
						Name:      "mcp_test_tool",
						Arguments: `{"arg1":"value1","arg2":42}`,
					},
				},
			},
		},
		{
			name: "multiple tool calls by index",
			body: `data: {"choices":[{"delta":{"tool_calls":[{"index":0,"id":"call_1","type":"function","function":{"name":"tool_one"}},{"index":1,"id":"call_2","type":"function","function":{"name":"tool_two"}}]}}]}
data: {"choices":[{"delta":{"tool_calls":[{"index":0,"function":{"arguments":"{\"x\":1}"}},{"index":1,"function":{"arguments":"{\"y\":2}"}}]}}]}
data: [DONE]`,
			expected: []types.ChatCompletionMessageToolCall{
				{
					ID:   "call_1",
					Type: types.Function,
					Function: types.ChatCompletionMessageToolCallFunction{
						Name:      "tool_one",
						Arguments: `{"x":1}`,
					},
				},
				{
					ID:   "call_2",
					Type: types.Function,
					Function: types.ChatCompletionMessageToolCallFunction{
						Name:      "tool_two",
						Arguments: `{"y":2}`,
					},
				},
			},
		},
		{
			name:     "content-only stream has no tool calls",
			body:     `data: {"choices":[{"delta":{"content":"hello"}}]}`,
			expected: nil,
		},
		{
			name:     "unnamed tool calls are dropped",
			body:     `data: {"choices":[{"delta":{"tool_calls":[{"index":0,"function":{"arguments":"{}"}}]}}]}`,
			expected: nil,
		},
		{
			name:     "malformed chunks are skipped",
			body:     "data: not-json\ndata: [DONE]",
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := types.AccumulateStreamingToolCalls(tt.body)
			require.Len(t, result, len(tt.expected))
			assert.Equal(t, tt.expected, result)
		})
	}
}
