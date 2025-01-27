package providers

const (
	// Ollama endpoints
	OllamaListEndpoint     = "/api/tags"
	OllamaGenerateEndpoint = "/api/generate"

	// OpenAI endpoints
	OpenAIListEndpoint     = "/v1/models"
	OpenAIGenerateEndpoint = "/v1/chat/completions"

	// Groq endpoints
	GroqListEndpoint     = "/openai/v1/models"
	GroqGenerateEndpoint = "/openai/v1/chat/completions"

	// Google endpoints
	GoogleListEndpoint     = "/v1beta/models"
	GoogleGenerateEndpoint = "/v1beta/models/{model}:generateContent"

	// Cohere endpoints
	CohereListEndpoint     = "/v1/models"
	CohereGenerateEndpoint = "/v2/chat"

	// Cloudflare endpoints
	CloudflareListEndpoint     = "/ai/finetunes/public"
	CloudflareGenerateEndpoint = "/ai/run/@cf/meta/{model}"

	// Anthropic endpoints
	AnthropicListEndpoint     = "/v1/models"
	AnthropicGenerateEndpoint = "/v1/messages"
)

// Endpoints exposed by each provider
type Endpoints struct {
	List     string
	Generate string
}

// Base provider configuration
type Config struct {
	ID           string
	Name         string
	URL          string
	Token        string
	AuthType     string
	ExtraHeaders map[string][]string
	Endpoints    Endpoints
}

// The registry of all providers
var Registry = map[string]Config{
	AnthropicID: {
		ID:       AnthropicID,
		Name:     AnthropicDisplayName,
		URL:      AnthropicDefaultBaseURL,
		AuthType: AuthTypeXheader,
		ExtraHeaders: map[string][]string{
			"anthropic-version": {"2023-06-01"},
		},
		Endpoints: Endpoints{
			List:     AnthropicListEndpoint,
			Generate: AnthropicGenerateEndpoint,
		},
	},
	CloudflareID: {
		ID:       CloudflareID,
		Name:     CloudflareDisplayName,
		URL:      CloudflareDefaultBaseURL,
		AuthType: AuthTypeBearer,
		Endpoints: Endpoints{
			List:     CloudflareListEndpoint,
			Generate: CloudflareGenerateEndpoint,
		},
	},
	CohereID: {
		ID:       CohereID,
		Name:     CohereDisplayName,
		URL:      CohereDefaultBaseURL,
		AuthType: AuthTypeBearer,
		Endpoints: Endpoints{
			List:     CohereListEndpoint,
			Generate: CohereGenerateEndpoint,
		},
	},
	GoogleID: {
		ID:       GoogleID,
		Name:     GoogleDisplayName,
		URL:      GoogleDefaultBaseURL,
		AuthType: AuthTypeQuery,
		Endpoints: Endpoints{
			List:     GoogleListEndpoint,
			Generate: GoogleGenerateEndpoint,
		},
	},
	GroqID: {
		ID:       GroqID,
		Name:     GroqDisplayName,
		URL:      GroqDefaultBaseURL,
		AuthType: AuthTypeBearer,
		Endpoints: Endpoints{
			List:     GroqListEndpoint,
			Generate: GroqGenerateEndpoint,
		},
	},
	OllamaID: {
		ID:       OllamaID,
		Name:     OllamaDisplayName,
		URL:      OllamaDefaultBaseURL,
		AuthType: AuthTypeNone,
		Endpoints: Endpoints{
			List:     OllamaListEndpoint,
			Generate: OllamaGenerateEndpoint,
		},
	},
	OpenaiID: {
		ID:       OpenaiID,
		Name:     OpenaiDisplayName,
		URL:      OpenaiDefaultBaseURL,
		AuthType: AuthTypeBearer,
		Endpoints: Endpoints{
			List:     OpenAIListEndpoint,
			Generate: OpenAIGenerateEndpoint,
		},
	},
}
