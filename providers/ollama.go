package providers

type ListModelsResponseOllama struct {
	Models []Model `json:"models"`
}

func (l *ListModelsResponseOllama) Transform() ListModelsResponse {
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

type GenerateRequestOllama struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
	Stream bool   `json:"stream"`
	System string `json:"system"`
}

type GenerateResponseOllama struct {
	Provider string   `json:"provider"`
	Response struct{} `json:"response"`
}
