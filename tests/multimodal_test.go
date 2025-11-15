package tests

import (
	"testing"

	providers "github.com/inference-gateway/inference-gateway/providers"
	assert "github.com/stretchr/testify/assert"
)

func TestMessage_HasImageContent(t *testing.T) {
	tests := []struct {
		name     string
		message  providers.Message
		expected bool
	}{
		{
			name: "String content has no images",
			message: providers.Message{
				Role:    providers.MessageRoleUser,
				Content: "Hello, how are you?",
			},
			expected: false,
		},
		{
			name: "Array content with only text",
			message: providers.Message{
				Role: providers.MessageRoleUser,
				Content: []any{
					map[string]any{
						"type": "text",
						"text": "Hello, how are you?",
					},
				},
			},
			expected: false,
		},
		{
			name: "Array content with image",
			message: providers.Message{
				Role: providers.MessageRoleUser,
				Content: []any{
					map[string]any{
						"type": "text",
						"text": "What's in this image?",
					},
					map[string]any{
						"type": "image_url",
						"image_url": map[string]any{
							"url": "data:image/jpeg;base64,/9j/4AAQSkZJRgABAQEAAA...",
						},
					},
				},
			},
			expected: true,
		},
		{
			name: "Array content with only image",
			message: providers.Message{
				Role: providers.MessageRoleUser,
				Content: []any{
					map[string]any{
						"type": "image_url",
						"image_url": map[string]any{
							"url":    "https://example.com/image.jpg",
							"detail": "high",
						},
					},
				},
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.message.HasImageContent()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestMessage_GetTextContent(t *testing.T) {
	tests := []struct {
		name     string
		message  providers.Message
		expected string
	}{
		{
			name: "String content returns text",
			message: providers.Message{
				Role:    providers.MessageRoleUser,
				Content: "Hello, world!",
			},
			expected: "Hello, world!",
		},
		{
			name: "Array content with text returns first text part",
			message: providers.Message{
				Role: providers.MessageRoleUser,
				Content: []any{
					map[string]any{
						"type": "text",
						"text": "First text part",
					},
					map[string]any{
						"type": "text",
						"text": "Second text part",
					},
				},
			},
			expected: "First text part",
		},
		{
			name: "Array content with mixed types returns first text",
			message: providers.Message{
				Role: providers.MessageRoleUser,
				Content: []any{
					map[string]any{
						"type": "image_url",
						"image_url": map[string]any{
							"url": "https://example.com/image.jpg",
						},
					},
					map[string]any{
						"type": "text",
						"text": "What's in this image?",
					},
				},
			},
			expected: "What's in this image?",
		},
		{
			name: "Array content with only image returns empty string",
			message: providers.Message{
				Role: providers.MessageRoleUser,
				Content: []any{
					map[string]any{
						"type": "image_url",
						"image_url": map[string]any{
							"url": "https://example.com/image.jpg",
						},
					},
				},
			},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.message.GetTextContent()
			assert.Equal(t, tt.expected, result)
		})
	}
}
