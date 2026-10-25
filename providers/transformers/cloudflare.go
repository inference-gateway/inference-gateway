// Code generated from OpenAPI schema. DO NOT EDIT.
package transformers

import (
	constants "github.com/inference-gateway/inference-gateway/providers/constants"
	types "github.com/inference-gateway/inference-gateway/providers/types"
)

type ListModelsResponseCloudflare struct {
	Object string        `json:"object"`
	Data   []listedModel `json:"data"`
}

func (l *ListModelsResponseCloudflare) Transform() types.ListModelsResponse {
	provider := constants.CloudflareID
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
