package providers

type CohereModel struct {
	Name             string   `json:"name"`
	Endpoints        []string `json:"endpoints"`
	Finetuned        bool     `json:"finetuned"`
	ContextLength    float64  `json:"context_length"`
	TokenizerURL     string   `json:"tokenizer_url"`
	DefaultEndpoints []string `json:"default_endpoints"`
}

type ListModelsResponseCohere struct {
	Models        []CohereModel `json:"models"`
	NextPageToken string        `json:"next_page_token"`
}

func (l *ListModelsResponseCohere) Transform() ListModelsResponse {
	var models []Model
	for _, model := range l.Models {
		models = append(models, Model{
			Name: model.Name,
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
