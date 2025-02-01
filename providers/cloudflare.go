package providers

import (
	"bufio"

	"github.com/inference-gateway/inference-gateway/logger"
)

type CloudflareModel struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	CreatedAt   string `json:"created_at"`
	ModifiedAt  string `json:"modified_at"`
	Public      int    `json:"public"`
	Model       string `json:"model"`
}

type ListModelsResponseCloudflare struct {
	Result []CloudflareModel `json:"result"`
}

func (l *ListModelsResponseCloudflare) Transform() ListModelsResponse {
	var models []Model
	for _, model := range l.Result {
		models = append(models, Model{
			Name: model.Name,
		})
	}
	return ListModelsResponse{
		Provider: CloudflareID,
		Models:   models,
	}
}

type GenerateRequestCloudflare struct {
	Model             string    `json:"model"`
	Messages          []Message `json:"messages"`
	FrequencyPenalty  *float64  `json:"frequency_penalty,omitempty"`
	MaxTokens         *int      `json:"max_tokens,omitempty"`
	PresencePenalty   *float64  `json:"presence_penalty,omitempty"`
	RepetitionPenalty *float64  `json:"repetition_penalty,omitempty"`
	Seed              *int      `json:"seed,omitempty"`
	Stream            *bool     `json:"stream,omitempty"`
	Temperature       *float64  `json:"temperature,omitempty"`
	TopK              *int      `json:"top_k,omitempty"`
	TopP              *float64  `json:"top_p,omitempty"`
	Functions         []struct {
		Code string `json:"code"`
		Name string `json:"name"`
	} `json:"functions,omitempty"`
	Tools []struct {
		Description string                 `json:"description,omitempty"`
		Name        string                 `json:"name,omitempty"`
		Parameters  map[string]interface{} `json:"parameters,omitempty"`
		Function    map[string]interface{} `json:"function,omitempty"`
		Type        string                 `json:"type,omitempty"`
	} `json:"tools,omitempty"`
}

func (r *GenerateRequest) TransformCloudflare() GenerateRequestCloudflare {
	return GenerateRequestCloudflare{
		Messages:    r.Messages,
		Model:       r.Model,
		Stream:      &r.Stream,
		Temperature: float64Ptr(0.7),
	}
}

type CloudflareResult struct {
	Response string `json:"response"`
}

type GenerateResponseCloudflare struct {
	Result   CloudflareResult `json:"result"`
	Success  bool             `json:"success"`
	Errors   []string         `json:"errors"`
	Messages []string         `json:"messages"`
}

func (g *GenerateResponseCloudflare) Transform() GenerateResponse {
	return GenerateResponse{
		Provider: CloudflareDisplayName,
		Response: ResponseTokens{
			Role:    MessageRoleAssistant,
			Content: g.Result.Response,
			Model:   "", // Cloudflare doesn't return model info in response
		},
	}
}

type CloudflareStreamParser struct {
	logger logger.Logger
}

func (p *CloudflareStreamParser) ParseChunk(reader *bufio.Reader) (*SSEvent, error) {
	rawchunk, err := readSSEventsChunk(reader)
	if err != nil {
		return nil, err
	}

	event, err := ParseSSEvents(rawchunk)
	if err != nil {
		return nil, err
	}

	return event, nil
}
