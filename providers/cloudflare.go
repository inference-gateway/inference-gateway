package providers

type GenerateRequestCloudflare struct {
	Prompt string `json:"prompt"`
}

type GetModelsResponseCloudflare struct {
	Result []interface{} `json:"result"`
}

type GenerateResponseCloudflareResult struct {
	Response string `json:"response"`
}

type GenerateResponseCloudflare struct {
	Result GenerateResponseCloudflareResult `json:"result"`
}
