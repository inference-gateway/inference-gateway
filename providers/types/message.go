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

// StripImageContent removes every image part from multimodal content, leaving
// string content unchanged. No remaining text part yields an empty string, one
// yields that text and several stay an array of text parts. It returns an error
// if content conversion fails.
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
