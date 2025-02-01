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

type OpenaiModel struct {
	ID         string             `json:"id"`
	Object     string             `json:"object"`
	Created    int64              `json:"created"`
	OwnedBy    string             `json:"owned_by"`
	Permission []OpenaiPermission `json:"permission"`
	Root       string             `json:"root"`
	Parent     string             `json:"parent"`
}

type ListModelsResponseOpenai struct {
	Object string        `json:"object"`
	Data   []OpenaiModel `json:"data"`
}

func (l *ListModelsResponseOpenai) Transform() ListModelsResponse {
	var models []Model
	for _, model := range l.Data {
		models = append(models, Model{
			Name: model.ID,
		})
	}
	return ListModelsResponse{
		Provider: OpenaiID,
		Models:   models,
	}
}

type GenerateRequestOpenai struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	Temperature float64   `json:"temperature,omitempty"`
}

func (r *GenerateRequest) TransformOpenai() GenerateRequestOpenai {
	return GenerateRequestOpenai{
		Messages:    r.Messages,
		Model:       r.Model,
		Temperature: 0.7, // Default temperature for now until I add a configuration for this
	}
}

type OpenaiUsageDetails struct {
	ReasoningTokens          int `json:"reasoning_tokens"`
	AcceptedPredictionTokens int `json:"accepted_prediction_tokens"`
	RejectedPredictionTokens int `json:"rejected_prediction_tokens"`
}

type OpenaiUsage struct {
	PromptTokens     int                `json:"prompt_tokens"`
	CompletionTokens int                `json:"completion_tokens"`
	TotalTokens      int                `json:"total_tokens"`
	TokensDetails    OpenaiUsageDetails `json:"completion_tokens_details"`
}

type OpenaiChoice struct {
	Message      Message     `json:"message"`
	LogProbs     interface{} `json:"logprobs"`
	FinishReason string      `json:"finish_reason"`
	Index        int         `json:"index"`
}

type GenerateResponseOpenai struct {
	ID      string         `json:"id"`
	Object  string         `json:"object"`
	Created int64          `json:"created"`
	Model   string         `json:"model"`
	Usage   OpenaiUsage    `json:"usage"`
	Choices []OpenaiChoice `json:"choices"`
}

func (g *GenerateResponseOpenai) Transform() GenerateResponse {
	if len(g.Choices) == 0 {
		return GenerateResponse{}
	}

	return GenerateResponse{
		Provider: OpenaiDisplayName,
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
