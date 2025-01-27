package providers

type ListModelsResponseCloudflare struct {
	Result []interface{} `json:"result"`
}

func (l *ListModelsResponseCloudflare) Transform() ListModelsResponse {
	var models []Model
	for _, model := range l.Result {
		models = append(models, Model{
			Name: model.(string),
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
