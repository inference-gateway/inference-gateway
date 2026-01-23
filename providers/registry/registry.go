// Code generated from OpenAPI schema. DO NOT EDIT.
package registry

import (
	"fmt"

	logger "github.com/inference-gateway/inference-gateway/logger"
	client "github.com/inference-gateway/inference-gateway/providers/client"
	constants "github.com/inference-gateway/inference-gateway/providers/constants"
	core "github.com/inference-gateway/inference-gateway/providers/core"
	types "github.com/inference-gateway/inference-gateway/providers/types"
)

// Base provider configuration
type ProviderConfig struct {
	ID             types.Provider
	Name           string
	URL            string
	Token          string
	AuthType       string
	SupportsVision bool
	ExtraHeaders   map[string][]string
	Endpoints      types.Endpoints
}

//go:generate mockgen -source=registry.go -destination=../../tests/mocks/providers/registry.go -package=providersmocks
type ProviderRegistry interface {
	GetProviders() map[types.Provider]*ProviderConfig
	BuildProvider(providerID types.Provider, c client.Client) (core.IProvider, error)
}

type ProviderRegistryImpl struct {
	cfg    map[types.Provider]*ProviderConfig
	logger logger.Logger
}

func NewProviderRegistry(cfg map[types.Provider]*ProviderConfig, logger logger.Logger) ProviderRegistry {
	return &ProviderRegistryImpl{
		cfg:    cfg,
		logger: logger,
	}
}

func (p *ProviderRegistryImpl) GetProviders() map[types.Provider]*ProviderConfig {
	return p.cfg
}

func (p *ProviderRegistryImpl) BuildProvider(providerID types.Provider, c client.Client) (core.IProvider, error) {
	provider, ok := p.cfg[providerID]
	if !ok {
		return nil, fmt.Errorf("provider %s not found", providerID)
	}

	if provider.AuthType != constants.AuthTypeNone && provider.Token == "" {
		return nil, fmt.Errorf("provider %s token not configured", providerID)
	}

	return &core.ProviderImpl{
		ID:                 &provider.ID,
		Name:               provider.Name,
		URL:                provider.URL,
		Token:              provider.Token,
		AuthType:           provider.AuthType,
		SupportsVisionFlag: provider.SupportsVision,
		ExtraHeaders:       provider.ExtraHeaders,
		Endpoints:          provider.Endpoints,
		Logger:             p.logger,
		Client:             c,
	}, nil
}

// The registry of all providers
var Registry = map[types.Provider]*ProviderConfig{
	constants.AnthropicID: {
		ID:             constants.AnthropicID,
		Name:           constants.AnthropicDisplayName,
		URL:            constants.AnthropicDefaultBaseURL,
		AuthType:       constants.AuthTypeXheader,
		SupportsVision: true,
		ExtraHeaders: map[string][]string{
			"anthropic-version": {"2023-06-01"},
		},
		Endpoints: types.Endpoints{
			Models: constants.AnthropicModelsEndpoint,
			Chat:   constants.AnthropicChatEndpoint,
		},
	},
	constants.CloudflareID: {
		ID:             constants.CloudflareID,
		Name:           constants.CloudflareDisplayName,
		URL:            constants.CloudflareDefaultBaseURL,
		AuthType:       constants.AuthTypeBearer,
		SupportsVision: false,
		Endpoints: types.Endpoints{
			Models: constants.CloudflareModelsEndpoint,
			Chat:   constants.CloudflareChatEndpoint,
		},
	},
	constants.CohereID: {
		ID:             constants.CohereID,
		Name:           constants.CohereDisplayName,
		URL:            constants.CohereDefaultBaseURL,
		AuthType:       constants.AuthTypeBearer,
		SupportsVision: true,
		Endpoints: types.Endpoints{
			Models: constants.CohereModelsEndpoint,
			Chat:   constants.CohereChatEndpoint,
		},
	},
	constants.DeepseekID: {
		ID:             constants.DeepseekID,
		Name:           constants.DeepseekDisplayName,
		URL:            constants.DeepseekDefaultBaseURL,
		AuthType:       constants.AuthTypeBearer,
		SupportsVision: false,
		Endpoints: types.Endpoints{
			Models: constants.DeepseekModelsEndpoint,
			Chat:   constants.DeepseekChatEndpoint,
		},
	},
	constants.GoogleID: {
		ID:             constants.GoogleID,
		Name:           constants.GoogleDisplayName,
		URL:            constants.GoogleDefaultBaseURL,
		AuthType:       constants.AuthTypeBearer,
		SupportsVision: true,
		Endpoints: types.Endpoints{
			Models: constants.GoogleModelsEndpoint,
			Chat:   constants.GoogleChatEndpoint,
		},
	},
	constants.GroqID: {
		ID:             constants.GroqID,
		Name:           constants.GroqDisplayName,
		URL:            constants.GroqDefaultBaseURL,
		AuthType:       constants.AuthTypeBearer,
		SupportsVision: true,
		Endpoints: types.Endpoints{
			Models: constants.GroqModelsEndpoint,
			Chat:   constants.GroqChatEndpoint,
		},
	},
	constants.MistralID: {
		ID:             constants.MistralID,
		Name:           constants.MistralDisplayName,
		URL:            constants.MistralDefaultBaseURL,
		AuthType:       constants.AuthTypeBearer,
		SupportsVision: true,
		Endpoints: types.Endpoints{
			Models: constants.MistralModelsEndpoint,
			Chat:   constants.MistralChatEndpoint,
		},
	},
	constants.MoonshotID: {
		ID:             constants.MoonshotID,
		Name:           constants.MoonshotDisplayName,
		URL:            constants.MoonshotDefaultBaseURL,
		AuthType:       constants.AuthTypeBearer,
		SupportsVision: false,
		Endpoints: types.Endpoints{
			Models: constants.MoonshotModelsEndpoint,
			Chat:   constants.MoonshotChatEndpoint,
		},
	},
	constants.OllamaID: {
		ID:             constants.OllamaID,
		Name:           constants.OllamaDisplayName,
		URL:            constants.OllamaDefaultBaseURL,
		AuthType:       constants.AuthTypeNone,
		SupportsVision: true,
		Endpoints: types.Endpoints{
			Models: constants.OllamaModelsEndpoint,
			Chat:   constants.OllamaChatEndpoint,
		},
	},
	constants.OllamaCloudID: {
		ID:             constants.OllamaCloudID,
		Name:           constants.OllamaCloudDisplayName,
		URL:            constants.OllamaCloudDefaultBaseURL,
		AuthType:       constants.AuthTypeBearer,
		SupportsVision: true,
		Endpoints: types.Endpoints{
			Models: constants.OllamaCloudModelsEndpoint,
			Chat:   constants.OllamaCloudChatEndpoint,
		},
	},
	constants.OpenaiID: {
		ID:             constants.OpenaiID,
		Name:           constants.OpenaiDisplayName,
		URL:            constants.OpenaiDefaultBaseURL,
		AuthType:       constants.AuthTypeBearer,
		SupportsVision: true,
		Endpoints: types.Endpoints{
			Models: constants.OpenaiModelsEndpoint,
			Chat:   constants.OpenaiChatEndpoint,
		},
	},
}
