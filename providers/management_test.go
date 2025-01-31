package providers

import (
	"bytes"
	"testing"
)

func TestParseSSEDebug(t *testing.T) {
	testCases := []struct {
		name     string
		input    string
		wantType EventType
	}{
		{
			name:     "message-start event",
			input:    "event: message-start\n",
			wantType: EventMessageStart,
		},
		{
			name:     "content delta",
			input:    "data: {\"content\":\"hello\"}\n",
			wantType: EventContentDelta,
		},
		{
			name:     "stream end",
			input:    "data: [DONE]\n",
			wantType: EventStreamEnd,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			event, err := parseSSEvents([]byte(tc.input))
			if err != nil {
				t.Errorf("parseSSE() error = %v", err)
				return
			}
			t.Logf("Input: %q", tc.input)
			t.Logf("Got event type: %v", event.EventType)
			t.Logf("Got data: %q", event.Data)
		})
	}
}

func TestParseSSEWithEmbeddedMessageStart(t *testing.T) {
	input := `data: {"json": "{\"id\":\"d8c1879d-6c59-4eb7-8209-b184f81bcf15\",\"type\":\"message-start\",\"delta\":{\"message\":{\"role\":\"assistant\",\"content\":[],\"tool_plan\":\"\",\"tool_calls\":[],\"citations\":[]}}}"}`

	event, err := parseSSEvents([]byte(input))
	if err != nil {
		t.Fatalf("parseSSE() error = %v", err)
	}

	// Verify event type is MessageStart since message-start is in JSON
	if event.EventType != EventMessageStart {
		t.Errorf("expected EventMessageStart, got %v", event.EventType)
	}

	// Verify data is preserved
	if !bytes.Contains(event.Data, []byte("message-start")) {
		t.Errorf("data should contain message-start marker\ngot: %s", event.Data)
	}

	t.Logf("Event type: %v", event.EventType)
	t.Logf("Event data: %s", event.Data)
}
