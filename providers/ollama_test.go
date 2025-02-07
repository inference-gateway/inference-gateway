package providers_test

import (
	"testing"

	"github.com/inference-gateway/inference-gateway/providers"
	"github.com/stretchr/testify/assert"
)

func TestTransformOllama(t *testing.T) {
	tests := []struct {
		name     string
		request  providers.GenerateRequest
		expected providers.GenerateRequestOllama
	}{
		{
			name: "basic user message only",
			request: providers.GenerateRequest{
				Model: "llama2",
				Messages: []providers.Message{
					{Role: providers.MessageRoleUser, Content: "Hello"},
				},
				Stream: false,
			},
			expected: providers.GenerateRequestOllama{
				Model: "llama2",
				Messages: []providers.Message{
					{
						Role:    providers.MessageRoleUser,
						Content: "Hello",
					},
				},
				Stream: false,
				Options: &providers.OllamaOptions{
					Temperature: providers.Float64Ptr(0.7),
				},
			},
		},
		{
			name: "with system message",
			request: providers.GenerateRequest{
				Model: "llama2",
				Messages: []providers.Message{
					{Role: providers.MessageRoleSystem, Content: "You are a helpful assistant"},
					{Role: providers.MessageRoleUser, Content: "Hello"},
				},
				Stream: true,
			},
			expected: providers.GenerateRequestOllama{
				Model: "llama2",
				Messages: []providers.Message{
					{
						Role:    providers.MessageRoleSystem,
						Content: "You are a helpful assistant",
					},
					{
						Role:    providers.MessageRoleUser,
						Content: "Hello",
					},
				},
				Stream: true,
				Options: &providers.OllamaOptions{
					Temperature: providers.Float64Ptr(0.7),
				},
			},
		},
		{
			name: "with tools",
			request: providers.GenerateRequest{
				Model: "llama2",
				Messages: []providers.Message{
					{Role: providers.MessageRoleUser, Content: "Calculate 2+2"},
				},
				Stream: false,
				Tools: []providers.Tool{
					{
						Type: "function",
						Function: &providers.FunctionTool{
							Name:        "calculate",
							Description: "Calculate a math expression",
							Parameters: providers.ToolParams{
								Type: "object",
								Properties: map[string]providers.ToolProperty{
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
			expected: providers.GenerateRequestOllama{
				Model: "llama2",
				Messages: []providers.Message{
					{
						Role:    providers.MessageRoleUser,
						Content: "Calculate 2+2",
					},
				},
				Stream: false,
				Options: &providers.OllamaOptions{
					Temperature: providers.Float64Ptr(0.7),
				},
				Tools: []providers.Tool{
					{
						Type: "function",
						Function: &providers.FunctionTool{
							Name:        "calculate",
							Description: "Calculate a math expression",
							Parameters: providers.ToolParams{
								Type: "object",
								Properties: map[string]providers.ToolProperty{
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
			request: providers.GenerateRequest{
				Model: "llama2",
				Messages: []providers.Message{
					{Role: providers.MessageRoleSystem, Content: "You are a helpful assistant"},
					{Role: providers.MessageRoleUser, Content: "Hi"},
					{Role: providers.MessageRoleAssistant, Content: "Hello! How can I help?"},
					{Role: providers.MessageRoleUser, Content: "What's the weather?"},
				},
				Stream: true,
			},
			expected: providers.GenerateRequestOllama{
				Model: "llama2",
				Messages: []providers.Message{
					{Role: providers.MessageRoleSystem, Content: "You are a helpful assistant"},
					{Role: providers.MessageRoleUser, Content: "Hi"},
					{Role: providers.MessageRoleAssistant, Content: "Hello! How can I help?"},
					{Role: providers.MessageRoleUser, Content: "What's the weather?"},
				},
				Stream: true,
				Options: &providers.OllamaOptions{
					Temperature: providers.Float64Ptr(0.7),
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
