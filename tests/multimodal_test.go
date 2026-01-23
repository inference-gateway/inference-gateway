package tests

import (
	"testing"

	"github.com/inference-gateway/inference-gateway/providers/types"
	assert "github.com/stretchr/testify/assert"
)

func TestMessage_HasImageContent(t *testing.T) {
	tests := []struct {
		name     string
		makeMsg  func(t *testing.T) types.Message
		expected bool
	}{
		{
			name: "String content has no images",
			makeMsg: func(t *testing.T) types.Message {
				return types.NewTextMessage(t, types.User, "Hello, how are you?")
			},
			expected: false,
		},
		{
			name: "Array content with only text",
			makeMsg: func(t *testing.T) types.Message {
				return types.NewMultimodalMessage(t, types.User,
					types.NewTextContentPart(t, "Hello, how are you?"))
			},
			expected: false,
		},
		{
			name: "Array content with image",
			makeMsg: func(t *testing.T) types.Message {
				return types.NewMultimodalMessage(t, types.User,
					types.NewTextContentPart(t, "What's in this image?"),
					types.NewImageContentPart(t, "data:image/jpeg;base64,/9j/4AAQSkZJRgABAQEAAA...", nil))
			},
			expected: true,
		},
		{
			name: "Array content with only image",
			makeMsg: func(t *testing.T) types.Message {
				detail := types.ImageURLDetail("high")
				return types.NewMultimodalMessage(t, types.User,
					types.NewImageContentPart(t, "https://example.com/image.jpg", &detail))
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := tt.makeMsg(t)
			result := msg.HasImageContent()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestMessage_GetTextContent(t *testing.T) {
	tests := []struct {
		name     string
		makeMsg  func(t *testing.T) types.Message
		expected string
	}{
		{
			name: "String content returns text",
			makeMsg: func(t *testing.T) types.Message {
				return types.NewTextMessage(t, types.User, "Hello, world!")
			},
			expected: "Hello, world!",
		},
		{
			name: "Array content with text returns first text part",
			makeMsg: func(t *testing.T) types.Message {
				return types.NewMultimodalMessage(t, types.User,
					types.NewTextContentPart(t, "First text part"),
					types.NewTextContentPart(t, "Second text part"))
			},
			expected: "First text part",
		},
		{
			name: "Array content with mixed types returns first text",
			makeMsg: func(t *testing.T) types.Message {
				return types.NewMultimodalMessage(t, types.User,
					types.NewImageContentPart(t, "https://example.com/image.jpg", nil),
					types.NewTextContentPart(t, "What's in this image?"))
			},
			expected: "What's in this image?",
		},
		{
			name: "Array content with only image returns empty string",
			makeMsg: func(t *testing.T) types.Message {
				return types.NewMultimodalMessage(t, types.User,
					types.NewImageContentPart(t, "https://example.com/image.jpg", nil))
			},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := tt.makeMsg(t)
			result := msg.GetTextContent()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestMessage_StripImageContent(t *testing.T) {
	tests := []struct {
		name            string
		makeMsg         func(t *testing.T) types.Message
		expectedContent string
		checkAsString   bool
		checkAsParts    bool
		expectedParts   int
	}{
		{
			name: "String content remains unchanged",
			makeMsg: func(t *testing.T) types.Message {
				return types.NewTextMessage(t, types.User, "Hello, world!")
			},
			expectedContent: "Hello, world!",
			checkAsString:   true,
		},
		{
			name: "Array with only text remains as single string",
			makeMsg: func(t *testing.T) types.Message {
				return types.NewMultimodalMessage(t, types.User,
					types.NewTextContentPart(t, "Just text"))
			},
			expectedContent: "Just text",
			checkAsString:   true,
		},
		{
			name: "Array with text and image keeps only text as string",
			makeMsg: func(t *testing.T) types.Message {
				return types.NewMultimodalMessage(t, types.User,
					types.NewTextContentPart(t, "What's in this image?"),
					types.NewImageContentPart(t, "data:image/jpeg;base64,/9j/4AAQSkZJRgABAQEAAA...", nil))
			},
			expectedContent: "What's in this image?",
			checkAsString:   true,
		},
		{
			name: "Array with only images becomes empty string",
			makeMsg: func(t *testing.T) types.Message {
				return types.NewMultimodalMessage(t, types.User,
					types.NewImageContentPart(t, "https://example.com/image.jpg", nil))
			},
			expectedContent: "",
			checkAsString:   true,
		},
		{
			name: "Array with multiple text parts and images keeps only text parts",
			makeMsg: func(t *testing.T) types.Message {
				return types.NewMultimodalMessage(t, types.User,
					types.NewTextContentPart(t, "First part"),
					types.NewImageContentPart(t, "https://example.com/image1.jpg", nil),
					types.NewTextContentPart(t, "Second part"),
					types.NewImageContentPart(t, "https://example.com/image2.jpg", nil))
			},
			checkAsParts:  true,
			expectedParts: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := tt.makeMsg(t)
			err := msg.StripImageContent()
			assert.NoError(t, err)

			if tt.checkAsString {
				content, err := msg.Content.AsMessageContent0()
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedContent, content)
			}

			if tt.checkAsParts {
				parts, err := msg.Content.AsMessageContent1()
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedParts, len(parts))

				for i, part := range parts {
					textPart, err := part.AsTextContentPart()
					assert.NoError(t, err, "Part %d should be a text part", i)
					assert.NotEmpty(t, textPart.Text, "Part %d should have non-empty text", i)
				}
			}
		})
	}
}
