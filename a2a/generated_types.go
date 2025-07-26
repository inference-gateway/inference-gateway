// Code generated from JSON schema. DO NOT EDIT.
package a2a

// Defines the lifecycle states of a Task.
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

// Supported A2A transport protocols.
type TransportProtocol string

// TransportProtocol enum values
const (
	TransportProtocolGrpc     TransportProtocol = "GRPC"
	TransportProtocolHttpjson TransportProtocol = "HTTP+JSON"
	TransportProtocolJSONRPC  TransportProtocol = "JSONRPC"
)

// A discriminated union of all standard JSON-RPC and A2A-specific error types.
type A2AError interface{}

// A discriminated union representing all possible JSON-RPC 2.0 requests supported by the A2A specification.
type A2ARequest interface{}

// Defines a security scheme using an API key.
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

// The AgentCard is a self-describing manifest for an agent. It provides essential
// metadata including the agent's identity, capabilities, skills, supported
// communication methods, and security requirements.
type AgentCard struct {
	AdditionalInterfaces              []AgentInterface          `json:"additionalInterfaces,omitempty"`
	Capabilities                      AgentCapabilities         `json:"capabilities"`
	DefaultInputModes                 []string                  `json:"defaultInputModes"`
	DefaultOutputModes                []string                  `json:"defaultOutputModes"`
	Description                       string                    `json:"description"`
	DocumentationURL                  *string                   `json:"documentationUrl,omitempty"`
	IconURL                           *string                   `json:"iconUrl,omitempty"`
	Name                              string                    `json:"name"`
	PreferredTransport                string                    `json:"preferredTransport,omitempty"`
	ProtocolVersion                   string                    `json:"protocolVersion"`
	Provider                          *AgentProvider            `json:"provider,omitempty"`
	Security                          []map[string][]string     `json:"security,omitempty"`
	SecuritySchemes                   map[string]SecurityScheme `json:"securitySchemes,omitempty"`
	Signatures                        []AgentCardSignature      `json:"signatures,omitempty"`
	Skills                            []AgentSkill              `json:"skills"`
	SupportsAuthenticatedExtendedCard *bool                     `json:"supportsAuthenticatedExtendedCard,omitempty"`
	URL                               string                    `json:"url"`
	Version                           string                    `json:"version"`
}

// AgentCardSignature represents a JWS signature of an AgentCard.
// This follows the JSON format of an RFC 7515 JSON Web Signature (JWS).
type AgentCardSignature struct {
	Header    map[string]interface{} `json:"header,omitempty"`
	Protected string                 `json:"protected"`
	Signature string                 `json:"signature"`
}

// A declaration of a protocol extension supported by an Agent.
type AgentExtension struct {
	Description *string                `json:"description,omitempty"`
	Params      map[string]interface{} `json:"params,omitempty"`
	Required    *bool                  `json:"required,omitempty"`
	URI         string                 `json:"uri"`
}

// Declares a combination of a target URL and a transport protocol for interacting with the agent.
// This allows agents to expose the same functionality over multiple transport mechanisms.
type AgentInterface struct {
	Transport string `json:"transport"`
	URL       string `json:"url"`
}

// Represents the service provider of an agent.
type AgentProvider struct {
	Organization string `json:"organization"`
	URL          string `json:"url"`
}

// Represents a distinct capability or function that an agent can perform.
type AgentSkill struct {
	Description string   `json:"description"`
	Examples    []string `json:"examples,omitempty"`
	ID          string   `json:"id"`
	InputModes  []string `json:"inputModes,omitempty"`
	Name        string   `json:"name"`
	OutputModes []string `json:"outputModes,omitempty"`
	Tags        []string `json:"tags"`
}

