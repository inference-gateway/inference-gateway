package providers

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTransformOllama(t *testing.T) {
	tests := []struct {
		name     string
		request  GenerateRequest
		expected GenerateRequestOllama
	}{
		{
			name: "basic user message only",
			request: GenerateRequest{
				Model: "llama2",
				Messages: []Message{
					{Role: MessageRoleUser, Content: "Hello"},
				},
				Stream: false,
			},
			expected: GenerateRequestOllama{
				Model: "llama2",
				Messages: []Message{
					{
						Role:    MessageRoleUser,
						Content: "Hello",
					},
				},
				Stream: false,
				Options: &OllamaOptions{
					Temperature: float64Ptr(0.7),
				},
			},
		},
		{
			name: "with system message",
			request: GenerateRequest{
				Model: "llama2",
				Messages: []Message{
					{Role: MessageRoleSystem, Content: "You are a helpful assistant"},
					{Role: MessageRoleUser, Content: "Hello"},
				},
				Stream: true,
			},
			expected: GenerateRequestOllama{
				Model: "llama2",
				Messages: []Message{
					{
						Role:    MessageRoleSystem,
						Content: "You are a helpful assistant",
					},
					{
						Role:    MessageRoleUser,
						Content: "Hello",
					},
				},
				Stream: true,
				Options: &OllamaOptions{
					Temperature: float64Ptr(0.7),
				},
			},
		},
		{
			name: "with tools",
			request: GenerateRequest{
				Model: "llama2",
				Messages: []Message{
					{Role: MessageRoleUser, Content: "Calculate 2+2"},
				},
				Stream: false,
				Tools: []Tool{
					{
						Type: "function",
						Function: &FunctionTool{
							Name:        "calculate",
							Description: "Calculate a math expression",
							Parameters: ToolParams{
								Type: "object",
								Properties: map[string]ToolProperty{
									"expression": {
										Type:        "string",
										Description: "Math expression to evaluate",
									},
								},
								Required: []string{"expression"},
							},
						},
					},
				},
			},
			expected: GenerateRequestOllama{
				Model: "llama2",
				Messages: []Message{
					{
						Role:    MessageRoleUser,
						Content: "Calculate 2+2",
					},
				},
				Stream: false,
				Options: &OllamaOptions{
					Temperature: float64Ptr(0.7),
				},
				Tools: []Tool{
					{
						Type: "function",
						Function: &FunctionTool{
							Name:        "calculate",
							Description: "Calculate a math expression",
							Parameters: ToolParams{
								Type: "object",
								Properties: map[string]ToolProperty{
									"expression": {
										Type:        "string",
										Description: "Math expression to evaluate",
									},
								},
								Required: []string{"expression"},
							},
						},
					},
				},
			},
		},
		{
			name: "multiple messages with system",
			request: GenerateRequest{
				Model: "llama2",
				Messages: []Message{
					{Role: MessageRoleSystem, Content: "You are a helpful assistant"},
					{Role: MessageRoleUser, Content: "Hi"},
					{Role: MessageRoleAssistant, Content: "Hello! How can I help?"},
					{Role: MessageRoleUser, Content: "What's the weather?"},
				},
				Stream: true,
			},
			expected: GenerateRequestOllama{
				Model: "llama2",
				Messages: []Message{
					{Role: MessageRoleSystem, Content: "You are a helpful assistant"},
					{Role: MessageRoleUser, Content: "Hi"},
					{Role: MessageRoleAssistant, Content: "Hello! How can I help?"},
					{Role: MessageRoleUser, Content: "What's the weather?"},
				},
				Stream: true,
				Options: &OllamaOptions{
					Temperature: float64Ptr(0.7),
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.request.TransformOllama()
			assert.Equal(t, tt.expected, result)
		})
	}
}
