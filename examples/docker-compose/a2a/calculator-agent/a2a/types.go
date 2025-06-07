package a2a

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
	Jsonrpc string      `json:"jsonrpc"`
	Error   JSONRPCError `json:"error"`
	ID      interface{} `json:"id"`
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

// A2A Request types for method parameters
type GreetRequest struct {
	Name string `json:"name,omitempty"`
}

type GreetResponse struct {
	Message string `json:"message"`
}
