package providers

type GenerateRequestCohereMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type GenerateRequestCohere struct {
	Model    string                         `json:"model"`
	Messages []GenerateRequestCohereMessage `json:"messages"`
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
