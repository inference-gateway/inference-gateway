// Code generated from OpenAPI schema. DO NOT EDIT.
package transformers

import (
	constants "github.com/inference-gateway/inference-gateway/providers/constants"
	types "github.com/inference-gateway/inference-gateway/providers/types"
)

type ListModelsResponseGroq struct {
	Object string        `json:"object"`
	Data   []types.Model `json:"data"`
}

func (l *ListModelsResponseGroq) Transform() types.ListModelsResponse {
	provider := constants.GroqID
	models := make([]types.Model, len(l.Data))
	for i, model := range l.Data {
		model.ServedBy = provider
		model.ID = string(provider) + "/" + model.ID
		models[i] = model
	}

	return types.ListModelsResponse{
		Provider: &provider,
		Object:   l.Object,
		Data:     models,
	}
}
