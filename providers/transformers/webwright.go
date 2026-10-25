// Code generated from OpenAPI schema. DO NOT EDIT.
package transformers

import (
	constants "github.com/inference-gateway/inference-gateway/providers/constants"
	types "github.com/inference-gateway/inference-gateway/providers/types"
)

type ListModelsResponseWebwright struct {
	Object string        `json:"object"`
	Data   []listedModel `json:"data"`
}

func (l *ListModelsResponseWebwright) Transform() types.ListModelsResponse {
	provider := constants.WebwrightID
	return types.ListModelsResponse{
		Provider: &provider,
		Object:   l.Object,
		Data:     toModels(provider, l.Data),
	}
}
