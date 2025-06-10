// Code generated from JSON schema. DO NOT EDIT.
package a2a

// Represents the possible states of a Task.
type TaskState string

// TaskState enum values
const (
	TaskStateAuthRequired  TaskState = "auth-required"
	TaskStateCanceled      TaskState = "canceled"
	TaskStateCompleted     TaskState = "completed"
	TaskStateFailed        TaskState = "failed"
	TaskStateInputRequired TaskState = "input-required"
	TaskStateRejected      TaskState = "rejected"
	TaskStateSubmitted     TaskState = "submitted"
	TaskStateUnknown       TaskState = "unknown"
	TaskStateWorking       TaskState = "working"
)

type A2AError interface{}

// A2A supported request types
type A2ARequest interface{}

// API Key security scheme.
type APIKeySecurityScheme struct {
	Description *string `json:"description,omitempty"`
	In          string  `json:"in"`
	Name        string  `json:"name"`
	Type        string  `json:"type"`
}

// Defines optional capabilities supported by an agent.
type AgentCapabilities struct {
	Extensions             []AgentExtension `json:"extensions,omitempty"`
	PushNotifications      *bool            `json:"pushNotifications,omitempty"`
	StateTransitionHistory *bool            `json:"stateTransitionHistory,omitempty"`
	Streaming              *bool            `json:"streaming,omitempty"`
}

// An AgentCard conveys key information:
// - Overall details (version, name, description, uses)
// - Skills: A set of capabilities the agent can perform
// - Default modalities/content types supported by the agent.
// - Authentication requirements
type AgentCard struct {
	Capabilities                      AgentCapabilities         `json:"capabilities"`
	DefaultInputModes                 []string                  `json:"defaultInputModes"`
	DefaultOutputModes                []string                  `json:"defaultOutputModes"`
	Description                       string                    `json:"description"`
	DocumentationURL                  *string                   `json:"documentationUrl,omitempty"`
	IconURL                           *string                   `json:"iconUrl,omitempty"`
	Name                              string                    `json:"name"`
	Provider                          *AgentProvider            `json:"provider,omitempty"`
	Security                          []map[string][]string     `json:"security,omitempty"`
	SecuritySchemes                   map[string]SecurityScheme `json:"securitySchemes,omitempty"`
	Skills                            []AgentSkill              `json:"skills"`
	SupportsAuthenticatedExtendedCard *bool                     `json:"supportsAuthenticatedExtendedCard,omitempty"`
	URL                               string                    `json:"url"`
	Version                           string                    `json:"version"`
}

// A declaration of an extension supported by an Agent.
type AgentExtension struct {
	Description *string                `json:"description,omitempty"`
	Params      map[string]interface{} `json:"params,omitempty"`
	Required    *bool                  `json:"required,omitempty"`
	URI         string                 `json:"uri"`
}

// Represents the service provider of an agent.
type AgentProvider struct {
	Organization string `json:"organization"`
	URL          string `json:"url"`
}

// Represents a unit of capability that an agent can perform.
type AgentSkill struct {
	Description string   `json:"description"`
	Examples    []string `json:"examples,omitempty"`
	ID          string   `json:"id"`
	InputModes  []string `json:"inputModes,omitempty"`
	Name        string   `json:"name"`
	OutputModes []string `json:"outputModes,omitempty"`
	Tags        []string `json:"tags"`
}

