package providers

type CloudflareModel struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	CreatedAt   string `json:"created_at"`
	ModifiedAt  string `json:"modified_at"`
	Public      int    `json:"public"`
	Model       string `json:"model"`
}

type ListModelsResponseCloudflare struct {
	Result []CloudflareModel `json:"result"`
}

func (l *ListModelsResponseCloudflare) Transform() ListModelsResponse {
	var models []Model
	for _, model := range l.Result {
		models = append(models, Model{
			Name: model.Name,
		})
	}
	return ListModelsResponse{
		Provider: CloudflareDisplayName,
		Models:   models,
	}
}

type GenerateRequestCloudflare struct {
	Prompt string `json:"prompt"`
}

type GenerateResponseCloudflare struct {
	Result struct{} `json:"result"`
}
