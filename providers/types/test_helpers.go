package types

// Test helper functions for constructing messages with type-safe union types.
// These functions simplify message construction in tests and examples.

// NewTextMessage creates a message with simple text content
func NewTextMessage(role MessageRole, text string) Message {
	msg := Message{
		Role: role,
	}
	msg.Content.FromMessageContent0(text)
	return msg
}

// NewMultimodalMessage creates a message with multimodal content parts
func NewMultimodalMessage(role MessageRole, parts ...ContentPart) Message {
	msg := Message{
		Role: role,
	}
	msg.Content.FromMessageContent1(parts)
	return msg
}

// NewTextContentPart creates a text content part
func NewTextContentPart(text string) ContentPart {
	var part ContentPart
	part.FromTextContentPart(TextContentPart{
		Type: "text",
		Text: text,
	})
	return part
}

// NewImageContentPart creates an image content part with URL
func NewImageContentPart(url string, detail *ImageURLDetail) ContentPart {
	var part ContentPart
	imageURL := ImageURL{
		URL:    url,
		Detail: detail,
	}
	part.FromImageContentPart(ImageContentPart{
		Type:     "image_url",
		ImageURL: imageURL,
	})
	return part
}

// NewToolMessage creates a tool response message
func NewToolMessage(toolCallID string, content string) Message {
	msg := Message{
		Role:       Tool,
		ToolCallID: &toolCallID,
	}
	msg.Content.FromMessageContent0(content)
	return msg
}

// NewAssistantMessage creates an assistant message with optional tool calls
func NewAssistantMessage(content string, toolCalls *[]ChatCompletionMessageToolCall) Message {
	msg := Message{
		Role:      Assistant,
		ToolCalls: toolCalls,
	}
	if content != "" {
		msg.Content.FromMessageContent0(content)
	}
	return msg
}
