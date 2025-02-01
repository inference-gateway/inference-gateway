package providers

type DeepseekModel struct {
	ID            string      `json:"id"`
	Object        string      `json:"object"`
	Created       int64       `json:"created"`
	OwnedBy       string      `json:"owned_by"`
	Active        bool        `json:"active"`
	ContextWindow int         `json:"context_window"`
	PublicApps    interface{} `json:"public_apps"`
}

type ListModelsResponseDeepseek struct {
	Object string          `json:"object"`
	Data   []DeepseekModel `json:"data"`
}

func (l *ListModelsResponseDeepseek) Transform() ListModelsResponse {
	var models []Model
	for _, model := range l.Data {
		models = append(models, Model{
			Name: model.ID,
		})
	}
	return ListModelsResponse{
		Provider: DeepseekID,
		Models:   models,
	}
}
