// Code generated from OpenAPI schema. DO NOT EDIT.
package transformers

import (
	constants "github.com/inference-gateway/inference-gateway/providers/constants"
	types "github.com/inference-gateway/inference-gateway/providers/types"
)

type ListModelsResponseZai struct {
	modelListing
}

func (l *ListModelsResponseZai) Transform() types.ListModelsResponse {
	provider := constants.ZaiID
	return types.ListModelsResponse{
		Provider: &provider,
		Object:   l.Object,
		Data:     l.models(provider),
	}
}
