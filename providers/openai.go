package providers

type ListModelsResponseOpenai struct {
	Object string   `json:"object"`
	Data   []*Model `json:"data"`
}

func (l *ListModelsResponseOpenai) Transform() ListModelsResponse {
	models := make([]*Model, len(l.Data))
	for i, model := range l.Data {
		model.ServedBy = OpenaiID
		models[i] = model
	}

	return ListModelsResponse{
		Provider: OpenaiID,
		Object:   l.Object,
		Data:     l.Data,
	}
}
