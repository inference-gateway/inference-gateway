package providers

// HasImageContent checks if the message contains image content
func (m *Message) HasImageContent() bool {
	if contentArray, ok := m.Content.([]any); ok {
		for _, item := range contentArray {
			if itemMap, ok := item.(map[string]any); ok {
				if itemType, ok := itemMap["type"].(string); ok && itemType == "image_url" {
					return true
				}
			}
		}
	}
	return false
}

// GetTextContent extracts text content from the message
func (m *Message) GetTextContent() string {
	if content, ok := m.Content.(string); ok {
		return content
	}

	if contentArray, ok := m.Content.([]any); ok {
		for _, item := range contentArray {
			if itemMap, ok := item.(map[string]any); ok {
				if itemType, ok := itemMap["type"].(string); ok && itemType == "text" {
					if text, ok := itemMap["text"].(string); ok {
						return text
					}
				}
			}
		}
	}

	return ""
}

// StripImageContent removes image content from the message, keeping only text parts
func (m *Message) StripImageContent() {
	if _, ok := m.Content.(string); ok {
		return
	}

	if contentArray, ok := m.Content.([]any); ok {
		var textOnlyContent []any

		for _, item := range contentArray {
			if itemMap, ok := item.(map[string]any); ok {
				if itemType, ok := itemMap["type"].(string); ok {
					if itemType == "text" {
						textOnlyContent = append(textOnlyContent, item)
					}
				}
			}
		}

		switch len(textOnlyContent) {
		case 0:
			m.Content = ""
		case 1:
			if itemMap, ok := textOnlyContent[0].(map[string]any); ok {
				if text, ok := itemMap["text"].(string); ok {
					m.Content = text
				}
			}
		default:
			m.Content = textOnlyContent
		}
	}
}
