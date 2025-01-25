package providers

// The authentication type of the specific provider
const (
	AuthTypeBearer  = "bearer"
	AuthTypeXheader = "xheader"
	AuthTypeQuery   = "query"
	AuthTypeNone    = "none"
)

// The default base URLs of each provider
const (
	AnthropicDefaultBaseURL  = "https://api.anthropic.com"
	CloudflareDefaultBaseURL = "https://api.cloudflare.com/client/v4/accounts/{ACCOUNT_ID}"
	CohereDefaultBaseURL     = "https://api.cohere.com"
	GoogleDefaultBaseURL     = "https://generativelanguage.googleapis.com"
	GroqDefaultBaseURL       = "https://api.groq.com"
	OllamaDefaultBaseURL     = "http://ollama:8080"
	OpenaiDefaultBaseURL     = "https://api.openai.com"
)

// The ID's of each provider
const (
	OllamaID     = "ollama"
	GroqID       = "groq"
	OpenaiID     = "openai"
	AnthropicID  = "anthropic"
	CohereID     = "cohere"
	CloudflareID = "cloudflare"
	GoogleID     = "google"
)

// Display names for providers
const (
	OllamaDisplayName     = "Ollama"
	GroqDisplayName       = "Groq"
	AnthropicDisplayName  = "Anthropic"
	OpenaiDisplayName     = "Openai"
	CloudflareDisplayName = "Cloudflare"
	CohereDisplayName     = "Cohere"
	GoogleDisplayName     = "Google"
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
