package providers

type ListModelsResponseGroq struct {
	Object string   `json:"object"`
	Data   []*Model `json:"data"`
}

func (l *ListModelsResponseGroq) Transform() ListModelsResponse {
	models := make([]*Model, len(l.Data))
	for i, model := range l.Data {
		model.ServedBy = GroqID
		models[i] = model
	}

	return ListModelsResponse{
		Provider: GroqID,
		Object:   l.Object,
		Data:     models,
	}
}
