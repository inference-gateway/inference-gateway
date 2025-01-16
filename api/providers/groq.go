package providers

type GenerateRequestGroqMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type GenerateRequestGroq struct {
	Model    string                       `json:"model"`
	Messages []GenerateRequestGroqMessage `json:"messages"`
}

type GenerateResponseGroqMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type GenerateResponseGroqChoice struct {
	Message GenerateResponseGroqMessage `json:"message"`
}

type GenerateResponseGroq struct {
	Model   string                       `json:"model"`
	Choices []GenerateResponseGroqChoice `json:"choices"`
}
