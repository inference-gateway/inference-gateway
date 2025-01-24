package providers

// Extra headers for Anthropic provider
var AnthropicExtraHeaders = map[string][]string{
	"anthropic-version": {"2023-06-01"},
}

type GetModelsResponseAnthropic struct {
	Models []interface{} `json:"models"`
}

type GenerateRequestAnthropic struct {
	Messages []Message `json:"messages"`
	Model    string    `json:"model"`
}

type GenerateResponseAnthropic struct {
	Choices []struct{} `json:"choices"`
	Model   string     `json:"model"`
}
