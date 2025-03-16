package providers

import (
	"bufio"
	"time"

	"github.com/inference-gateway/inference-gateway/logger"
)

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
	Data    []AnthropicModel `json:"data"`
	HasMore bool             `json:"has_more"`
	FirstID string           `json:"first_id"`
	LastID  string           `json:"last_id"`
}

func (l *ListModelsResponseAnthropic) Transform() ListModelsResponse {
	var models []Model
	for _, model := range l.Data {
		t, err := time.Parse(time.RFC3339, model.CreatedAt)
		var created int64
		if err != nil {
			created = 0
		} else {
			created = t.Unix()
		}

		models = append(models, Model{
			ID:       model.ID,
			Object:   "model",
			Created:  created,
			OwnedBy:  AnthropicID,
			ServedBy: AnthropicID,
		})
	}
	return ListModelsResponse{
		Object:   "list",
		Provider: AnthropicID,
		Data:     models,
	}
}

type GenerateRequestAnthropic struct {
	Model     string    `json:"model"`
	MaxTokens *int      `json:"max_tokens,omitempty"`
	Messages  []Message `json:"messages"`
}

func (r *CreateChatCompletionRequest) TransformAnthropic() CreateChatCompletionRequest {
	return *r
}

type AnthropicContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type AnthropicUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

type GenerateResponseAnthropic struct {
	ID           string             `json:"id"`
	Type         string             `json:"type"`
	Role         string             `json:"role"`
	Content      []AnthropicContent `json:"content"`
	Model        string             `json:"model"`
	StopReason   string             `json:"stop_reason"`
	StopSequence interface{}        `json:"stop_sequence"`
	Usage        AnthropicUsage     `json:"usage"`
}

func (g *GenerateResponseAnthropic) Transform() GenerateResponse {
	if len(g.Content) == 0 {
		return GenerateResponse{}
	}

	return GenerateResponse{
		Provider: AnthropicDisplayName,
		Response: ResponseTokens{
			Role:    MessageRole(g.Role),
			Content: g.Content[0].Text,
			Model:   g.Model,
		},
	}
}

type AnthropicStreamParser struct {
	logger logger.Logger
}

func (p *AnthropicStreamParser) ParseChunk(reader *bufio.Reader) (*SSEvent, error) {
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
