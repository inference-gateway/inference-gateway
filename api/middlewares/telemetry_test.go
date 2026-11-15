package middlewares

import (
	"testing"

	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
	gomock "go.uber.org/mock/gomock"

	mocks "github.com/inference-gateway/inference-gateway/tests/mocks"
)

func TestParseStreamingResponse_optionalDataSpace(t *testing.T) {
	const chunk = `{"choices":[{"delta":{"tool_calls":[{"index":0,"id":"call_123","type":"function","function":{"name":"test_tool","arguments":"{}"}}]}}],"usage":{"prompt_tokens":10,"completion_tokens":3,"total_tokens":13}}`
	for _, prefix := range []string{"data:", "data: "} {
		t.Run(prefix, func(t *testing.T) {
			middleware := &TelemetryMiddleware{}
			result := middleware.parseStreamingResponse([]byte(prefix+chunk+"\n\n"+prefix+"[DONE]\n\n"), "", "")
			assert.EqualValues(t, 10, result.PromptTokens)
			assert.EqualValues(t, 3, result.CompletionTokens)
			assert.EqualValues(t, 13, result.TotalTokens)
			require.Len(t, result.ToolCalls, 1)
			assert.Equal(t, "test_tool", result.ToolCalls[0].Function.Name)
		})
	}
}

func TestParseStreamingResponse_emptyData(t *testing.T) {
	const CHUNK = `{"usage":{"prompt_tokens":10,"completion_tokens":3,"total_tokens":13}}`
	for _, frame := range []string{"data:", "data: ", "data:\t"} {
		t.Run(frame, func(t *testing.T) {
			middleware := &TelemetryMiddleware{logger: mocks.NewMockLogger(gomock.NewController(t))}
			result := middleware.parseStreamingResponse([]byte("data:"+CHUNK+"\n\n"+frame+"\n\ndata:[DONE]\n\n"), "", "")
			assert.EqualValues(t, 10, result.PromptTokens)
			assert.EqualValues(t, 3, result.CompletionTokens)
			assert.EqualValues(t, 13, result.TotalTokens)
			assert.Empty(t, result.ToolCalls)
		})
	}
}
