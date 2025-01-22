package providers

type GenerateRequestAnthropic struct {
	Model    string            `json:"model"`
	Messages []GenerateMessage `json:"messages"`
}

type GenerateResponseAnthropicChoice struct {
	Message GenerateMessage `json:"message"`
}

type GenerateResponseAnthropic struct {
	Model   string                            `json:"model"`
	Choices []GenerateResponseAnthropicChoice `json:"choices"`
}
