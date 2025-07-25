package providers

import (
	"time"
)

type ModelCloudflare struct {
	ID          string `json:"id,omitempty"`
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
	CreatedAt   string `json:"created_at,omitempty"`
	ModifiedAt  string `json:"modified_at,omitempty"`
	Public      int8   `json:"public,omitempty"`
	Model       string `json:"model,omitempty"`
}

type ListModelsResponseCloudflare struct {
	Success bool               `json:"success,omitempty"`
	Result  []*ModelCloudflare `json:"result,omitempty"`
}

func (l *ListModelsResponseCloudflare) Transform() ListModelsResponse {
	provider := CloudflareID
	models := make([]Model, len(l.Result))
	for i, model := range l.Result {
		created := time.Now().Unix()
		if model.CreatedAt != "" {
			createdAt, err := time.Parse("2006-01-02 15:04:05.999", model.CreatedAt)
			if err == nil {
				created = createdAt.Unix()
			}
		}

		models[i] = Model{
			ID:       string(provider) + "/" + model.ID,
			Object:   "model",
			Created:  created,
			OwnedBy:  string(provider),
			ServedBy: provider,
		}
	}

	return ListModelsResponse{
		Provider: &provider,
		Object:   "list",
		Data:     models,
	}
}
