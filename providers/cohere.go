package providers

type CohereModel struct {
	Name             string   `json:"name"`
	Endpoints        []string `json:"endpoints"`
	Finetuned        bool     `json:"finetuned"`
	ContextLength    float64  `json:"context_length"`
	TokenizerURL     string   `json:"tokenizer_url"`
	DefaultEndpoints []string `json:"default_endpoints"`
}

type ListModelsResponseCohere struct {
	Models        []CohereModel `json:"models"`
	NextPageToken string        `json:"next_page_token"`
}

func (l *ListModelsResponseCohere) Transform() ListModelsResponse {
	var models []Model
	for _, model := range l.Models {
		models = append(models, Model{
			Name: model.Name,
		})
	}
	return ListModelsResponse{
		Provider: CohereDisplayName,
		Models:   models,
	}
}

type CohereResponseFormat struct {
	Type       string                 `json:"type,omitempty"`
	JsonSchema map[string]interface{} `json:"json_schema,omitempty"`
}

type CohereCitationOptions struct {
	Content interface{} `json:"content,omitempty"` // Could be further defined based on needs
}

type CohereDocument struct {
	Content  string                 `json:"content,omitempty"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

type CohereTool struct {
	Description string                 `json:"description,omitempty"`
	Name        string                 `json:"name,omitempty"`
	Parameters  map[string]interface{} `json:"parameters,omitempty"`
}

type GenerateRequestCohere struct {
	Messages         []Message              `json:"messages"`
	Model            string                 `json:"model"`
	Stream           bool                   `json:"stream"`
	Tools            []CohereTool           `json:"tools,omitempty"`
	Documents        []CohereDocument       `json:"documents,omitempty"`
	CitationOptions  *CohereCitationOptions `json:"citation_options,omitempty"`
	ResponseFormat   *CohereResponseFormat  `json:"response_format,omitempty"`
	SafetyMode       string                 `json:"safety_mode,omitempty"`
	MaxTokens        *int                   `json:"max_tokens,omitempty"`
	StopSequences    []string               `json:"stop_sequences,omitempty"`
	Temperature      *float64               `json:"temperature,omitempty"`
	Seed             *int                   `json:"seed,omitempty"`
	FrequencyPenalty *float64               `json:"frequency_penalty,omitempty"`
	PresencePenalty  *float64               `json:"presence_penalty,omitempty"`
	K                *float64               `json:"k,omitempty"`
	P                *float64               `json:"p,omitempty"`
	LogProbs         *bool                  `json:"logprobs,omitempty"`
	ToolChoice       string                 `json:"tool_choice,omitempty"`
	StrictTools      *bool                  `json:"strict_tools,omitempty"`
}

func (r *GenerateRequest) TransformCohere() GenerateRequestCohere {
	return GenerateRequestCohere{
		Messages:    r.Messages,
		Model:       r.Model,
		Stream:      false,           // Default to non-streaming
		Temperature: float64Ptr(0.3), // Default temperature as per docs
	}
}

type CohereContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type CohereMessage struct {
	Role    string          `json:"role"`
	Content []CohereContent `json:"content"`
}

type CohereUsageUnits struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

type CohereUsage struct {
	BilledUnits CohereUsageUnits `json:"billed_units"`
	Tokens      CohereUsageUnits `json:"tokens"`
}

type GenerateResponseCohere struct {
	ID           string        `json:"id"`
	FinishReason string        `json:"finish_reason"`
	Message      CohereMessage `json:"message"`
	Usage        CohereUsage   `json:"usage,omitempty"`
	LogProbs     []interface{} `json:"logprobs,omitempty"`
}

func (g *GenerateResponseCohere) Transform() GenerateResponse {
	if len(g.Message.Content) == 0 {
		return GenerateResponse{}
	}

	return GenerateResponse{
		Provider: CohereDisplayName,
		Response: ResponseTokens{
			Role:    g.Message.Role,
			Content: g.Message.Content[0].Text,
			Model:   "", // Cohere doesn't return model info in response
		},
	}
}
