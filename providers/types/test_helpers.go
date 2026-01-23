package types

import "testing"

// Test helper functions for constructing messages with type-safe union types.
// These functions are designed for test code and will fail the test on error.

// NewTextMessage creates a message with simple text content.
// Calls t.Fatal() on error.
func NewTextMessage(t testing.TB, role MessageRole, text string) Message {
	t.Helper()
	msg := Message{
		Role: role,
	}
	if err := msg.Content.FromMessageContent0(text); err != nil {
		t.Fatalf("NewTextMessage: %v", err)
	}
	return msg
}

// NewMultimodalMessage creates a message with multimodal content parts.
// Calls t.Fatal() on error.
func NewMultimodalMessage(t testing.TB, role MessageRole, parts ...ContentPart) Message {
	t.Helper()
	msg := Message{
		Role: role,
	}
	if err := msg.Content.FromMessageContent1(parts); err != nil {
		t.Fatalf("NewMultimodalMessage: %v", err)
	}
	return msg
}

// NewTextContentPart creates a text content part.
// Calls t.Fatal() on error.
func NewTextContentPart(t testing.TB, text string) ContentPart {
	t.Helper()
	var part ContentPart
	if err := part.FromTextContentPart(TextContentPart{
		Type: "text",
		Text: text,
	}); err != nil {
		t.Fatalf("NewTextContentPart: %v", err)
	}
	return part
}

// NewImageContentPart creates an image content part with URL.
// Calls t.Fatal() on error.
func NewImageContentPart(t testing.TB, url string, detail *ImageURLDetail) ContentPart {
	t.Helper()
	var part ContentPart
	imageURL := ImageURL{
		URL:    url,
		Detail: detail,
	}
	if err := part.FromImageContentPart(ImageContentPart{
		Type:     "image_url",
		ImageURL: imageURL,
	}); err != nil {
		t.Fatalf("NewImageContentPart: %v", err)
	}
	return part
}

// NewToolMessage creates a tool response message.
// Calls t.Fatal() on error.
func NewToolMessage(t testing.TB, toolCallID string, content string) Message {
	t.Helper()
	msg := Message{
		Role:       Tool,
		ToolCallID: &toolCallID,
	}
	if err := msg.Content.FromMessageContent0(content); err != nil {
		t.Fatalf("NewToolMessage: %v", err)
	}
	return msg
}

// NewAssistantMessage creates an assistant message with optional tool calls.
// Calls t.Fatal() on error.
func NewAssistantMessage(t testing.TB, content string, toolCalls *[]ChatCompletionMessageToolCall) Message {
	t.Helper()
	msg := Message{
		Role:      Assistant,
		ToolCalls: toolCalls,
	}
	if content != "" {
		if err := msg.Content.FromMessageContent0(content); err != nil {
			t.Fatalf("NewAssistantMessage: %v", err)
		}
	}
	return msg
}
