// Code generated from OpenAPI schema. DO NOT EDIT.
package transformers

import (
	constants "github.com/inference-gateway/inference-gateway/providers/constants"
	types "github.com/inference-gateway/inference-gateway/providers/types"
)

type ListModelsResponseCohere struct {
	modelListing
}

func (l *ListModelsResponseCohere) Transform() types.ListModelsResponse {
	provider := constants.CohereID
	return types.ListModelsResponse{
		Provider: &provider,
		Object:   l.Object,
		Data:     l.models(provider),
	}
}
