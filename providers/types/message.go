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
	if content, err := m.Content.AsMessageContent0(); err == nil {
		return content
	}

	parts, err := m.Content.AsMessageContent1()
	if err != nil {
		return ""
	}

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
// Returns an error if content conversion fails.
func (m *Message) StripImageContent() error {
	if _, err := m.Content.AsMessageContent0(); err == nil {
		return nil
	}

	parts, err := m.Content.AsMessageContent1()
	if err != nil {
		return nil
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
		if err := m.Content.FromMessageContent0(""); err != nil {
			return err
		}
	case 1:
		if textPart, err := textParts[0].AsTextContentPart(); err == nil {
			if err := m.Content.FromMessageContent0(textPart.Text); err != nil {
				return err
			}
		}
	default:
		if err := m.Content.FromMessageContent1(textParts); err != nil {
			return err
		}
	}
	return nil
}
