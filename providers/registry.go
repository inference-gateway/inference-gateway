package providers

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
	},
	CloudflareID: {
		ID:       CloudflareID,
		Name:     CloudflareDisplayName,
		URL:      CloudflareDefaultBaseURL,
		AuthType: AuthTypeBearer,
	},
	CohereID: {
		ID:       CohereID,
		Name:     CohereDisplayName,
		URL:      CohereDefaultBaseURL,
		AuthType: AuthTypeBearer,
	},
	GoogleID: {
		ID:       GoogleID,
		Name:     GoogleDisplayName,
		URL:      GoogleDefaultBaseURL,
		AuthType: AuthTypeQuery,
	},
	GroqID: {
		ID:       GroqID,
		Name:     GroqDisplayName,
		URL:      GroqDefaultBaseURL,
		AuthType: AuthTypeBearer,
	},
	OllamaID: {
		ID:       OllamaID,
		Name:     OllamaDisplayName,
		URL:      OllamaDefaultBaseURL,
		AuthType: AuthTypeNone,
	},
	OpenaiID: {
		ID:       OpenaiID,
		Name:     OpenaiDisplayName,
		URL:      OpenaiDefaultBaseURL,
		AuthType: AuthTypeBearer,
	},
}
