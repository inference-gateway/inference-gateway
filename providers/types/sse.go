package types

import "encoding/json"

// SSE framing shared by the streaming relay, the MCP agent and telemetry.
const (
	SSEDataFieldPrefix = "data:"
	SSEDataPrefix      = SSEDataFieldPrefix + " "
	SSEDoneData        = "[DONE]"
	SSEDoneEvent       = SSEDataPrefix + SSEDoneData + "\n\n"
)

// SSEErrorEvent renders a gateway-side error as one JSON-safe SSE data frame.
func SSEErrorEvent(message string) []byte {
	payload, _ := json.Marshal(struct {
		Error string `json:"error"`
	}{message})
	return []byte(SSEDataPrefix + string(payload) + "\n\n")
}
