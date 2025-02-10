package providers_test

import (
	"bufio"
	"strings"
	"testing"

	"github.com/inference-gateway/inference-gateway/providers"
	"github.com/inference-gateway/inference-gateway/tests/mocks"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

func TestGenerateRequestTransformGroq(t *testing.T) {
	tests := []struct {
		name     string
		request  providers.GenerateRequest
		expected providers.GenerateRequestGroq
	}{
		{
			name: "basic request",
			request: providers.GenerateRequest{
				Model:    "llama2",
				Messages: []providers.Message{{Role: "user", Content: "Hello"}},
				Stream:   true,
			},
			expected: providers.GenerateRequestGroq{
				Model:       "llama2",
				Messages:    []providers.Message{{Role: "user", Content: "Hello"}},
				Stream:      providers.BoolPtr(true),
				Temperature: providers.Float64Ptr(1.0),
			},
		},
		{
			name: "request with tools",
			request: providers.GenerateRequest{
				Model:    "llama2",
				Messages: []providers.Message{{Role: "user", Content: "Calculate"}},
				Stream:   false,
				Tools: []providers.Tool{
					{
						Type: "function",
						Function: &providers.FunctionTool{
							Name: "calculate",
							Parameters: providers.ToolParams{
								Type: "object",
							},
						},
					},
				},
			},
			expected: providers.GenerateRequestGroq{
				Model:       "llama2",
				Messages:    []providers.Message{{Role: "user", Content: "Calculate"}},
				Stream:      providers.BoolPtr(false),
				Temperature: providers.Float64Ptr(1.0),
				Tools: []providers.Tool{
					{
						Type: "function",
						Function: &providers.FunctionTool{
							Name: "calculate",
							Parameters: providers.ToolParams{
								Type: "object",
							},
						},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.request.TransformGroq()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGenerateResponseGroqTransform(t *testing.T) {
	tests := []struct {
		name     string
		response providers.GenerateResponseGroq
		expected providers.GenerateResponse
	}{
		{
			name: "message with content",
			response: providers.GenerateResponseGroq{
				Model: "llama2",
				Choices: []providers.GroqChoice{
					{
						Message: providers.Message{
							Content: "Hello!",
							Role:    "assistant",
						},
					},
				},
			},
			expected: providers.GenerateResponse{
				Provider: providers.GroqDisplayName,
				Response: providers.ResponseTokens{
					Content: "Hello!",
					Model:   "llama2",
					Role:    "assistant",
				},
			},
		},
		{
			name: "message with tool calls",
			response: providers.GenerateResponseGroq{
				Model: "llama2",
				Choices: []providers.GroqChoice{
					{
						Message: providers.Message{
							Reasoning: "Let me calculate that",
							ToolCalls: []providers.ToolCall{{Function: providers.FunctionToolCall{Name: "calculate"}}},
						},
					},
				},
			},
			expected: providers.GenerateResponse{
				Provider: providers.GroqDisplayName,
				Response: providers.ResponseTokens{
					Content:   "Let me calculate that",
					Model:     "llama2",
					Role:      "assistant",
					ToolCalls: []providers.ToolCall{{Function: providers.FunctionToolCall{Name: "calculate"}}},
				},
			},
		},
		{
			name: "stream start",
			response: providers.GenerateResponseGroq{
				Model: "llama2",
				Choices: []providers.GroqChoice{
					{
						Delta: providers.GroqDelta{
							Role:    "assistant",
							Content: "",
						},
					},
				},
			},
			expected: providers.GenerateResponse{
				Provider: providers.GroqDisplayName,
				Response: providers.ResponseTokens{
					Model: "llama2",
					Role:  "assistant",
				},
				EventType: providers.EventMessageStart,
			},
		},
		{
			name: "stream end",
			response: providers.GenerateResponseGroq{
				Model: "llama2",
				Choices: []providers.GroqChoice{
					{
						FinishReason: "stop",
					},
				},
			},
			expected: providers.GenerateResponse{
				Provider: providers.GroqDisplayName,
				Response: providers.ResponseTokens{
					Model: "llama2",
					Role:  "assistant",
				},
				EventType: providers.EventStreamEnd,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.response.Transform()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGroqStreamParser(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := mocks.NewMockLogger(ctrl)
	parser := providers.NewGroqStreamParser(mockLogger)

	tests := []struct {
		name        string
		input       string
		expected    *providers.SSEvent
		expectError bool
	}{
		{
			name: "valid message chunk",
			input: `data: {"id":"123","choices":[{"delta":{"content":"Hello"}}]}
`,
			expected: &providers.SSEvent{
				EventType: "content-delta",
				Data:      []byte(`{"id":"123","choices":[{"delta":{"content":"Hello"}}]}`),
			},
			expectError: false,
		},
		{
			name:        "invalid chunk - empty input",
			input:       "",
			expected:    nil,
			expectError: true,
		},
		{
			name:        "invalid chunk - only whitespace",
			input:       "    \n\t",
			expected:    nil,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := bufio.NewReader(strings.NewReader(tt.input))
			result, err := parser.ParseChunk(reader)

			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}
