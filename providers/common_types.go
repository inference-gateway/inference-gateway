package providers

const (
	OllamaID     = "ollama"
	GroqID       = "groq"
	OpenaiID     = "openai"
	AnthropicID  = "anthropic"
	CohereID     = "cohere"
	CloudflareID = "cloudflare"
	GoogleID     = "google"
)

type Model struct {
	Name string `json:"name"`
}

type ModelsResponse struct {
	Provider string        `json:"provider"`
	Models   []interface{} `json:"models"`
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type GenerateRequest struct {
	Model    string    `json:"model"`
	Messages []Message `json:"messages"`
}

type ResponseTokens struct {
	Role    string `json:"role"`
	Model   string `json:"model"`
	Content string `json:"content"`
}

type GetModelsResponse struct {
	Object string        `json:"object"`
	Data   []interface{} `json:"data"`
}

type GenerateResponse struct {
	Provider string         `json:"provider"`
	Response ResponseTokens `json:"response"`
}
