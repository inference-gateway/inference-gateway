package providers

type GenerateRequestOllama struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
	Stream bool   `json:"stream"`
}

type GenerateResponseOllama struct {
	Response string `json:"response"`
}
