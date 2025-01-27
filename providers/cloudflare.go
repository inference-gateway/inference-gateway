package providers

type ListModelsResponseCloudflare struct {
	Result []interface{} `json:"result"`
}

func (l *ListModelsResponseCloudflare) Transform() ListModelsResponse {
	var models []map[string]interface{}
	for _, model := range l.Result {
		models = append(models, map[string]interface{}{
			"name": model,
			"id":   CloudflareID,
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
