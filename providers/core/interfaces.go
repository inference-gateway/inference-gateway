package core

import (
	"context"

	types "github.com/inference-gateway/inference-gateway/providers/types"
)

//go:generate mockgen -source=interfaces.go -destination=../../../tests/mocks/providers/management.go -package=providersmocks
type IProvider interface {
	// Getters
	GetID() *types.Provider
	GetName() string
	GetURL() string
	GetToken() string
	GetAuthType() string
	GetExtraHeaders() map[string][]string

	// Fetchers
	ListModels(ctx context.Context) (types.ListModelsResponse, error)
	ChatCompletions(ctx context.Context, clientReq types.CreateChatCompletionRequest) (types.CreateChatCompletionResponse, error)
	StreamChatCompletions(ctx context.Context, clientReq types.CreateChatCompletionRequest) (<-chan []byte, error)
	SupportsVision(ctx context.Context, model string) (bool, error)
}
