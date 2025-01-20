package providers

type GenerateMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type GenerateRequest struct {
	Model    string            `json:"model"`
	Messages []GenerateMessage `json:"messages"`
}

type ResponseTokens struct {
	Role    string `json:"role"`
	Model   string `json:"model"`
	Content string `json:"content"`
}

type GenerateResponse struct {
	Provider string         `json:"provider"`
	Response ResponseTokens `json:"response"`
}
