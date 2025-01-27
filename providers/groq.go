package providers

type ListModelsResponseGroq struct {
	Models []interface{} `json:"models"`
}

type GenerateRequestGroq struct {
	Messages []Message `json:"messages"`
	Model    string    `json:"model"`
}

type GenerateResponseGroq struct {
	Choices []struct{} `json:"choices"`
	Model   string     `json:"model"`
}
