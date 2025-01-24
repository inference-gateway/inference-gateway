package providers

type GetModelsResponseOpenai struct {
	Models []interface{} `json:"models"`
}

type GenerateRequestOpenai struct {
	Messages []Message `json:"messages"`
	Model    string    `json:"model"`
}

type GenerateResponseOpenai struct {
	Choices []struct{} `json:"choices"`
	Model   string     `json:"model"`
}
