package providers

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
