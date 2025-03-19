package providers

type ListModelsResponseOllama struct {
	Object string   `json:"object"`
	Data   []*Model `json:"data"`
}

func (l *ListModelsResponseOllama) Transform() ListModelsResponse {
	models := make([]*Model, len(l.Data))
	for i, model := range l.Data {
		model.ServedBy = OllamaID
		models[i] = model
	}

	return ListModelsResponse{
		Provider: OllamaID,
		Object:   l.Object,
		Data:     models,
	}
}
