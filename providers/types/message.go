package types

// HasImageContent checks if the message contains image content.
// Returns true if the message has multimodal content with at least one image part.
func (m *Message) HasImageContent() bool {
	parts, err := m.Content.AsMessageContent1()
	if err != nil {
		return false
	}

	for _, part := range parts {
		if imagePart, err := part.AsImageContentPart(); err == nil {
			if imagePart.Type == "image_url" {
				return true
			}
		}
	}
	return false
}

// GetTextContent extracts the first text content from the message.
// For string content, returns the string directly.
// For multimodal content, returns the text from the first text part found.
// Returns empty string if no text content is found.
func (m *Message) GetTextContent() string {
	// Try string content first
	if content, err := m.Content.AsMessageContent0(); err == nil {
		return content
	}

	// Try array of ContentParts
	parts, err := m.Content.AsMessageContent1()
	if err != nil {
		return ""
	}

	// Find first text part by checking the Type field
	for _, part := range parts {
		if textPart, err := part.AsTextContentPart(); err == nil {
			if textPart.Type == "text" {
				return textPart.Text
			}
		}
	}
	return ""
}

// StripImageContent removes all image content from the message, keeping only text parts.
// For string content, the message is left unchanged.
// For multimodal content:
// - If no text parts remain, content becomes an empty string
// - If exactly one text part remains, content becomes that text string
// - If multiple text parts remain, content stays as an array of text parts
func (m *Message) StripImageContent() {
	if _, err := m.Content.AsMessageContent0(); err == nil {
		return
	}

	parts, err := m.Content.AsMessageContent1()
	if err != nil {
		return
	}

	var textParts []ContentPart
	for _, part := range parts {
		if textPart, err := part.AsTextContentPart(); err == nil {
			if textPart.Type == "text" {
				textParts = append(textParts, part)
			}
		}
	}

	switch len(textParts) {
	case 0:
		m.Content.FromMessageContent0("")
	case 1:
		if textPart, err := textParts[0].AsTextContentPart(); err == nil {
			m.Content.FromMessageContent0(textPart.Text)
		}
	default:
		m.Content.FromMessageContent1(textParts)
	}
}
