package providers

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
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
	GoogleDefaultBaseURL     = "https://generativelanguage.googleapis.com"
	GroqDefaultBaseURL       = "https://api.groq.com"
	OllamaDefaultBaseURL     = "http://ollama:8080"
	OpenaiDefaultBaseURL     = "https://api.openai.com"
)

// The ID's of each provider
const (
	AnthropicID  = "anthropic"
	CloudflareID = "cloudflare"
	CohereID     = "cohere"
	GoogleID     = "google"
	GroqID       = "groq"
	OllamaID     = "ollama"
	OpenaiID     = "openai"
)

// Display names for providers
const (
	AnthropicDisplayName  = "Anthropic"
	CloudflareDisplayName = "Cloudflare"
	CohereDisplayName     = "Cohere"
	GoogleDisplayName     = "Google"
	GroqDisplayName       = "Groq"
	OllamaDisplayName     = "Ollama"
	OpenaiDisplayName     = "Openai"
)

const (
	RoleSystem    = "system"
	RoleUser      = "user"
	RoleAssistant = "assistant"
)

// Common response and request types
type GenerateRequest struct {
	Messages []Message `json:"messages"`
	Model    string    `json:"model"`
	Stream   bool      `json:"stream"`
	SSEvents bool      `json:"ss_events"`
}

type GenerateResponse struct {
	Provider string         `json:"provider"`
	Response ResponseTokens `json:"response"`
}

type ListModelsResponse struct {
	Models   []Model `json:"models"`
	Provider string  `json:"provider"`
}

type Message struct {
	Content string `json:"content"`
	Role    string `json:"role"`
}

type Model struct {
	Name string `json:"name"`
}

type ResponseTokens struct {
	Content string `json:"content"`
	Model   string `json:"model"`
	Role    string `json:"role"`
}

func float64Ptr(v float64) *float64 {
	return &v
}

func intPtr(v int) *int {
	return &v
}

type EventType string

const (
	EventMessageStart EventType = "message-start"
	EventContentStart EventType = "content-start"
	EventContentDelta EventType = "content-delta"
	EventContentEnd   EventType = "content-end"
	EventStreamEnd    EventType = "message-end"
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

// Some providers have SSE built-in and some don't, for now I'll just
// chop off the "data: " prefix and add it after the unmarshal process
// I think it's up to the user to decide if they want SSE or not, therefore
// I made it configurable in the GenerateRequest struct
func parseSSE(line []byte) (*SSEvent, error) {
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

		if bytes.Equal(line, []byte("data: [DONE]")) {
			event.EventType = EventStreamEnd
			return event, nil
		}

		parts := bytes.SplitN(line, []byte(":"), 2)
		if len(parts) != 2 {
			continue
		}

		field := string(bytes.TrimSpace(parts[0]))
		value := bytes.TrimSpace(parts[1])

		switch field {
		case "data":
			event.Data = value
			if bytes.Equal(value, []byte("[DONE]")) {
				event.EventType = EventStreamEnd
			} else {
				// Check if data contains message-start type
				if bytes.Contains(value, []byte("message-start")) {
					event.EventType = EventMessageStart
				} else {
					event.EventType = EventContentDelta
				}
			}
		case "event":
			event.EventType = EventType(string(value))
		}
	}

	return event, nil
}

func readSSEChunk(reader *bufio.Reader) ([]byte, error) {
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
			if bytes.Contains(buffer, []byte("data: [DONE]")) {
				return buffer, nil
			}

			if bytes.HasSuffix(buffer, []byte("\n\n")) {
				return buffer, nil
			}
		}
	}
}

type StreamParser interface {
	ParseChunk(reader *bufio.Reader) (*SSEvent, error)
}

func NewStreamParser(provider string) StreamParser {
	switch provider {
	case OllamaID:
		return &OllamaStreamParser{}
	case OpenaiID:
		return &OpenaiStreamParser{}
	default:
		return &GroqStreamParser{}
	}
}
