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
	ID           types.Provider
	Name         string
	URL          string
	Token        string
	AuthType     string
	ExtraHeaders map[string][]string
	Endpoints    types.Endpoints
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
		ID:           &provider.ID,
		Name:         provider.Name,
		URL:          provider.URL,
		Token:        provider.Token,
		AuthType:     provider.AuthType,
		ExtraHeaders: provider.ExtraHeaders,
		Endpoints:    provider.Endpoints,
		Logger:       p.logger,
		Client:       c,
	}, nil
}

// ptr is used by the generated registry_data.go to build optional endpoints.
func ptr[T any](v T) *T {
	return &v
}
