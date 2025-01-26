package providers

type GetModelsResponseOllama struct {
	Models []interface{} `json:"models"`
}

type GenerateRequestOllama struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
	Stream bool   `json:"stream"`
	System string `json:"system"`
}

type GenerateResponseOllama struct {
	Provider string   `json:"provider"`
	Response struct{} `json:"response"`
}