// Represents a file, data structure, or other resource generated by an agent during a task.
type Artifact struct {
	ArtifactID  string                 `json:"artifactId"`
	Description *string                `json:"description,omitempty"`
	Extensions  []string               `json:"extensions,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
	Name        *string                `json:"name,omitempty"`
	Parts       []Part                 `json:"parts"`
}

// Defines configuration details for the OAuth 2.0 Authorization Code flow.
type AuthorizationCodeOAuthFlow struct {
	AuthorizationURL string            `json:"authorizationUrl"`
	RefreshURL       *string           `json:"refreshUrl,omitempty"`
	Scopes           map[string]string `json:"scopes"`
	TokenURL         string            `json:"tokenUrl"`
}

// Represents a JSON-RPC request for the `tasks/cancel` method.
type CancelTaskRequest struct {
	ID      interface{}  `json:"id"`
	JSONRPC string       `json:"jsonrpc"`
	Method  string       `json:"method"`
	Params  TaskIdParams `json:"params"`
}

// Represents a JSON-RPC response for the `tasks/cancel` method.
type CancelTaskResponse interface{}

// Represents a successful JSON-RPC response for the `tasks/cancel` method.
type CancelTaskSuccessResponse struct {
	ID      interface{} `json:"id"`
	JSONRPC string      `json:"jsonrpc"`
	Result  Task        `json:"result"`
}

// Defines configuration details for the OAuth 2.0 Client Credentials flow.
type ClientCredentialsOAuthFlow struct {
	RefreshURL *string           `json:"refreshUrl,omitempty"`
	Scopes     map[string]string `json:"scopes"`
	TokenURL   string            `json:"tokenUrl"`
}

// An A2A-specific error indicating an incompatibility between the requested
// content types and the agent's capabilities.
type ContentTypeNotSupportedError struct {
	Code    int          `json:"code"`
	Data    *interface{} `json:"data,omitempty"`
	Message string       `json:"message"`
}

// Represents a structured data segment (e.g., JSON) within a message or artifact.
type DataPart struct {
	Data     map[string]interface{} `json:"data"`
	Kind     string                 `json:"kind"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// Defines parameters for deleting a specific push notification configuration for a task.
type DeleteTaskPushNotificationConfigParams struct {
	ID                       string                 `json:"id"`
	Metadata                 map[string]interface{} `json:"metadata,omitempty"`
	PushNotificationConfigID string                 `json:"pushNotificationConfigId"`
}

// Represents a JSON-RPC request for the `tasks/pushNotificationConfig/delete` method.
type DeleteTaskPushNotificationConfigRequest struct {
	ID      interface{}                            `json:"id"`
	JSONRPC string                                 `json:"jsonrpc"`
	Method  string                                 `json:"method"`
	Params  DeleteTaskPushNotificationConfigParams `json:"params"`
}

// Represents a JSON-RPC response for the `tasks/pushNotificationConfig/delete` method.
type DeleteTaskPushNotificationConfigResponse interface{}

// Represents a successful JSON-RPC response for the `tasks/pushNotificationConfig/delete` method.
type DeleteTaskPushNotificationConfigSuccessResponse struct {
	ID      interface{} `json:"id"`
	JSONRPC string      `json:"jsonrpc"`
	Result  interface{} `json:"result"`
}

// Defines base properties for a file.
type FileBase struct {
	MIMEType *string `json:"mimeType,omitempty"`
	Name     *string `json:"name,omitempty"`
}

// Represents a file segment within a message or artifact. The file content can be
// provided either directly as bytes or as a URI.
type FilePart struct {
	File     interface{}            `json:"file"`
	Kind     string                 `json:"kind"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// Represents a file with its content provided directly as a base64-encoded string.
type FileWithBytes struct {
	Bytes    string  `json:"bytes"`
	MIMEType *string `json:"mimeType,omitempty"`
	Name     *string `json:"name,omitempty"`
}

// Represents a file with its content located at a specific URI.
type FileWithUri struct {
	MIMEType *string `json:"mimeType,omitempty"`
	Name     *string `json:"name,omitempty"`
	URI      string  `json:"uri"`
}

// Defines parameters for fetching a specific push notification configuration for a task.
type GetTaskPushNotificationConfigParams struct {
	ID                       string                 `json:"id"`
	Metadata                 map[string]interface{} `json:"metadata,omitempty"`
	PushNotificationConfigID *string                `json:"pushNotificationConfigId,omitempty"`
}

// Represents a JSON-RPC request for the `tasks/pushNotificationConfig/get` method.
type GetTaskPushNotificationConfigRequest struct {
	ID      interface{} `json:"id"`
	JSONRPC string      `json:"jsonrpc"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params"`
}

// Represents a JSON-RPC response for the `tasks/pushNotificationConfig/get` method.
type GetTaskPushNotificationConfigResponse interface{}

// Represents a successful JSON-RPC response for the `tasks/pushNotificationConfig/get` method.
type GetTaskPushNotificationConfigSuccessResponse struct {
	ID      interface{}                `json:"id"`
	JSONRPC string                     `json:"jsonrpc"`
	Result  TaskPushNotificationConfig `json:"result"`
}

// Represents a JSON-RPC request for the `tasks/get` method.
type GetTaskRequest struct {
	ID      interface{}     `json:"id"`
	JSONRPC string          `json:"jsonrpc"`
	Method  string          `json:"method"`
	Params  TaskQueryParams `json:"params"`
}

// Represents a JSON-RPC response for the `tasks/get` method.
type GetTaskResponse interface{}

// Represents a successful JSON-RPC response for the `tasks/get` method.
type GetTaskSuccessResponse struct {
	ID      interface{} `json:"id"`
	JSONRPC string      `json:"jsonrpc"`
	Result  Task        `json:"result"`
}

// Defines a security scheme using HTTP authentication.
type HTTPAuthSecurityScheme struct {
	BearerFormat *string `json:"bearerFormat,omitempty"`
	Description  *string `json:"description,omitempty"`
	Scheme       string  `json:"scheme"`
	Type         string  `json:"type"`
}

// Defines configuration details for the OAuth 2.0 Implicit flow.
type ImplicitOAuthFlow struct {
	AuthorizationURL string            `json:"authorizationUrl"`
	RefreshURL       *string           `json:"refreshUrl,omitempty"`
	Scopes           map[string]string `json:"scopes"`
}

// An error indicating an internal error on the server.
type InternalError struct {
	Code    int          `json:"code"`
	Data    *interface{} `json:"data,omitempty"`
	Message string       `json:"message"`
}

// An A2A-specific error indicating that the agent returned a response that
// does not conform to the specification for the current method.
type InvalidAgentResponseError struct {
	Code    int          `json:"code"`
	Data    *interface{} `json:"data,omitempty"`
	Message string       `json:"message"`
}

// An error indicating that the method parameters are invalid.
type InvalidParamsError struct {
	Code    int          `json:"code"`
	Data    *interface{} `json:"data,omitempty"`
	Message string       `json:"message"`
}

// An error indicating that the JSON sent is not a valid Request object.
type InvalidRequestError struct {
	Code    int          `json:"code"`
	Data    *interface{} `json:"data,omitempty"`
	Message string       `json:"message"`
}

// An error indicating that the server received invalid JSON.
type JSONParseError struct {
	Code    int          `json:"code"`
	Data    *interface{} `json:"data,omitempty"`
	Message string       `json:"message"`
}

// Represents a JSON-RPC 2.0 Error object, included in an error response.
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

// Defines the base structure for any JSON-RPC 2.0 request, response, or notification.
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

// A discriminated union representing all possible JSON-RPC 2.0 responses
// for the A2A specification methods.
type JSONRPCResponse interface{}

// Represents a successful JSON-RPC 2.0 Response object.
type JSONRPCSuccessResponse struct {
	ID      interface{} `json:"id"`
	JSONRPC string      `json:"jsonrpc"`
	Result  interface{} `json:"result"`
}

// Defines parameters for listing all push notification configurations associated with a task.
type ListTaskPushNotificationConfigParams struct {
	ID       string                 `json:"id"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// Represents a JSON-RPC request for the `tasks/pushNotificationConfig/list` method.
type ListTaskPushNotificationConfigRequest struct {
	ID      interface{}                          `json:"id"`
	JSONRPC string                               `json:"jsonrpc"`
	Method  string                               `json:"method"`
	Params  ListTaskPushNotificationConfigParams `json:"params"`
}

// Represents a JSON-RPC response for the `tasks/pushNotificationConfig/list` method.
type ListTaskPushNotificationConfigResponse interface{}

// Represents a successful JSON-RPC response for the `tasks/pushNotificationConfig/list` method.
type ListTaskPushNotificationConfigSuccessResponse struct {
	ID      interface{}                  `json:"id"`
	JSONRPC string                       `json:"jsonrpc"`
	Result  []TaskPushNotificationConfig `json:"result"`
}

// Represents a single message in the conversation between a user and an agent.
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

// Defines configuration options for a `message/send` or `message/stream` request.
type MessageSendConfiguration struct {
	AcceptedOutputModes    []string                `json:"acceptedOutputModes,omitempty"`
	Blocking               *bool                   `json:"blocking,omitempty"`
	HistoryLength          *int                    `json:"historyLength,omitempty"`
	PushNotificationConfig *PushNotificationConfig `json:"pushNotificationConfig,omitempty"`
}

// Defines the parameters for a request to send a message to an agent. This can be used
// to create a new task, continue an existing one, or restart a task.
type MessageSendParams struct {
	Configuration *MessageSendConfiguration `json:"configuration,omitempty"`
	Message       Message                   `json:"message"`
	Metadata      map[string]interface{}    `json:"metadata,omitempty"`
}

// An error indicating that the requested method does not exist or is not available.
type MethodNotFoundError struct {
	Code    int          `json:"code"`
	Data    *interface{} `json:"data,omitempty"`
	Message string       `json:"message"`
}

// Defines a security scheme using OAuth 2.0.
type OAuth2SecurityScheme struct {
	Description *string    `json:"description,omitempty"`
	Flows       OAuthFlows `json:"flows"`
	Type        string     `json:"type"`
}

// Defines the configuration for the supported OAuth 2.0 flows.
type OAuthFlows struct {
	AuthorizationCode *AuthorizationCodeOAuthFlow `json:"authorizationCode,omitempty"`
	ClientCredentials *ClientCredentialsOAuthFlow `json:"clientCredentials,omitempty"`
	Implicit          *ImplicitOAuthFlow          `json:"implicit,omitempty"`
	Password          *PasswordOAuthFlow          `json:"password,omitempty"`
}

// Defines a security scheme using OpenID Connect.
type OpenIdConnectSecurityScheme struct {
	Description      *string `json:"description,omitempty"`
	OpenIDConnectURL string  `json:"openIdConnectUrl"`
	Type             string  `json:"type"`
}

// A discriminated union representing a part of a message or artifact, which can
// be text, a file, or structured data.
type Part interface{}

// Defines base properties common to all message or artifact parts.
type PartBase struct {
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// Defines configuration details for the OAuth 2.0 Resource Owner Password flow.
type PasswordOAuthFlow struct {
	RefreshURL *string           `json:"refreshUrl,omitempty"`
	Scopes     map[string]string `json:"scopes"`
	TokenURL   string            `json:"tokenUrl"`
}

// Defines authentication details for a push notification endpoint.
type PushNotificationAuthenticationInfo struct {
	Credentials *string  `json:"credentials,omitempty"`
	Schemes     []string `json:"schemes"`
}

// Defines the configuration for setting up push notifications for task updates.
type PushNotificationConfig struct {
	Authentication *PushNotificationAuthenticationInfo `json:"authentication,omitempty"`
	ID             *string                             `json:"id,omitempty"`
	Token          *string                             `json:"token,omitempty"`
	URL            string                              `json:"url"`
}

// An A2A-specific error indicating that the agent does not support push notifications.
type PushNotificationNotSupportedError struct {
	Code    int          `json:"code"`
	Data    *interface{} `json:"data,omitempty"`
	Message string       `json:"message"`
}

// Defines a security scheme that can be used to secure an agent's endpoints.
// This is a discriminated union type based on the OpenAPI 3.0 Security Scheme Object.
type SecurityScheme interface{}

// Defines base properties shared by all security scheme objects.
type SecuritySchemeBase struct {
	Description *string `json:"description,omitempty"`
}

// Represents a JSON-RPC request for the `message/send` method.
type SendMessageRequest struct {
	ID      interface{}       `json:"id"`
	JSONRPC string            `json:"jsonrpc"`
	Method  string            `json:"method"`
	Params  MessageSendParams `json:"params"`
}

// Represents a JSON-RPC response for the `message/send` method.
type SendMessageResponse interface{}

// Represents a successful JSON-RPC response for the `message/send` method.
type SendMessageSuccessResponse struct {
	ID      interface{} `json:"id"`
	JSONRPC string      `json:"jsonrpc"`
	Result  interface{} `json:"result"`
}

// Represents a JSON-RPC request for the `message/stream` method.
type SendStreamingMessageRequest struct {
	ID      interface{}       `json:"id"`
	JSONRPC string            `json:"jsonrpc"`
	Method  string            `json:"method"`
	Params  MessageSendParams `json:"params"`
}

// Represents a JSON-RPC response for the `message/stream` method.
type SendStreamingMessageResponse interface{}

// Represents a successful JSON-RPC response for the `message/stream` method.
// The server may send multiple response objects for a single request.
type SendStreamingMessageSuccessResponse struct {
	ID      interface{} `json:"id"`
	JSONRPC string      `json:"jsonrpc"`
	Result  interface{} `json:"result"`
}

// Represents a JSON-RPC request for the `tasks/pushNotificationConfig/set` method.
type SetTaskPushNotificationConfigRequest struct {
	ID      interface{}                `json:"id"`
	JSONRPC string                     `json:"jsonrpc"`
	Method  string                     `json:"method"`
	Params  TaskPushNotificationConfig `json:"params"`
}

// Represents a JSON-RPC response for the `tasks/pushNotificationConfig/set` method.
type SetTaskPushNotificationConfigResponse interface{}

// Represents a successful JSON-RPC response for the `tasks/pushNotificationConfig/set` method.
type SetTaskPushNotificationConfigSuccessResponse struct {
	ID      interface{}                `json:"id"`
	JSONRPC string                     `json:"jsonrpc"`
	Result  TaskPushNotificationConfig `json:"result"`
}

// Represents a single, stateful operation or conversation between a client and an agent.
type Task struct {
	Artifacts []Artifact             `json:"artifacts,omitempty"`
	ContextID string                 `json:"contextId"`
	History   []Message              `json:"history,omitempty"`
	ID        string                 `json:"id"`
	Kind      string                 `json:"kind"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
	Status    TaskStatus             `json:"status"`
}

// An event sent by the agent to notify the client that an artifact has been
// generated or updated. This is typically used in streaming models.
type TaskArtifactUpdateEvent struct {
	Append    *bool                  `json:"append,omitempty"`
	Artifact  Artifact               `json:"artifact"`
	ContextID string                 `json:"contextId"`
	Kind      string                 `json:"kind"`
	LastChunk *bool                  `json:"lastChunk,omitempty"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
	TaskID    string                 `json:"taskId"`
}

// Defines parameters containing a task ID, used for simple task operations.
type TaskIdParams struct {
	ID       string                 `json:"id"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// An A2A-specific error indicating that the task is in a state where it cannot be canceled.
type TaskNotCancelableError struct {
	Code    int          `json:"code"`
	Data    *interface{} `json:"data,omitempty"`
	Message string       `json:"message"`
}

// An A2A-specific error indicating that the requested task ID was not found.
type TaskNotFoundError struct {
	Code    int          `json:"code"`
	Data    *interface{} `json:"data,omitempty"`
	Message string       `json:"message"`
}

// A container associating a push notification configuration with a specific task.
type TaskPushNotificationConfig struct {
	PushNotificationConfig PushNotificationConfig `json:"pushNotificationConfig"`
	TaskID                 string                 `json:"taskId"`
}

// Defines parameters for querying a task, with an option to limit history length.
type TaskQueryParams struct {
	HistoryLength *int                   `json:"historyLength,omitempty"`
	ID            string                 `json:"id"`
	Metadata      map[string]interface{} `json:"metadata,omitempty"`
}

// Represents a JSON-RPC request for the `tasks/resubscribe` method, used to resume a streaming connection.
type TaskResubscriptionRequest struct {
	ID      interface{}  `json:"id"`
	JSONRPC string       `json:"jsonrpc"`
	Method  string       `json:"method"`
	Params  TaskIdParams `json:"params"`
}

// Represents the status of a task at a specific point in time.
type TaskStatus struct {
	Message   *Message  `json:"message,omitempty"`
	State     TaskState `json:"state"`
	Timestamp *string   `json:"timestamp,omitempty"`
}

// An event sent by the agent to notify the client of a change in a task's status.
// This is typically used in streaming or subscription models.
type TaskStatusUpdateEvent struct {
	ContextID string                 `json:"contextId"`
	Final     bool                   `json:"final"`
	Kind      string                 `json:"kind"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
	Status    TaskStatus             `json:"status"`
	TaskID    string                 `json:"taskId"`
}

// Represents a text segment within a message or artifact.
type TextPart struct {
	Kind     string                 `json:"kind"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
	Text     string                 `json:"text"`
}

// An A2A-specific error indicating that the requested operation is not supported by the agent.
type UnsupportedOperationError struct {
	Code    int          `json:"code"`
	Data    *interface{} `json:"data,omitempty"`
	Message string       `json:"message"`
}
