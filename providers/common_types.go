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
	AnthropicID  = "anthropic"
	CloudflareID = "cloudflare"
	CohereID     = "cohere"
	GoogleID     = "google"
	GroqID       = "groq"
	OllamaID     = "ollama"
	OpenaiID     = "openai"
)

// Display names for providers
const (
	AnthropicDisplayName  = "Anthropic"
	CloudflareDisplayName = "Cloudflare"
	CohereDisplayName     = "Cohere"
	GoogleDisplayName     = "Google"
	GroqDisplayName       = "Groq"
	OllamaDisplayName     = "Ollama"
	OpenaiDisplayName     = "Openai"
)

const (
	RoleSystem    = "system"
	RoleUser      = "user"
	RoleAssistant = "assistant"
)

// Common response and request types
type AuthType struct {
}
type GenerateRequest struct {
	Messages []Message `json:"messages"`
	Model    string    `json:"model"`
}
type GenerateResponse struct {
	Provider string         `json:"provider"`
	Response ResponseTokens `json:"response"`
}
type ListModelsResponse struct {
	Models   []Model `json:"models"`
	Provider string  `json:"provider"`
}
type Message struct {
	Content string `json:"content"`
	Role    string `json:"role"`
}
type Model struct {
	Name string `json:"name"`
}
type ResponseTokens struct {
	Content string `json:"content"`
	Model   string `json:"model"`
	Role    string `json:"role"`
}

func float64Ptr(v float64) *float64 {
	return &v
}

func intPtr(v int) *int {
	return &v
}
