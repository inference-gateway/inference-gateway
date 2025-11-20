package providers

type ListModelsResponseOllamaCloud struct {
	Object string  `json:"object"`
	Data   []Model `json:"data"`
}

func (l *ListModelsResponseOllamaCloud) Transform() ListModelsResponse {
	provider := OllamaCloudID
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
