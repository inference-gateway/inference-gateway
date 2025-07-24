package providers

import "strings"

type ListModelsResponseDeepseek struct {
	Object string  `json:"object"`
	Data   []Model `json:"data"`
}

func (l *ListModelsResponseDeepseek) Transform() ListModelsResponse {
	provider := DeepseekID
	models := make([]Model, len(l.Data))
	for i, model := range l.Data {
		model.ServedBy = provider
		if !strings.Contains(model.ID, "/") {
			model.ID = string(provider) + "/" + model.ID
		}
		models[i] = model
	}

	return ListModelsResponse{
		Provider: &provider,
		Object:   l.Object,
		Data:     models,
	}
}
