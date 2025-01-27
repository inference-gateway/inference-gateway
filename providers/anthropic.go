package providers

// Extra headers for Anthropic provider
var AnthropicExtraHeaders = map[string][]string{
	"anthropic-version": {"2023-06-01"},
}

type ListModelsResponseAnthropic struct {
	Models []interface{} `json:"models"`
}

func (l *ListModelsResponseAnthropic) Transform() ListModelsResponse {
	var models []map[string]interface{}
	for _, model := range l.Models {
		models = append(models, map[string]interface{}{
			"name": model,
			"id":   AnthropicID,
		})
	}
	return ListModelsResponse{
		Provider: AnthropicDisplayName,
		Models:   models,
	}
}

type GenerateRequestAnthropic struct {
	Messages []Message `json:"messages"`
	Model    string    `json:"model"`
}

type GenerateResponseAnthropic struct {
	Choices []struct{} `json:"choices"`
	Model   string     `json:"model"`
}
