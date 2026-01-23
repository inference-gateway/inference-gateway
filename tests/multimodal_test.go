package tests

import (
	"testing"

	"github.com/inference-gateway/inference-gateway/providers/types"
	assert "github.com/stretchr/testify/assert"
)

func TestMessage_HasImageContent(t *testing.T) {
	tests := []struct {
		name     string
		message  types.Message
		expected bool
	}{
		{
			name:     "String content has no images",
			message:  types.NewTextMessage(types.User, "Hello, how are you?"),
			expected: false,
		},
		{
			name: "Array content with only text",
			message: types.NewMultimodalMessage(
				types.User,
				types.NewTextContentPart("Hello, how are you?"),
			),
			expected: false,
		},
		{
			name: "Array content with image",
			message: types.NewMultimodalMessage(
				types.User,
				types.NewTextContentPart("What's in this image?"),
				types.NewImageContentPart("data:image/jpeg;base64,/9j/4AAQSkZJRgABAQEAAA...", nil),
			),
			expected: true,
		},
		{
			name: "Array content with only image",
			message: func() types.Message {
				detail := types.ImageURLDetail("high")
				return types.NewMultimodalMessage(
					types.User,
					types.NewImageContentPart("https://example.com/image.jpg", &detail),
				)
			}(),
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
		message  types.Message
		expected string
	}{
		{
			name:     "String content returns text",
			message:  types.NewTextMessage(types.User, "Hello, world!"),
			expected: "Hello, world!",
		},
		{
			name: "Array content with text returns first text part",
			message: types.NewMultimodalMessage(
				types.User,
				types.NewTextContentPart("First text part"),
				types.NewTextContentPart("Second text part"),
			),
			expected: "First text part",
		},
		{
			name: "Array content with mixed types returns first text",
			message: types.NewMultimodalMessage(
				types.User,
				types.NewImageContentPart("https://example.com/image.jpg", nil),
				types.NewTextContentPart("What's in this image?"),
			),
			expected: "What's in this image?",
		},
		{
			name: "Array content with only image returns empty string",
			message: types.NewMultimodalMessage(
				types.User,
				types.NewImageContentPart("https://example.com/image.jpg", nil),
			),
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

func TestMessage_StripImageContent(t *testing.T) {
	tests := []struct {
		name            string
		message         types.Message
		expectedContent string
		checkAsString   bool
		checkAsParts    bool
		expectedParts   int
	}{
		{
			name:            "String content remains unchanged",
			message:         types.NewTextMessage(types.User, "Hello, world!"),
			expectedContent: "Hello, world!",
			checkAsString:   true,
		},
		{
			name: "Array with only text remains as single string",
			message: types.NewMultimodalMessage(
				types.User,
				types.NewTextContentPart("Just text"),
			),
			expectedContent: "Just text",
			checkAsString:   true,
		},
		{
			name: "Array with text and image keeps only text as string",
			message: types.NewMultimodalMessage(
				types.User,
				types.NewTextContentPart("What's in this image?"),
				types.NewImageContentPart("data:image/jpeg;base64,/9j/4AAQSkZJRgABAQEAAA...", nil),
			),
			expectedContent: "What's in this image?",
			checkAsString:   true,
		},
		{
			name: "Array with only images becomes empty string",
			message: types.NewMultimodalMessage(
				types.User,
				types.NewImageContentPart("https://example.com/image.jpg", nil),
			),
			expectedContent: "",
			checkAsString:   true,
		},
		{
			name: "Array with multiple text parts and images keeps only text parts",
			message: types.NewMultimodalMessage(
				types.User,
				types.NewTextContentPart("First part"),
				types.NewImageContentPart("https://example.com/image1.jpg", nil),
				types.NewTextContentPart("Second part"),
				types.NewImageContentPart("https://example.com/image2.jpg", nil),
			),
			checkAsParts:  true,
			expectedParts: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.message.StripImageContent()

			if tt.checkAsString {
				content, err := tt.message.Content.AsMessageContent0()
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedContent, content)
			}

			if tt.checkAsParts {
				parts, err := tt.message.Content.AsMessageContent1()
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedParts, len(parts))

				// Verify all parts are text parts
				for i, part := range parts {
					textPart, err := part.AsTextContentPart()
					assert.NoError(t, err, "Part %d should be a text part", i)
					assert.NotEmpty(t, textPart.Text, "Part %d should have non-empty text", i)
				}
			}
		})
	}
}
