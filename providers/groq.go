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
	Messages            []Message `json:"messages"`
	Model               string    `json:"model"`
	Temperature         *float64  `json:"temperature,omitempty"`
	MaxCompletionTokens int       `json:"max_completion_tokens,omitempty"`
	TopP                *float64  `json:"top_p,omitempty"`
	FrequencyPenalty    *float64  `json:"frequency_penalty,omitempty"`
	PresencePenalty     *float64  `json:"presence_penalty,omitempty"`
	Stream              *bool     `json:"stream,omitempty"`
	Stop                []string  `json:"stop,omitempty"`
	User                *string   `json:"user,omitempty"`
	ResponseFormat      *struct {
		Type string `json:"type"`
	} `json:"response_format,omitempty"`
	Seed        *int    `json:"seed,omitempty"`
	ServiceTier *string `json:"service_tier,omitempty"`
	Tools       []Tool  `json:"tools,omitempty"`
}

func (r *GenerateRequest) TransformGroq() GenerateRequestGroq {
	return GenerateRequestGroq{
		Messages:            r.Messages,
		Model:               r.Model,
		Stream:              &r.Stream,
		Temperature:         Float64Ptr(1.0),
		Tools:               r.Tools,
		MaxCompletionTokens: r.MaxTokens,
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

type GroqDelta struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type GroqChoice struct {
	Index        int         `json:"index"`
	Message      Message     `json:"message"`
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
	Usage             *GroqUsage   `json:"usage,omitempty"`
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

	resp := GenerateResponse{
		Provider: GroqDisplayName,
		Response: response,
	}

	if g.Usage != nil {
		resp.Usage = &Usage{
			QueueTime:        g.Usage.QueueTime,
			PromptTokens:     g.Usage.PromptTokens,
			PromptTime:       g.Usage.PromptTime,
			CompletionTokens: g.Usage.CompletionTokens,
			CompletionTime:   g.Usage.CompletionTime,
			TotalTokens:      g.Usage.TotalTokens,
			TotalTime:        g.Usage.TotalTime,
		}
	}

	choice := g.Choices[0]
	switch {
	case len(choice.Message.ToolCalls) > 0:
		resp.Response.Content = choice.Message.Reasoning
		resp.Response.ToolCalls = choice.Message.ToolCalls
		return resp

	case choice.Message.Content != "":
		resp.Response.Content = choice.Message.Content
		resp.Response.Role = choice.Message.Role
		return resp

	case choice.Delta.Role == MessageRoleAssistant && choice.Delta.Content == "":
		resp.EventType = EventMessageStart
		return resp

	case choice.Delta.Content != "":
		resp.Response.Content = choice.Delta.Content
		resp.EventType = EventContentDelta
		return resp

	case choice.FinishReason == "stop":
		resp.EventType = EventStreamEnd
		return resp
	}

	resp.EventType = EventContentDelta
	return resp
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
