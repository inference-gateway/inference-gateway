package providers

type GetModelsResponseCloudflare struct {
	Result []interface{} `json:"result"`
}

type GenerateRequestCloudflare struct {
	Prompt string `json:"prompt"`
}

type GenerateResponseCloudflare struct {
	Result struct{} `json:"result"`
}
