package providers

type GroqModel struct {
	ID            string      `json:"id"`
	Object        string      `json:"object"`
	Created       int64       `json:"created"`
	OwnedBy       string      `json:"owned_by"`
	Active        bool        `json:"active"`
	ContextWindow int         `json:"context_window"`
	PublicApps    interface{} `json:"public_apps"`
}

type ListModelsResponseGroq struct {
	Object string      `json:"object"`
	Data   []GroqModel `json:"data"`
}

func (l *ListModelsResponseGroq) Transform() ListModelsResponse {
	var models []Model
	for _, model := range l.Data {
		models = append(models, Model{
			Name: model.ID,
		})
	}
	return ListModelsResponse{
		Provider: GroqDisplayName,
		Models:   models,
	}
}

type GenerateRequestGroq struct {
	Messages []Message `json:"messages"`
	Model    string    `json:"model"`
}

type GenerateResponseGroq struct {
	Choices []struct{} `json:"choices"`
	Model   string     `json:"model"`
}
