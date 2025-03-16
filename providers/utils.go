package providers

import (
	"bufio"
	"bytes"
	"fmt"
	"io"

	"github.com/inference-gateway/inference-gateway/logger"
)

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
