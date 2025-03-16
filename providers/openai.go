package providers

import (
	"bufio"

	"github.com/inference-gateway/inference-gateway/logger"
)

type OpenaiPermission struct {
	ID                 string `json:"id"`
	Object             string `json:"object"`
	Created            int64  `json:"created"`
	AllowCreateEngine  bool   `json:"allow_create_engine"`
	AllowSampling      bool   `json:"allow_sampling"`
	AllowLogprobs      bool   `json:"allow_logprobs"`
	AllowSearchIndices bool   `json:"allow_search_indices"`
	AllowView          bool   `json:"allow_view"`
	AllowFineTuning    bool   `json:"allow_fine_tuning"`
}

func (l *ListModelsResponse) Transform() ListModelsResponse {
	return ListModelsResponse{
		Provider: OpenaiID,
		Object:   l.Object,
		Data:     l.Data,
	}
}

type GenerateRequestOpenai struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	Temperature float64   `json:"temperature,omitempty"`
}

func (r *CreateChatCompletionRequest) TransformOpenai() CreateChatCompletionRequest {
	return *r
}

type OpenaiUsageDetails struct {
	ReasoningTokens          int `json:"reasoning_tokens"`
	AcceptedPredictionTokens int `json:"accepted_prediction_tokens"`
	RejectedPredictionTokens int `json:"rejected_prediction_tokens"`
}

func (g *GenerateResponseOpenai) Transform() GenerateResponse {
	if len(g.Choices) == 0 {
		return GenerateResponse{}
	}

	return GenerateResponse{
		Provider: OpenaiID,
		Response: ResponseTokens{
			Role:    g.Choices[0].Message.Role,
			Model:   g.Model,
			Content: g.Choices[0].Message.Content,
		},
	}
}

type OpenaiStreamParser struct {
	logger logger.Logger
}

func (p *OpenaiStreamParser) ParseChunk(reader *bufio.Reader) (*SSEvent, error) {
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
