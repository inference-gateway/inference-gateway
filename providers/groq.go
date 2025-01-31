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
			Name: model.ID,
		})
	}
	return ListModelsResponse{
		Provider: GroqID,
		Models:   models,
	}
}

type GenerateRequestGroq struct {
	Messages         []Message `json:"messages"`
	Model            string    `json:"model"`
	Temperature      *float64  `json:"temperature,omitempty"`
	MaxTokens        *int      `json:"max_tokens,omitempty"`
	TopP             *float64  `json:"top_p,omitempty"`
	FrequencyPenalty *float64  `json:"frequency_penalty,omitempty"`
	PresencePenalty  *float64  `json:"presence_penalty,omitempty"`
	Stream           *bool     `json:"stream,omitempty"`
	Stop             []string  `json:"stop,omitempty"`
	User             *string   `json:"user,omitempty"`
	ResponseFormat   *struct {
		Type string `json:"type"`
	} `json:"response_format,omitempty"`
	Seed        *int    `json:"seed,omitempty"`
	ServiceTier *string `json:"service_tier,omitempty"`
}

func (r *GenerateRequest) TransformGroq() GenerateRequestGroq {
	return GenerateRequestGroq{
		Messages:    r.Messages,
		Model:       r.Model,
		Stream:      &r.Stream,
		Temperature: float64Ptr(1.0),
	}
}

type GroqUsage struct {
	QueueTime        float64 `json:"queue_time"`
	PromptTokens     int     `json:"prompt_tokens"`
	PromptTime       float64 `json:"prompt_time"`
	CompletionTokens int     `json:"completion_tokens"`
	CompletionTime   float64 `json:"completion_time"`
	TotalTokens      int     `json:"total_tokens"`
	TotalTime        float64 `json:"total_time"`
}

type GroqMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type GroqDelta struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type GroqChoice struct {
	Index        int         `json:"index"`
	Message      GroqMessage `json:"message"`
	Delta        GroqDelta   `json:"delta,omitempty"`
	LogProbs     interface{} `json:"logprobs"`
	FinishReason string      `json:"finish_reason"`
}

type GenerateResponseGroq struct {
	ID                string       `json:"id"`
	Object            string       `json:"object"`
	Created           int64        `json:"created"`
	Model             string       `json:"model"`
	Choices           []GroqChoice `json:"choices"`
	Usage             GroqUsage    `json:"usage"`
	SystemFingerprint string       `json:"system_fingerprint"`
	XGroq             struct {
		ID string `json:"id"`
	} `json:"x_groq"`
}

func (g *GenerateResponseGroq) Transform() GenerateResponse {
	if len(g.Choices) == 0 {
		return GenerateResponse{}
	}

	response := ResponseTokens{
		Model: g.Model,
		Role:  MessageRoleAssistant,
	}

	// Handle non-streaming case using Message first
	if g.Choices[0].Message.Content != "" {
		response.Content = g.Choices[0].Message.Content
		response.Role = g.Choices[0].Message.Role
		return GenerateResponse{
			Provider:  GroqDisplayName,
			Response:  response,
			EventType: EventContentDelta,
		}
	}

	// Handle streaming cases with Delta field

	// Handle initial message with role
	if g.Choices[0].Delta.Role == MessageRoleAssistant && g.Choices[0].Delta.Content == "" {
		return GenerateResponse{
			Provider:  GroqDisplayName,
			Response:  response,
			EventType: EventMessageStart,
		}
	}

	// Handle content delta
	if g.Choices[0].Delta.Content != "" {
		response.Content = g.Choices[0].Delta.Content
		return GenerateResponse{
			Provider:  GroqDisplayName,
			Response:  response,
			EventType: EventContentDelta,
		}
	}

	// Handle stream end (empty delta with finish_reason "stop")
	if g.Choices[0].FinishReason == "stop" {
		return GenerateResponse{
			Provider:  GroqDisplayName,
			Response:  response,
			EventType: EventStreamEnd,
		}
	}

	return GenerateResponse{
		Provider:  GroqDisplayName,
		Response:  response,
		EventType: EventContentDelta,
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

	event, err := parseSSEvents(rawchunk)
	if err != nil {
		return nil, err
	}

	return event, nil
}
