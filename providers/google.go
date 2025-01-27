package providers

type ListModelsResponseGoogle struct {
	Models []Model `json:"models"`
}

func (l *ListModelsResponseGoogle) Transform() ListModelsResponse {
	var models []map[string]interface{}
	for _, model := range l.Models {
		models = append(models, map[string]interface{}{
			"name": model.Name,
			"id":   OllamaID,
		})
	}
	return ListModelsResponse{
		Provider: OllamaDisplayName,
		Models:   models,
	}
}

type GenerateRequestGoogle struct {
	Contents struct{} `json:"contents"`
}

type GenerateResponseGoogle struct {
	Candidates []struct{} `json:"candidates"`
}
