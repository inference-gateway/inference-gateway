// Code generated from OpenAPI schema. DO NOT EDIT.
package transformers

import (
	constants "github.com/inference-gateway/inference-gateway/providers/constants"
	types "github.com/inference-gateway/inference-gateway/providers/types"
)

type ListModelsResponseMinimax struct {
	Object string        `json:"object"`
	Data   []listedModel `json:"data"`
}

func (l *ListModelsResponseMinimax) Transform() types.ListModelsResponse {
	provider := constants.MinimaxID
	models := make([]types.Model, len(l.Data))
	for i, model := range l.Data {
		models[i] = model.toModel(provider)
	}

	return types.ListModelsResponse{
		Provider: &provider,
		Object:   l.Object,
		Data:     models,
	}
}
