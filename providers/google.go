package providers

type ListModelsResponseGoogle struct {
	Object string  `json:"object"`
	Data   []Model `json:"data"`
}

func (l *ListModelsResponseGoogle) Transform() ListModelsResponse {
	provider := GoogleID
	models := make([]Model, len(l.Data))
	for i, model := range l.Data {
		model.ServedBy = provider
		model.ID = string(provider) + "/" + model.ID
		models[i] = model
	}

	return ListModelsResponse{
		Provider: &provider,
		Object:   l.Object,
		Data:     models,
	}
}
