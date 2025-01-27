package providers

// Extra headers for Anthropic provider
var AnthropicExtraHeaders = map[string][]string{
	"anthropic-version": {"2023-06-01"},
}

type AnthropicModel struct {
	Type        string `json:"type"`
	ID          string `json:"id"`
	DisplayName string `json:"display_name"`
	CreatedAt   string `json:"created_at"`
}

type ListModelsResponseAnthropic struct {
	Data    []AnthropicModel `json:"models"`
	HasMore bool             `json:"has_more"`
	FirstID string           `json:"first_id"`
	LastID  string           `json:"last_id"`
}

func (l *ListModelsResponseAnthropic) Transform() ListModelsResponse {
	var models []Model
	for _, model := range l.Data {
		models = append(models, Model{
			Name: model.ID,
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
