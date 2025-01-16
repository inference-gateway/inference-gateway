package providers

type GenerateRequestOpenAIMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type GenerateRequestOpenAI struct {
	Model    string                         `json:"model"`
	Messages []GenerateRequestOpenAIMessage `json:"messages"`
}

type GenerateResponseOpenAIMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type GenerateResponseOpenAIChoice struct {
	Message GenerateResponseOpenAIMessage `json:"message"`
}

type GenerateResponseOpenAI struct {
	Model   string                         `json:"model"`
	Choices []GenerateResponseOpenAIChoice `json:"choices"`
}
