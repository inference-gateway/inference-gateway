package a2a

import (
	"time"
)

// JSON-RPC 2.0 types
type JSONRPCRequest struct {
	Jsonrpc string                 `json:"jsonrpc"`
	Method  string                 `json:"method"`
	Params  map[string]interface{} `json:"params,omitempty"`
	ID      interface{}            `json:"id"`
}

type JSONRPCSuccessResponse struct {
	Jsonrpc string      `json:"jsonrpc"`
	Result  interface{} `json:"result"`
	ID      interface{} `json:"id"`
}

type JSONRPCErrorResponse struct {
	Jsonrpc string       `json:"jsonrpc"`
	Error   JSONRPCError `json:"error"`
	ID      interface{}  `json:"id"`
}

type JSONRPCError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// A2A Agent types
type AgentCapabilities struct {
	Pushnotifications      bool `json:"pushNotifications"`
	Statetransitionhistory bool `json:"stateTransitionHistory"`
	Streaming              bool `json:"streaming"`
}

type AgentSkill struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Inputmodes  []string `json:"inputModes"`
	Outputmodes []string `json:"outputModes"`
}

type AgentCard struct {
	Capabilities       AgentCapabilities `json:"capabilities"`
	Defaultinputmodes  []string          `json:"defaultInputModes"`
	Defaultoutputmodes []string          `json:"defaultOutputModes"`
	Description        string            `json:"description"`
	Name               string            `json:"name"`
	Skills             []AgentSkill      `json:"skills"`
	URL                string            `json:"url"`
	Version            string            `json:"version"`
}

type Part struct {
	Type     string                 `json:"type"`
	Text     string                 `json:"text,omitempty"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

type Message struct {
	Role      string                 `json:"role"`
	Parts     []Part                 `json:"parts"`
	MessageId string                 `json:"messageId"`
	ContextId string                 `json:"contextId,omitempty"`
	TaskId    string                 `json:"taskId,omitempty"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

type MessageSendConfiguration struct {
	Blocking bool `json:"blocking,omitempty"`
}

type MessageSendParams struct {
	Message       Message                  `json:"message"`
	Configuration MessageSendConfiguration `json:"configuration,omitempty"`
	Metadata      map[string]interface{}   `json:"metadata,omitempty"`
}

type TaskStatus struct {
	State     string    `json:"state"`
	Timestamp time.Time `json:"timestamp"`
	Message   *Message  `json:"message,omitempty"`
}

type Artifact struct {
	ArtifactId string                 `json:"artifactId"`
	Name       string                 `json:"name,omitempty"`
	Parts      []Part                 `json:"parts"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
}

type Task struct {
	Id        string                 `json:"id"`
	ContextId string                 `json:"contextId"`
	Status    TaskStatus             `json:"status"`
	Artifacts []Artifact             `json:"artifacts,omitempty"`
	History   []Message              `json:"history,omitempty"`
	Kind      string                 `json:"kind"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}
