package providers

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"

	"github.com/inference-gateway/inference-gateway/logger"
)

// The authentication type of the specific provider
const (
	AuthTypeBearer  = "bearer"
	AuthTypeXheader = "xheader"
	AuthTypeQuery   = "query"
	AuthTypeNone    = "none"
)

// The default base URLs of each provider
const (
	AnthropicDefaultBaseURL  = "https://api.anthropic.com"
	CloudflareDefaultBaseURL = "https://api.cloudflare.com/client/v4/accounts/{ACCOUNT_ID}"
	CohereDefaultBaseURL     = "https://api.cohere.com"
	GroqDefaultBaseURL       = "https://api.groq.com"
	OllamaDefaultBaseURL     = "http://ollama:8080"
	OpenaiDefaultBaseURL     = "https://api.openai.com"
)

// The ID's of each provider
const (
	AnthropicID  = "anthropic"
	CloudflareID = "cloudflare"
	CohereID     = "cohere"
	GroqID       = "groq"
	OllamaID     = "ollama"
	OpenaiID     = "openai"
)

// Display names for providers
const (
	AnthropicDisplayName  = "Anthropic"
	CloudflareDisplayName = "Cloudflare"
	CohereDisplayName     = "Cohere"
	GroqDisplayName       = "Groq"
	OllamaDisplayName     = "Ollama"
	OpenaiDisplayName     = "Openai"
)

const (
	MessageRoleSystem    = "system"
	MessageRoleUser      = "user"
	MessageRoleAssistant = "assistant"
	MessageRoleTool      = "tool"
)

// ToolProperty represents a parameter property
type ToolProperty struct {
	Type        string `json:"type"`
	Description string `json:"description"`
}

// ToolParams represents the parameters for a function tool
type ToolParams struct {
	Type       string                  `json:"type"`
	Properties map[string]ToolProperty `json:"properties"`
	Required   []string                `json:"required"`
}

// FunctionTool represents a function that can be called
type FunctionTool struct {
	Name        string     `json:"name"`
	Description string     `json:"description"`
	Parameters  ToolParams `json:"parameters"`
}

// Tool represents a function tool that can be called by the LLM
type Tool struct {
	Type     string        `json:"type"`
	Function *FunctionTool `json:"function,omitempty"`
}

// Common response and request types
type GenerateRequest struct {
	Messages  []Message `json:"messages"`
	Model     string    `json:"model"`
	Stream    bool      `json:"stream"`
	SSEvents  bool      `json:"ssevents"`
	Tools     []Tool    `json:"tools"`
	MaxTokens int       `json:"max_tokens,omitempty"`
}

// ToolCall represents a tool invocation by the LLM
type ToolCall struct {
	ID       string           `json:"id,omitempty"`
	Type     string           `json:"type,omitempty"`
	Function FunctionToolCall `json:"function"`
}

// FunctionToolCall represents a function call
type FunctionToolCall struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Arguments   json.RawMessage `json:"arguments"`
}

type GenerateResponse struct {
	Provider  string         `json:"provider"`
	Response  ResponseTokens `json:"response"`
	EventType EventType      `json:"event_type,omitempty"`
	Usage     *Usage         `json:"usage,omitempty"`
}

type ListModelsResponse struct {
	Models   []Model `json:"models"`
	Provider string  `json:"provider"`
}

type Message struct {
	Content    string     `json:"content"`
	Role       string     `json:"role"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
	Reasoning  string     `json:"reasoning,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
}

type Model struct {
	Name string `json:"name"`
}

type ResponseTokens struct {
	Content   string     `json:"content"`
	Model     string     `json:"model,omitempty"`
	Role      string     `json:"role,omitempty"`
	ToolCalls []ToolCall `json:"tool_calls,omitempty"`
}

type Usage struct {
	QueueTime        float64 `json:"queue_time"`
	PromptTokens     int     `json:"prompt_tokens"`
	PromptTime       float64 `json:"prompt_time"`
	CompletionTokens int     `json:"completion_tokens"`
	CompletionTime   float64 `json:"completion_time"`
	TotalTokens      int     `json:"total_tokens"`
	TotalTime        float64 `json:"total_time"`
}

func Float64Ptr(v float64) *float64 {
	return &v
}

func IntPtr(v int) *int {
	return &v
}

func BoolPtr(v bool) *bool {
	return &v
}

