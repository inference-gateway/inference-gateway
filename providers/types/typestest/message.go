// Package typestest provides test helpers for constructing typed provider messages.
// It mirrors net/http/httptest: test-only code lives next to the package it
// supports, keeping providers/types itself free of any testing import.
package typestest

import (
	"testing"

	types "github.com/inference-gateway/inference-gateway/providers/types"
)

// NewTextMessage creates a message with simple text content.
// Calls t.Fatal() on error.
func NewTextMessage(t testing.TB, role types.MessageRole, text string) types.Message {
	t.Helper()
	msg := types.Message{
		Role: role,
	}
	if err := msg.Content.FromMessageContent0(text); err != nil {
		t.Fatalf("NewTextMessage: %v", err)
	}
	return msg
}

// NewMultimodalMessage creates a message with multimodal content parts.
// Calls t.Fatal() on error.
func NewMultimodalMessage(t testing.TB, role types.MessageRole, parts ...types.ContentPart) types.Message {
	t.Helper()
	msg := types.Message{
		Role: role,
	}
	if err := msg.Content.FromMessageContent1(parts); err != nil {
		t.Fatalf("NewMultimodalMessage: %v", err)
	}
	return msg
}

// NewTextContentPart creates a text content part.
// Calls t.Fatal() on error.
func NewTextContentPart(t testing.TB, text string) types.ContentPart {
	t.Helper()
	var part types.ContentPart
	if err := part.FromTextContentPart(types.TextContentPart{
		Type: "text",
		Text: text,
	}); err != nil {
		t.Fatalf("NewTextContentPart: %v", err)
	}
	return part
}

// NewImageContentPart creates an image content part with URL.
// Calls t.Fatal() on error.
func NewImageContentPart(t testing.TB, url string, detail *types.ImageURLDetail) types.ContentPart {
	t.Helper()
	var part types.ContentPart
	imageURL := types.ImageURL{
		URL:    url,
		Detail: detail,
	}
	if err := part.FromImageContentPart(types.ImageContentPart{
		Type:     "image_url",
		ImageURL: imageURL,
	}); err != nil {
		t.Fatalf("NewImageContentPart: %v", err)
	}
	return part
}

// NewAssistantMessage creates an assistant message with optional tool calls.
// Calls t.Fatal() on error.
func NewAssistantMessage(t testing.TB, content string, toolCalls *[]types.ChatCompletionMessageToolCall) types.Message {
	t.Helper()
	msg := types.Message{
		Role:      types.Assistant,
		ToolCalls: toolCalls,
	}
	if content != "" {
		if err := msg.Content.FromMessageContent0(content); err != nil {
			t.Fatalf("NewAssistantMessage: %v", err)
		}
	}
	return msg
}
