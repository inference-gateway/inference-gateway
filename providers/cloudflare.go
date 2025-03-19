package providers

import "time"

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
	Success bool              `json:"success,omitempty"`
	Result  []ModelCloudflare `json:"result,omitempty"`
}

func (l *ListModelsResponseCloudflare) Transform() ListModelsResponse {
	models := make([]*Model, len(l.Result))
	for i, model := range l.Result {
		models[i].ID = model.Name
		models[i].Object = "model"
		if model.CreatedAt != "" {
			createdAt, err := time.Parse("2006-01-02 15:04:05.999", model.CreatedAt)
			if err == nil {
				models[i].Created = createdAt.Unix()
			}
		}
		models[i].OwnedBy = CloudflareID
		models[i].ServedBy = CloudflareID
	}

	return ListModelsResponse{
		Provider: CloudflareID,
		Object:   "list",
		Data:     models,
	}
}
