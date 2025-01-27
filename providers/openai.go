package providers

type ListModelsResponseOpenai struct {
	Models []interface{} `json:"models"`
}

func (l *ListModelsResponseOpenai) Transform() ListModelsResponse {
	var models []Model
	for _, model := range l.Models {
		models = append(models, Model{
			Name: model.(string),
		})
	}
	return ListModelsResponse{
		Provider: OllamaDisplayName,
		Models:   models,
	}
}

type GenerateRequestOpenai struct {
	Messages []Message `json:"messages"`
	Model    string    `json:"model"`
}

type GenerateResponseOpenai struct {
	Choices []struct{} `json:"choices"`
	Model   string     `json:"model"`
}