type EventType string
type EventTypeValue string

const (
	EventStreamStart    EventType = "stream-start"
	EventMessageStart   EventType = "message-start"
	EventContentStart   EventType = "content-start"
	EventContentDelta   EventType = "content-delta"
	EventContentEnd     EventType = "content-end"
	EventMessageEnd     EventType = "message-end"
	EventStreamEnd      EventType = "stream-end"
	EventTextGeneration EventType = "text-generation"
)

const (
	EventStreamStartValue    EventTypeValue = `{"role":"assistant"}`
	EventMessageStartValue   EventTypeValue = `{}`
	EventContentStartValue   EventTypeValue = `{}`
	EventContentEndValue     EventTypeValue = `{}`
	EventMessageEndValue     EventTypeValue = `{}`
	EventStreamEndValue      EventTypeValue = `{}`
	EventTextGenerationValue EventTypeValue = `{}`
)

const (
	Event = "event"
	Done  = "[DONE]"
	Data  = "data"
	Retry = "retry"
)

// SSEEvent represents a Server-Sent Event
type SSEvent struct {
	EventType EventType
	Data      []byte
}

// ParseSSEvents parses a Server-Sent Event from a byte slice
func ParseSSEvents(line []byte) (*SSEvent, error) {
	if len(bytes.TrimSpace(line)) == 0 {
		return nil, fmt.Errorf("empty line")
	}

	lines := bytes.Split(line, []byte("\n"))
	event := &SSEvent{}
	for _, line := range lines {
		line = bytes.TrimSpace(line)
		if len(line) == 0 {
			continue
		}

		parts := bytes.SplitN(line, []byte(":"), 2)
		if len(parts) != 2 {
			continue
		}

		field := string(bytes.TrimSpace(parts[0]))
		value := bytes.TrimSpace(parts[1])

		if bytes.Equal(value, []byte(Done)) {
			event.EventType = EventStreamEnd
			return event, nil
		}

		switch field {
		case "data":
			event.Data = value

			switch {
			case bytes.Contains(value, []byte(EventStreamStart)):
				event.EventType = EventStreamStart
			case bytes.Contains(value, []byte(EventMessageStart)):
				event.EventType = EventMessageStart
			case bytes.Contains(value, []byte(EventContentStart)):
				event.EventType = EventContentStart
			case bytes.Contains(value, []byte(EventContentDelta)):
				event.EventType = EventContentDelta
			case bytes.Contains(value, []byte(EventTextGeneration)):
				event.EventType = EventContentDelta
			case bytes.Contains(value, []byte(EventContentEnd)):
				event.EventType = EventContentEnd
			case bytes.Contains(value, []byte(EventMessageEnd)):
				event.EventType = EventMessageEnd
			case bytes.Contains(value, []byte(EventStreamEnd)):
				event.EventType = EventStreamEnd
			default:
				event.EventType = EventContentDelta
			}

		case "event":
			event.EventType = EventType(string(value))
		}
	}

	return event, nil
}

func readSSEventsChunk(reader *bufio.Reader) ([]byte, error) {
	var buffer []byte

	for {
		line, err := reader.ReadBytes('\n')

		if err != nil {
			if err == io.EOF {
				if len(buffer) > 0 {
					return buffer, nil
				}
				return nil, err
			}
			return nil, err
		}

		buffer = append(buffer, line...)

		if len(buffer) > 2 {
			if bytes.HasSuffix(buffer, []byte("\n\n")) {
				return buffer, nil
			}
		}
	}
}

type StreamParser interface {
	ParseChunk(reader *bufio.Reader) (*SSEvent, error)
}

func NewStreamParser(l logger.Logger, provider string) (StreamParser, error) {
	switch provider {
	case OllamaID:
		return &OllamaStreamParser{
			logger: l,
		}, nil
	case OpenaiID:
		return &OpenaiStreamParser{
			logger: l,
		}, nil
	case GroqID:
		return NewGroqStreamParser(l), nil
	case CloudflareID:
		return &CloudflareStreamParser{
			logger: l,
		}, nil
	case CohereID:
		return &CohereStreamParser{
			logger: l,
		}, nil
	case AnthropicID:
		return &AnthropicStreamParser{
			logger: l,
		}, nil
	default:
		return nil, fmt.Errorf("unsupported provider: %s", provider)
	}
}
