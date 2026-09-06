package types_test

import (
	"encoding/json"
	"strings"
	"testing"

	types "github.com/inference-gateway/inference-gateway/providers/types"
)

func TestSSEErrorEvent(t *testing.T) {
	message := `upstream said "no" ` + "\nretry later"

	frame := string(types.SSEErrorEvent(message))

	payload, found := strings.CutPrefix(strings.TrimSuffix(frame, "\n\n"), types.SSEDataPrefix)
	if !found {
		t.Fatalf("frame %q is missing the %q prefix", frame, types.SSEDataPrefix)
	}
	if strings.ContainsAny(payload, "\n") {
		t.Fatalf("payload %q contains a raw newline", payload)
	}

	var decoded struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal([]byte(payload), &decoded); err != nil {
		t.Fatalf("failed to decode payload %q: %v", payload, err)
	}
	if decoded.Error != message {
		t.Errorf("expected error %q, got %q", message, decoded.Error)
	}
}