// Represents an artifact generated for a task.
type Artifact struct {
	ArtifactID  string                 `json:"artifactId"`
	Description *string                `json:"description,omitempty"`
	Extensions  []string               `json:"extensions,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
	Name        *string                `json:"name,omitempty"`
	Parts       []Part                 `json:"parts"`
}

// Configuration details for a supported OAuth Flow
type AuthorizationCodeOAuthFlow struct {
	AuthorizationURL string            `json:"authorizationUrl"`
	RefreshURL       *string           `json:"refreshUrl,omitempty"`
	Scopes           map[string]string `json:"scopes"`
	TokenURL         string            `json:"tokenUrl"`
}

// JSON-RPC request model for the 'tasks/cancel' method.
type CancelTaskRequest struct {
	ID      interface{}  `json:"id"`
	JSONRPC string       `json:"jsonrpc"`
	Method  string       `json:"method"`
	Params  TaskIdParams `json:"params"`
}

// JSON-RPC response for the 'tasks/cancel' method.
type CancelTaskResponse interface{}

// JSON-RPC success response model for the 'tasks/cancel' method.
type CancelTaskSuccessResponse struct {
	ID      interface{} `json:"id"`
	JSONRPC string      `json:"jsonrpc"`
	Result  Task        `json:"result"`
}

// Configuration details for a supported OAuth Flow
type ClientCredentialsOAuthFlow struct {
	RefreshURL *string           `json:"refreshUrl,omitempty"`
	Scopes     map[string]string `json:"scopes"`
	TokenURL   string            `json:"tokenUrl"`
}

// A2A specific error indicating incompatible content types between request and agent capabilities.
type ContentTypeNotSupportedError struct {
	Code    int          `json:"code"`
	Data    *interface{} `json:"data,omitempty"`
	Message string       `json:"message"`
}

// Represents a structured data segment within a message part.
type DataPart struct {
	Data     map[string]interface{} `json:"data"`
	Kind     string                 `json:"kind"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// Represents the base entity for FileParts
type FileBase struct {
	MIMEType *string `json:"mimeType,omitempty"`
	Name     *string `json:"name,omitempty"`
}

// Represents a File segment within parts.
type FilePart struct {
	File     interface{}            `json:"file"`
	Kind     string                 `json:"kind"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// Define the variant where 'bytes' is present and 'uri' is absent
type FileWithBytes struct {
	Bytes    string  `json:"bytes"`
	MIMEType *string `json:"mimeType,omitempty"`
	Name     *string `json:"name,omitempty"`
}

// Define the variant where 'uri' is present and 'bytes' is absent
type FileWithUri struct {
	MIMEType *string `json:"mimeType,omitempty"`
	Name     *string `json:"name,omitempty"`
	URI      string  `json:"uri"`
}

// JSON-RPC request model for the 'tasks/pushNotificationConfig/get' method.
type GetTaskPushNotificationConfigRequest struct {
	ID      interface{}  `json:"id"`
	JSONRPC string       `json:"jsonrpc"`
	Method  string       `json:"method"`
	Params  TaskIdParams `json:"params"`
}

// JSON-RPC response for the 'tasks/pushNotificationConfig/set' method.
type GetTaskPushNotificationConfigResponse interface{}

// JSON-RPC success response model for the 'tasks/pushNotificationConfig/get' method.
type GetTaskPushNotificationConfigSuccessResponse struct {
	ID      interface{}                `json:"id"`
	JSONRPC string                     `json:"jsonrpc"`
	Result  TaskPushNotificationConfig `json:"result"`
}

// JSON-RPC request model for the 'tasks/get' method.
type GetTaskRequest struct {
	ID      interface{}     `json:"id"`
	JSONRPC string          `json:"jsonrpc"`
	Method  string          `json:"method"`
	Params  TaskQueryParams `json:"params"`
}

// JSON-RPC response for the 'tasks/get' method.
type GetTaskResponse interface{}

// JSON-RPC success response for the 'tasks/get' method.
type GetTaskSuccessResponse struct {
	ID      interface{} `json:"id"`
	JSONRPC string      `json:"jsonrpc"`
	Result  Task        `json:"result"`
}

// HTTP Authentication security scheme.
type HTTPAuthSecurityScheme struct {
	BearerFormat *string `json:"bearerFormat,omitempty"`
	Description  *string `json:"description,omitempty"`
	Scheme       string  `json:"scheme"`
	Type         string  `json:"type"`
}

// Configuration details for a supported OAuth Flow
type ImplicitOAuthFlow struct {
	AuthorizationURL string            `json:"authorizationUrl"`
	RefreshURL       *string           `json:"refreshUrl,omitempty"`
	Scopes           map[string]string `json:"scopes"`
}

// JSON-RPC error indicating an internal JSON-RPC error on the server.
type InternalError struct {
	Code    int          `json:"code"`
	Data    *interface{} `json:"data,omitempty"`
	Message string       `json:"message"`
}

// A2A specific error indicating agent returned invalid response for the current method
type InvalidAgentResponseError struct {
	Code    int          `json:"code"`
	Data    *interface{} `json:"data,omitempty"`
	Message string       `json:"message"`
}

// JSON-RPC error indicating invalid method parameter(s).
type InvalidParamsError struct {
	Code    int          `json:"code"`
	Data    *interface{} `json:"data,omitempty"`
	Message string       `json:"message"`
}

// JSON-RPC error indicating the JSON sent is not a valid Request object.
type InvalidRequestError struct {
	Code    int          `json:"code"`
	Data    *interface{} `json:"data,omitempty"`
	Message string       `json:"message"`
}

// JSON-RPC error indicating invalid JSON was received by the server.
type JSONParseError struct {
	Code    int          `json:"code"`
	Data    *interface{} `json:"data,omitempty"`
	Message string       `json:"message"`
}

// Represents a JSON-RPC 2.0 Error object.
// This is typically included in a JSONRPCErrorResponse when an error occurs.
type JSONRPCError struct {
	Code    int          `json:"code"`
	Data    *interface{} `json:"data,omitempty"`
	Message string       `json:"message"`
}

// Represents a JSON-RPC 2.0 Error Response object.
type JSONRPCErrorResponse struct {
	Error   interface{} `json:"error"`
	ID      interface{} `json:"id"`
	JSONRPC string      `json:"jsonrpc"`
}

// Base interface for any JSON-RPC 2.0 request or response.
type JSONRPCMessage struct {
	ID      *interface{} `json:"id,omitempty"`
	JSONRPC string       `json:"jsonrpc"`
}

// Represents a JSON-RPC 2.0 Request object.
type JSONRPCRequest struct {
	ID      *interface{}           `json:"id,omitempty"`
	JSONRPC string                 `json:"jsonrpc"`
	Method  string                 `json:"method"`
	Params  map[string]interface{} `json:"params,omitempty"`
}

// Represents a JSON-RPC 2.0 Response object.
type JSONRPCResponse interface{}

// Represents a JSON-RPC 2.0 Success Response object.
type JSONRPCSuccessResponse struct {
	ID      interface{} `json:"id"`
	JSONRPC string      `json:"jsonrpc"`
	Result  interface{} `json:"result"`
}

// Represents a single message exchanged between user and agent.
type Message struct {
	ContextID        *string                `json:"contextId,omitempty"`
	Extensions       []string               `json:"extensions,omitempty"`
	Kind             string                 `json:"kind"`
	MessageID        string                 `json:"messageId"`
	Metadata         map[string]interface{} `json:"metadata,omitempty"`
	Parts            []Part                 `json:"parts"`
	ReferenceTaskIds []string               `json:"referenceTaskIds,omitempty"`
	Role             string                 `json:"role"`
	TaskID           *string                `json:"taskId,omitempty"`
}

// Configuration for the send message request.
type MessageSendConfiguration struct {
	AcceptedOutputModes    []string                `json:"acceptedOutputModes"`
	Blocking               *bool                   `json:"blocking,omitempty"`
	HistoryLength          *int                    `json:"historyLength,omitempty"`
	PushNotificationConfig *PushNotificationConfig `json:"pushNotificationConfig,omitempty"`
}

// Sent by the client to the agent as a request. May create, continue or restart a task.
type MessageSendParams struct {
	Configuration *MessageSendConfiguration `json:"configuration,omitempty"`
	Message       Message                   `json:"message"`
	Metadata      map[string]interface{}    `json:"metadata,omitempty"`
}

// JSON-RPC error indicating the method does not exist or is not available.
type MethodNotFoundError struct {
	Code    int          `json:"code"`
	Data    *interface{} `json:"data,omitempty"`
	Message string       `json:"message"`
}

// OAuth2.0 security scheme configuration.
type OAuth2SecurityScheme struct {
	Description *string    `json:"description,omitempty"`
	Flows       OAuthFlows `json:"flows"`
	Type        string     `json:"type"`
}

// Allows configuration of the supported OAuth Flows
type OAuthFlows struct {
	AuthorizationCode *AuthorizationCodeOAuthFlow `json:"authorizationCode,omitempty"`
	ClientCredentials *ClientCredentialsOAuthFlow `json:"clientCredentials,omitempty"`
	Implicit          *ImplicitOAuthFlow          `json:"implicit,omitempty"`
	Password          *PasswordOAuthFlow          `json:"password,omitempty"`
}

// OpenID Connect security scheme configuration.
type OpenIdConnectSecurityScheme struct {
	Description      *string `json:"description,omitempty"`
	OpenIDConnectURL string  `json:"openIdConnectUrl"`
	Type             string  `json:"type"`
}

// Represents a part of a message, which can be text, a file, or structured data.
type Part interface{}

// Base properties common to all message parts.
type PartBase struct {
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// Configuration details for a supported OAuth Flow
type PasswordOAuthFlow struct {
	RefreshURL *string           `json:"refreshUrl,omitempty"`
	Scopes     map[string]string `json:"scopes"`
	TokenURL   string            `json:"tokenUrl"`
}

// Defines authentication details for push notifications.
type PushNotificationAuthenticationInfo struct {
	Credentials *string  `json:"credentials,omitempty"`
	Schemes     []string `json:"schemes"`
}

// Configuration for setting up push notifications for task updates.
type PushNotificationConfig struct {
	Authentication *PushNotificationAuthenticationInfo `json:"authentication,omitempty"`
	ID             *string                             `json:"id,omitempty"`
	Token          *string                             `json:"token,omitempty"`
	URL            string                              `json:"url"`
}

// A2A specific error indicating the agent does not support push notifications.
type PushNotificationNotSupportedError struct {
	Code    int          `json:"code"`
	Data    *interface{} `json:"data,omitempty"`
	Message string       `json:"message"`
}

// Mirrors the OpenAPI Security Scheme Object
// (https://swagger.io/specification/#security-scheme-object)
type SecurityScheme interface{}

// Base properties shared by all security schemes.
type SecuritySchemeBase struct {
	Description *string `json:"description,omitempty"`
}

// JSON-RPC request model for the 'message/send' method.
type SendMessageRequest struct {
	ID      interface{}       `json:"id"`
	JSONRPC string            `json:"jsonrpc"`
	Method  string            `json:"method"`
	Params  MessageSendParams `json:"params"`
}

// JSON-RPC response model for the 'message/send' method.
type SendMessageResponse interface{}

// JSON-RPC success response model for the 'message/send' method.
type SendMessageSuccessResponse struct {
	ID      interface{} `json:"id"`
	JSONRPC string      `json:"jsonrpc"`
	Result  interface{} `json:"result"`
}

// JSON-RPC request model for the 'message/stream' method.
type SendStreamingMessageRequest struct {
	ID      interface{}       `json:"id"`
	JSONRPC string            `json:"jsonrpc"`
	Method  string            `json:"method"`
	Params  MessageSendParams `json:"params"`
}

// JSON-RPC response model for the 'message/stream' method.
type SendStreamingMessageResponse interface{}

// JSON-RPC success response model for the 'message/stream' method.
type SendStreamingMessageSuccessResponse struct {
	ID      interface{} `json:"id"`
	JSONRPC string      `json:"jsonrpc"`
	Result  interface{} `json:"result"`
}

// JSON-RPC request model for the 'tasks/pushNotificationConfig/set' method.
type SetTaskPushNotificationConfigRequest struct {
	ID      interface{}                `json:"id"`
	JSONRPC string                     `json:"jsonrpc"`
	Method  string                     `json:"method"`
	Params  TaskPushNotificationConfig `json:"params"`
}

// JSON-RPC response for the 'tasks/pushNotificationConfig/set' method.
type SetTaskPushNotificationConfigResponse interface{}

// JSON-RPC success response model for the 'tasks/pushNotificationConfig/set' method.
type SetTaskPushNotificationConfigSuccessResponse struct {
	ID      interface{}                `json:"id"`
	JSONRPC string                     `json:"jsonrpc"`
	Result  TaskPushNotificationConfig `json:"result"`
}

type Task struct {
	Artifacts []Artifact             `json:"artifacts,omitempty"`
	ContextID string                 `json:"contextId"`
	History   []Message              `json:"history,omitempty"`
	ID        string                 `json:"id"`
	Kind      string                 `json:"kind"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
	Status    TaskStatus             `json:"status"`
}

// Sent by server during sendStream or subscribe requests
type TaskArtifactUpdateEvent struct {
	Append    *bool                  `json:"append,omitempty"`
	Artifact  Artifact               `json:"artifact"`
	ContextID string                 `json:"contextId"`
	Kind      string                 `json:"kind"`
	LastChunk *bool                  `json:"lastChunk,omitempty"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
	TaskID    string                 `json:"taskId"`
}

// Parameters containing only a task ID, used for simple task operations.
type TaskIdParams struct {
	ID       string                 `json:"id"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// A2A specific error indicating the task is in a state where it cannot be canceled.
type TaskNotCancelableError struct {
	Code    int          `json:"code"`
	Data    *interface{} `json:"data,omitempty"`
	Message string       `json:"message"`
}

// A2A specific error indicating the requested task ID was not found.
type TaskNotFoundError struct {
	Code    int          `json:"code"`
	Data    *interface{} `json:"data,omitempty"`
	Message string       `json:"message"`
}

// Parameters for setting or getting push notification configuration for a task
type TaskPushNotificationConfig struct {
	PushNotificationConfig PushNotificationConfig `json:"pushNotificationConfig"`
	TaskID                 string                 `json:"taskId"`
}

// Parameters for querying a task, including optional history length.
type TaskQueryParams struct {
	HistoryLength *int                   `json:"historyLength,omitempty"`
	ID            string                 `json:"id"`
	Metadata      map[string]interface{} `json:"metadata,omitempty"`
}

// JSON-RPC request model for the 'tasks/resubscribe' method.
type TaskResubscriptionRequest struct {
	ID      interface{}  `json:"id"`
	JSONRPC string       `json:"jsonrpc"`
	Method  string       `json:"method"`
	Params  TaskIdParams `json:"params"`
}

// TaskState and accompanying message.
type TaskStatus struct {
	Message   *Message  `json:"message,omitempty"`
	State     TaskState `json:"state"`
	Timestamp *string   `json:"timestamp,omitempty"`
}

// Sent by server during sendStream or subscribe requests
type TaskStatusUpdateEvent struct {
	ContextID string                 `json:"contextId"`
	Final     bool                   `json:"final"`
	Kind      string                 `json:"kind"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
	Status    TaskStatus             `json:"status"`
	TaskID    string                 `json:"taskId"`
}

// Represents a text segment within parts.
type TextPart struct {
	Kind     string                 `json:"kind"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
	Text     string                 `json:"text"`
}

// A2A specific error indicating the requested operation is not supported by the agent.
type UnsupportedOperationError struct {
	Code    int          `json:"code"`
	Data    *interface{} `json:"data,omitempty"`
	Message string       `json:"message"`
}
