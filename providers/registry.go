// Code generated from OpenAPI schema. DO NOT EDIT.
package providers

import (
	"fmt"

	"github.com/inference-gateway/inference-gateway/logger"
)

// Base provider configuration
type Config struct {
	ID             Provider
	Name           string
	URL            string
	Token          string
	AuthType       string
	SupportsVision bool
	ExtraHeaders   map[string][]string
	Endpoints      Endpoints
}

//go:generate mockgen -source=registry.go -destination=../tests/mocks/providers/registry.go -package=providersmocks
type ProviderRegistry interface {
	GetProviders() map[Provider]*Config
	BuildProvider(providerID Provider, client Client) (IProvider, error)
}

type ProviderRegistryImpl struct {
	cfg    map[Provider]*Config
	logger logger.Logger
}

func NewProviderRegistry(cfg map[Provider]*Config, logger logger.Logger) ProviderRegistry {
	return &ProviderRegistryImpl{
		cfg:    cfg,
		logger: logger,
	}
}

func (p *ProviderRegistryImpl) GetProviders() map[Provider]*Config {
	return p.cfg
}

func (p *ProviderRegistryImpl) BuildProvider(providerID Provider, client Client) (IProvider, error) {
	provider, ok := p.cfg[providerID]
	if !ok {
		return nil, fmt.Errorf("provider %s not found", providerID)
	}

	if provider.AuthType != AuthTypeNone && provider.Token == "" {
		return nil, fmt.Errorf("provider %s token not configured", providerID)
	}

	return &ProviderImpl{
		id:             &provider.ID,
		name:           provider.Name,
		url:            provider.URL,
		token:          provider.Token,
		authType:       provider.AuthType,
		supportsVision: provider.SupportsVision,
		extraHeaders:   provider.ExtraHeaders,
		endpoints:      provider.Endpoints,
		logger:         p.logger,
		client:         client,
	}, nil
}

// The registry of all providers
var Registry = map[Provider]*Config{
	AnthropicID: {
		ID:             AnthropicID,
		Name:           AnthropicDisplayName,
		URL:            AnthropicDefaultBaseURL,
		AuthType:       AuthTypeXheader,
		SupportsVision: true,
		ExtraHeaders: map[string][]string{
			"anthropic-version": {"2023-06-01"},
		},
		Endpoints: Endpoints{
			Models: AnthropicModelsEndpoint,
			Chat:   AnthropicChatEndpoint,
		},
	},
	CloudflareID: {
		ID:             CloudflareID,
		Name:           CloudflareDisplayName,
		URL:            CloudflareDefaultBaseURL,
		AuthType:       AuthTypeBearer,
		SupportsVision: false,
		Endpoints: Endpoints{
			Models: CloudflareModelsEndpoint,
			Chat:   CloudflareChatEndpoint,
		},
	},
	CohereID: {
		ID:             CohereID,
		Name:           CohereDisplayName,
		URL:            CohereDefaultBaseURL,
		AuthType:       AuthTypeBearer,
		SupportsVision: true,
		Endpoints: Endpoints{
			Models: CohereModelsEndpoint,
			Chat:   CohereChatEndpoint,
		},
	},
	DeepseekID: {
		ID:             DeepseekID,
		Name:           DeepseekDisplayName,
		URL:            DeepseekDefaultBaseURL,
		AuthType:       AuthTypeBearer,
		SupportsVision: false,
		Endpoints: Endpoints{
			Models: DeepseekModelsEndpoint,
			Chat:   DeepseekChatEndpoint,
		},
	},
	GoogleID: {
		ID:             GoogleID,
		Name:           GoogleDisplayName,
		URL:            GoogleDefaultBaseURL,
		AuthType:       AuthTypeBearer,
		SupportsVision: true,
		Endpoints: Endpoints{
			Models: GoogleModelsEndpoint,
			Chat:   GoogleChatEndpoint,
		},
	},
	GroqID: {
		ID:             GroqID,
		Name:           GroqDisplayName,
		URL:            GroqDefaultBaseURL,
		AuthType:       AuthTypeBearer,
		SupportsVision: true,
		Endpoints: Endpoints{
			Models: GroqModelsEndpoint,
			Chat:   GroqChatEndpoint,
		},
	},
	MistralID: {
		ID:             MistralID,
		Name:           MistralDisplayName,
		URL:            MistralDefaultBaseURL,
		AuthType:       AuthTypeBearer,
		SupportsVision: true,
		Endpoints: Endpoints{
			Models: MistralModelsEndpoint,
			Chat:   MistralChatEndpoint,
		},
	},
	MoonshotID: {
		ID:             MoonshotID,
		Name:           MoonshotDisplayName,
		URL:            MoonshotDefaultBaseURL,
		AuthType:       AuthTypeBearer,
		SupportsVision: false,
		Endpoints: Endpoints{
			Models: MoonshotModelsEndpoint,
			Chat:   MoonshotChatEndpoint,
		},
	},
	OllamaID: {
		ID:             OllamaID,
		Name:           OllamaDisplayName,
		URL:            OllamaDefaultBaseURL,
		AuthType:       AuthTypeNone,
		SupportsVision: true,
		Endpoints: Endpoints{
			Models: OllamaModelsEndpoint,
			Chat:   OllamaChatEndpoint,
		},
	},
	OllamaCloudID: {
		ID:             OllamaCloudID,
		Name:           OllamaCloudDisplayName,
		URL:            OllamaCloudDefaultBaseURL,
		AuthType:       AuthTypeBearer,
		SupportsVision: true,
		Endpoints: Endpoints{
			Models: OllamaCloudModelsEndpoint,
			Chat:   OllamaCloudChatEndpoint,
		},
	},
	OpenaiID: {
		ID:             OpenaiID,
		Name:           OpenaiDisplayName,
		URL:            OpenaiDefaultBaseURL,
		AuthType:       AuthTypeBearer,
		SupportsVision: true,
		Endpoints: Endpoints{
			Models: OpenaiModelsEndpoint,
			Chat:   OpenaiChatEndpoint,
		},
	},
}
