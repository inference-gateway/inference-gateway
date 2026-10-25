// Code generated from OpenAPI schema. DO NOT EDIT.
package transformers

import (
	constants "github.com/inference-gateway/inference-gateway/providers/constants"
	types "github.com/inference-gateway/inference-gateway/providers/types"
)

type ListModelsResponseOllamaCloud struct {
	Object string        `json:"object"`
	Data   []listedModel `json:"data"`
}

func (l *ListModelsResponseOllamaCloud) Transform() types.ListModelsResponse {
	provider := constants.OllamaCloudID
	return types.ListModelsResponse{
		Provider: &provider,
		Object:   l.Object,
		Data:     toModels(provider, l.Data),
	}
}
