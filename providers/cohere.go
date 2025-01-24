package providers

type GetModelsResponseCohere struct {
	Models []interface{} `json:"models"`
}

type GenerateRequestCohere struct {
	Messages []Message `json:"messages"`
	Model    string    `json:"model"`
}

type GenerateResponseCohere struct {
	Message struct{} `json:"message"`
}
