// Code generated from OpenAPI schema. DO NOT EDIT.
package constants

import (
	types "github.com/inference-gateway/inference-gateway/providers/types"
)

// The authentication type of the specific provider
const (
	AuthTypeBearer  = "bearer"
	AuthTypeXheader = "xheader"
	AuthTypeQuery   = "query"
	AuthTypeNone    = "none"
)

// The default base URLs of each provider
const (
	AnthropicDefaultBaseURL   = "https://api.anthropic.com/v1"
	CloudflareDefaultBaseURL  = "https://api.cloudflare.com/client/v4/accounts/{ACCOUNT_ID}/ai"
	CohereDefaultBaseURL      = "https://api.cohere.ai"
	DeepseekDefaultBaseURL    = "https://api.deepseek.com"
	GoogleDefaultBaseURL      = "https://generativelanguage.googleapis.com/v1beta/openai"
	GroqDefaultBaseURL        = "https://api.groq.com/openai/v1"
	MistralDefaultBaseURL     = "https://api.mistral.ai/v1"
	MoonshotDefaultBaseURL    = "https://api.moonshot.ai/v1"
	OllamaDefaultBaseURL      = "http://ollama:8080/v1"
	OllamaCloudDefaultBaseURL = "https://ollama.com/v1"
	OpenaiDefaultBaseURL      = "https://api.openai.com/v1"
)

// The default endpoints of each provider
const (
	AnthropicModelsEndpoint   = "/models"
	AnthropicChatEndpoint     = "/chat/completions"
	CloudflareModelsEndpoint  = "/finetunes/public?limit=1000"
	CloudflareChatEndpoint    = "/v1/chat/completions"
	CohereModelsEndpoint      = "/v1/models"
	CohereChatEndpoint        = "/compatibility/v1/chat/completions"
	DeepseekModelsEndpoint    = "/models"
	DeepseekChatEndpoint      = "/chat/completions"
	GoogleModelsEndpoint      = "/models"
	GoogleChatEndpoint        = "/chat/completions"
	GroqModelsEndpoint        = "/models"
	GroqChatEndpoint          = "/chat/completions"
	MistralModelsEndpoint     = "/models"
	MistralChatEndpoint       = "/chat/completions"
	MoonshotModelsEndpoint    = "/models"
	MoonshotChatEndpoint      = "/chat/completions"
	OllamaModelsEndpoint      = "/models"
	OllamaChatEndpoint        = "/chat/completions"
	OllamaCloudModelsEndpoint = "/models"
	OllamaCloudChatEndpoint   = "/chat/completions"
	OpenaiModelsEndpoint      = "/models"
	OpenaiChatEndpoint        = "/chat/completions"
)

// The ID's of each provider
const (
	AnthropicID   types.Provider = "anthropic"
	CloudflareID  types.Provider = "cloudflare"
	CohereID      types.Provider = "cohere"
	DeepseekID    types.Provider = "deepseek"
	GoogleID      types.Provider = "google"
	GroqID        types.Provider = "groq"
	MistralID     types.Provider = "mistral"
	MoonshotID    types.Provider = "moonshot"
	OllamaID      types.Provider = "ollama"
	OllamaCloudID types.Provider = "ollama_cloud"
	OpenaiID      types.Provider = "openai"
)

// Display names for providers
const (
	AnthropicDisplayName   = "Anthropic"
	CloudflareDisplayName  = "Cloudflare"
	CohereDisplayName      = "Cohere"
	DeepseekDisplayName    = "Deepseek"
	GoogleDisplayName      = "Google"
	GroqDisplayName        = "Groq"
	MistralDisplayName     = "Mistral"
	MoonshotDisplayName    = "Moonshot"
	OllamaDisplayName      = "Ollama"
	OllamaCloudDisplayName = "OllamaCloud"
	OpenaiDisplayName      = "Openai"
)

// ListModelsTransformer interface for transforming provider-specific responses
type ListModelsTransformer interface {
	Transform() types.ListModelsResponse
}
