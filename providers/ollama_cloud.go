package providers

type ListModelsResponseOllama_cloud struct {
	Object string  `json:"object"`
	Data   []Model `json:"data"`
}

func (l *ListModelsResponseOllama_cloud) Transform() ListModelsResponse {
	provider := Ollama_cloudID
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
