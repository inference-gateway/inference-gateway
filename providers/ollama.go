package providers

type ListModelsResponseOllama struct {
	Models []Model `json:"models"`
}

func (l *ListModelsResponseOllama) Transform() ListModelsResponse {
	var models []Model
	for _, model := range l.Models {
		models = append(models, Model{
			Name: model.Name,
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
