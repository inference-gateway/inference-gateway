package providers

import (
	"fmt"

	"github.com/inference-gateway/inference-gateway/logger"
)

const (
	// Ollama endpoints
	OllamaListEndpoint     = "/api/tags"
	OllamaGenerateEndpoint = "/api/chat"

	// OpenAI endpoints
	OpenAIListEndpoint     = "/v1/models"
	OpenAIGenerateEndpoint = "/v1/chat/completions"

	// Groq endpoints
	GroqListEndpoint     = "/openai/v1/models"
	GroqGenerateEndpoint = "/openai/v1/chat/completions"

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

//go:generate mockgen -source=registry.go -destination=../tests/mocks/provider_registry.go -package=mocks
type ProviderRegistry interface {
	GetProviders() map[string]*Config
	BuildProvider(providerID string, client Client) (Provider, error)
}

type ProviderRegistryImpl struct {
	cfg    map[string]*Config
	logger logger.Logger
}

func NewProviderRegistry(cfg map[string]*Config, logger logger.Logger) ProviderRegistry {
	return &ProviderRegistryImpl{
		cfg:    cfg,
		logger: logger,
	}
}

func (p *ProviderRegistryImpl) GetProviders() map[string]*Config {
	return p.cfg
}

func (p *ProviderRegistryImpl) BuildProvider(providerID string, client Client) (Provider, error) {
	provider, ok := p.cfg[providerID]
	if !ok {
		return nil, fmt.Errorf("provider %s not found", providerID)
	}

	if provider.AuthType != AuthTypeNone && provider.Token == "" {
		return nil, fmt.Errorf("provider %s token not configured", providerID)
	}

	return &ProviderImpl{
		id:           provider.ID,
		name:         provider.Name,
		url:          provider.URL,
		token:        provider.Token,
		authType:     provider.AuthType,
		extraHeaders: provider.ExtraHeaders,
		endpoints:    provider.Endpoints,
		logger:       p.logger,
		client:       client,
	}, nil
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
