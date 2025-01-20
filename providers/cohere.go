package providers

type GenerateRequestCohere struct {
	Model    string            `json:"model"`
	Messages []GenerateMessage `json:"messages"`
}

type GenerateResponseCohereContent struct {
	TypeStr string `json:"type"`
	Text    string `json:"text"`
}

type GenerateResponseCohereMessage struct {
	Role    string                          `json:"role"`
	Content []GenerateResponseCohereContent `json:"content"`
}

type GenerateResponseCohere struct {
	Message GenerateResponseCohereMessage `json:"message"`
}
