package providers

func getSystemMessage(messages []Message) string {
	for _, message := range messages {
		if message.Role == "system" {
			return message.Content
		}
	}
	return ""
}

func getUserMessage(messages []Message) string {
	var prompt string
	for _, message := range messages {
		if message.Role == "user" {
			prompt += message.Content
		}
	}
	return prompt
}
