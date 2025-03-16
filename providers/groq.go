package providers

import (
	"bufio"

	"github.com/inference-gateway/inference-gateway/logger"
)

type GroqModel struct {
	ID            string      `json:"id"`
	Object        string      `json:"object"`
	Created       int64       `json:"created"`
	OwnedBy       string      `json:"owned_by"`
	Active        bool        `json:"active"`
	ContextWindow int         `json:"context_window"`
	PublicApps    interface{} `json:"public_apps"`
}

type ListModelsResponseGroq struct {
	Object string      `json:"object"`
	Data   []GroqModel `json:"data"`
}

func (l *ListModelsResponseGroq) Transform() ListModelsResponse {
	var models []Model
	for _, model := range l.Data {
		models = append(models, Model{
			ID:       model.ID,
			Object:   model.Object,
			Created:  model.Created,
			OwnedBy:  model.OwnedBy,
			ServedBy: GroqID,
		})
	}
	return ListModelsResponse{
		Object:   l.Object,
		Provider: GroqID,
		Data:     models,
	}
}

func (r *CreateChatCompletionRequest) TransformGroq() CreateChatCompletionRequest {
	return *r
}

func NewGroqStreamParser(logger logger.Logger) *GroqStreamParser {
	return &GroqStreamParser{
		logger: logger,
	}
}

type GroqStreamParser struct {
	logger logger.Logger
}

func (p *GroqStreamParser) ParseChunk(reader *bufio.Reader) (*SSEvent, error) {
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
