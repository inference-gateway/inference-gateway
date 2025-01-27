package providers

type ListModelsResponseCohere struct {
	Models []interface{} `json:"models"`
}

func (l *ListModelsResponseCohere) Transform() ListModelsResponse {
	var models []map[string]interface{}
	for _, model := range l.Models {
		models = append(models, map[string]interface{}{
			"name": model,
			"id":   CohereID,
		})
	}
	return ListModelsResponse{
		Provider: CohereDisplayName,
		Models:   models,
	}
}

type GenerateRequestCohere struct {
	Messages []Message `json:"messages"`
	Model    string    `json:"model"`
}

type GenerateResponseCohere struct {
	Message struct{} `json:"message"`
}
