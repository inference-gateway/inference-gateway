// Code generated from A2A schema. DO NOT EDIT.
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

type A2AError struct {
}

// A2A supported request types
type A2ARequest struct {
}

// API Key security scheme.
type APIKeySecurityScheme struct {
	Description string `json:"description"`
	In          string `json:"in"`
	Name        string `json:"name"`
	Type        string `json:"type"`
}

// Defines optional capabilities supported by an agent.
type AgentCapabilities struct {
	Extensions             []AgentExtension `json:"extensions"`
	Pushnotifications      bool             `json:"pushNotifications"`
	Statetransitionhistory bool             `json:"stateTransitionHistory"`
	Streaming              bool             `json:"streaming"`
}

// An AgentCard conveys key information:
// - Overall details (version, name, description, uses)
// - Skills: A set of capabilities the agent can perform
// - Default modalities/content types supported by the agent.
// - Authentication requirements
type AgentCard struct {
	Capabilities                      AgentCapabilities        `json:"capabilities"`
	Defaultinputmodes                 []string                 `json:"defaultInputModes"`
	Defaultoutputmodes                []string                 `json:"defaultOutputModes"`
	Description                       string                   `json:"description"`
	Documentationurl                  string                   `json:"documentationUrl"`
	Iconurl                           string                   `json:"iconUrl"`
	Name                              string                   `json:"name"`
	Provider                          AgentProvider            `json:"provider"`
	Security                          []map[string]interface{} `json:"security"`
	Securityschemes                   map[string]interface{}   `json:"securitySchemes"`
	Skills                            []AgentSkill             `json:"skills"`
	Supportsauthenticatedextendedcard bool                     `json:"supportsAuthenticatedExtendedCard"`
	URL                               string                   `json:"url"`
	Version                           string                   `json:"version"`
}

// A declaration of an extension supported by an Agent.
type AgentExtension struct {
	Description string                 `json:"description"`
	Params      map[string]interface{} `json:"params"`
	Required    bool                   `json:"required"`
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
	Examples    []string `json:"examples"`
	ID          string   `json:"id"`
	Inputmodes  []string `json:"inputModes"`
	Name        string   `json:"name"`
	Outputmodes []string `json:"outputModes"`
	Tags        []string `json:"tags"`
}

// Represents an artifact generated for a task.
type Artifact struct {
	Artifactid  string                 `json:"artifactId"`
	Description string                 `json:"description"`
	Extensions  []string               `json:"extensions"`
	Metadata    map[string]interface{} `json:"metadata"`
	Name        string                 `json:"name"`
	Parts       []Part                 `json:"parts"`
}

// Configuration details for a supported OAuth Flow
type AuthorizationCodeOAuthFlow struct {
	Authorizationurl string                 `json:"authorizationUrl"`
	Refreshurl       string                 `json:"refreshUrl"`
	Scopes           map[string]interface{} `json:"scopes"`
	Tokenurl         string                 `json:"tokenUrl"`
}

// JSON-RPC request model for the 'tasks/cancel' method.
type CancelTaskRequest struct {
	ID      interface{}  `json:"id"`
	JSONRPC string       `json:"jsonrpc"`
	Method  string       `json:"method"`
	Params  TaskIdParams `json:"params"`
}

// JSON-RPC response for the 'tasks/cancel' method.
type CancelTaskResponse struct {
}

// JSON-RPC success response model for the 'tasks/cancel' method.
type CancelTaskSuccessResponse struct {
	ID      interface{} `json:"id"`
	JSONRPC string      `json:"jsonrpc"`
	Result  Task        `json:"result"`
}

// Configuration details for a supported OAuth Flow
type ClientCredentialsOAuthFlow struct {
	Refreshurl string                 `json:"refreshUrl"`
	Scopes     map[string]interface{} `json:"scopes"`
	Tokenurl   string                 `json:"tokenUrl"`
}

// A2A specific error indicating incompatible content types between request and agent capabilities.
type ContentTypeNotSupportedError struct {
	Code    int         `json:"code"`
	Data    interface{} `json:"data"`
	Message string      `json:"message"`
}

// Represents a structured data segment within a message part.
type DataPart struct {
	Data     map[string]interface{} `json:"data"`
	Kind     string                 `json:"kind"`
	Metadata map[string]interface{} `json:"metadata"`
}

// Represents the base entity for FileParts
type FileBase struct {
	Mimetype string `json:"mimeType"`
	Name     string `json:"name"`
}

// Represents a File segment within parts.
type FilePart struct {
	File     interface{}            `json:"file"`
	Kind     string                 `json:"kind"`
	Metadata map[string]interface{} `json:"metadata"`
}

// Define the variant where 'bytes' is present and 'uri' is absent
type FileWithBytes struct {
	Bytes    string `json:"bytes"`
	Mimetype string `json:"mimeType"`
	Name     string `json:"name"`
}

// Define the variant where 'uri' is present and 'bytes' is absent
type FileWithUri struct {
	Mimetype string `json:"mimeType"`
	Name     string `json:"name"`
	URI      string `json:"uri"`
}

// JSON-RPC request model for the 'tasks/pushNotificationConfig/get' method.
type GetTaskPushNotificationConfigRequest struct {
	ID      interface{}  `json:"id"`
	JSONRPC string       `json:"jsonrpc"`
	Method  string       `json:"method"`
	Params  TaskIdParams `json:"params"`
}

// JSON-RPC response for the 'tasks/pushNotificationConfig/set' method.
type GetTaskPushNotificationConfigResponse struct {
}

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
type GetTaskResponse struct {
}

// JSON-RPC success response for the 'tasks/get' method.
type GetTaskSuccessResponse struct {
	ID      interface{} `json:"id"`
	JSONRPC string      `json:"jsonrpc"`
	Result  Task        `json:"result"`
}

// HTTP Authentication security scheme.
type HTTPAuthSecurityScheme struct {
	Bearerformat string `json:"bearerFormat"`
	Description  string `json:"description"`
	Scheme       string `json:"scheme"`
	Type         string `json:"type"`
}

// Configuration details for a supported OAuth Flow
type ImplicitOAuthFlow struct {
	Authorizationurl string                 `json:"authorizationUrl"`
	Refreshurl       string                 `json:"refreshUrl"`
	Scopes           map[string]interface{} `json:"scopes"`
}

// JSON-RPC error indicating an internal JSON-RPC error on the server.
type InternalError struct {
	Code    int         `json:"code"`
	Data    interface{} `json:"data"`
	Message string      `json:"message"`
}

// A2A specific error indicating agent returned invalid response for the current method
type InvalidAgentResponseError struct {
	Code    int         `json:"code"`
	Data    interface{} `json:"data"`
	Message string      `json:"message"`
}

// JSON-RPC error indicating invalid method parameter(s).
type InvalidParamsError struct {
	Code    int         `json:"code"`
	Data    interface{} `json:"data"`
	Message string      `json:"message"`
}

// JSON-RPC error indicating the JSON sent is not a valid Request object.
type InvalidRequestError struct {
	Code    int         `json:"code"`
	Data    interface{} `json:"data"`
	Message string      `json:"message"`
}

// JSON-RPC error indicating invalid JSON was received by the server.
type JSONParseError struct {
	Code    int         `json:"code"`
	Data    interface{} `json:"data"`
	Message string      `json:"message"`
}

// Represents a JSON-RPC 2.0 Error object.
// This is typically included in a JSONRPCErrorResponse when an error occurs.
type JSONRPCError struct {
	Code    int         `json:"code"`
	Data    interface{} `json:"data"`
	Message string      `json:"message"`
}

// Represents a JSON-RPC 2.0 Error Response object.
type JSONRPCErrorResponse struct {
	Error   interface{} `json:"error"`
	ID      interface{} `json:"id"`
	JSONRPC string      `json:"jsonrpc"`
}

// Base interface for any JSON-RPC 2.0 request or response.
type JSONRPCMessage struct {
	ID      interface{} `json:"id"`
	JSONRPC string      `json:"jsonrpc"`
}

// Represents a JSON-RPC 2.0 Request object.
type JSONRPCRequest struct {
	ID      interface{}            `json:"id"`
	JSONRPC string                 `json:"jsonrpc"`
	Method  string                 `json:"method"`
	Params  map[string]interface{} `json:"params"`
}

// Represents a JSON-RPC 2.0 Response object.
type JSONRPCResponse struct {
}

// Represents a JSON-RPC 2.0 Success Response object.
type JSONRPCSuccessResponse struct {
	ID      interface{} `json:"id"`
	JSONRPC string      `json:"jsonrpc"`
	Result  interface{} `json:"result"`
}

// Represents a single message exchanged between user and agent.
type Message struct {
	Contextid        string                 `json:"contextId"`
	Extensions       []string               `json:"extensions"`
	Kind             string                 `json:"kind"`
	Messageid        string                 `json:"messageId"`
	Metadata         map[string]interface{} `json:"metadata"`
	Parts            []Part                 `json:"parts"`
	Referencetaskids []string               `json:"referenceTaskIds"`
	Role             string                 `json:"role"`
	Taskid           string                 `json:"taskId"`
}

// Configuration for the send message request.
type MessageSendConfiguration struct {
	Acceptedoutputmodes    []string               `json:"acceptedOutputModes"`
	Blocking               bool                   `json:"blocking"`
	Historylength          int                    `json:"historyLength"`
	Pushnotificationconfig PushNotificationConfig `json:"pushNotificationConfig"`
}

// Sent by the client to the agent as a request. May create, continue or restart a task.
type MessageSendParams struct {
	Configuration MessageSendConfiguration `json:"configuration"`
	Message       Message                  `json:"message"`
	Metadata      map[string]interface{}   `json:"metadata"`
}

// JSON-RPC error indicating the method does not exist or is not available.
type MethodNotFoundError struct {
	Code    int         `json:"code"`
	Data    interface{} `json:"data"`
	Message string      `json:"message"`
}

// OAuth2.0 security scheme configuration.
type OAuth2SecurityScheme struct {
	Description string     `json:"description"`
	Flows       OAuthFlows `json:"flows"`
	Type        string     `json:"type"`
}

// Allows configuration of the supported OAuth Flows
type OAuthFlows struct {
	Authorizationcode AuthorizationCodeOAuthFlow `json:"authorizationCode"`
	Clientcredentials ClientCredentialsOAuthFlow `json:"clientCredentials"`
	Implicit          ImplicitOAuthFlow          `json:"implicit"`
	Password          PasswordOAuthFlow          `json:"password"`
}

// OpenID Connect security scheme configuration.
type OpenIdConnectSecurityScheme struct {
	Description      string `json:"description"`
	Openidconnecturl string `json:"openIdConnectUrl"`
	Type             string `json:"type"`
}

// Represents a part of a message, which can be text, a file, or structured data.
type Part struct {
}

// Base properties common to all message parts.
type PartBase struct {
	Metadata map[string]interface{} `json:"metadata"`
}

// Configuration details for a supported OAuth Flow
type PasswordOAuthFlow struct {
	Refreshurl string                 `json:"refreshUrl"`
	Scopes     map[string]interface{} `json:"scopes"`
	Tokenurl   string                 `json:"tokenUrl"`
}

// Defines authentication details for push notifications.
type PushNotificationAuthenticationInfo struct {
	Credentials string   `json:"credentials"`
	Schemes     []string `json:"schemes"`
}

// Configuration for setting up push notifications for task updates.
type PushNotificationConfig struct {
	Authentication PushNotificationAuthenticationInfo `json:"authentication"`
	ID             string                             `json:"id"`
	Token          string                             `json:"token"`
	URL            string                             `json:"url"`
}

// A2A specific error indicating the agent does not support push notifications.
type PushNotificationNotSupportedError struct {
	Code    int         `json:"code"`
	Data    interface{} `json:"data"`
	Message string      `json:"message"`
}

// Mirrors the OpenAPI Security Scheme Object
// (https://swagger.io/specification/#security-scheme-object)
type SecurityScheme struct {
}

// Base properties shared by all security schemes.
type SecuritySchemeBase struct {
	Description string `json:"description"`
}

// JSON-RPC request model for the 'message/send' method.
type SendMessageRequest struct {
	ID      interface{}       `json:"id"`
	JSONRPC string            `json:"jsonrpc"`
	Method  string            `json:"method"`
	Params  MessageSendParams `json:"params"`
}

// JSON-RPC response model for the 'message/send' method.
type SendMessageResponse struct {
}

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
type SendStreamingMessageResponse struct {
}

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
type SetTaskPushNotificationConfigResponse struct {
}

// JSON-RPC success response model for the 'tasks/pushNotificationConfig/set' method.
type SetTaskPushNotificationConfigSuccessResponse struct {
	ID      interface{}                `json:"id"`
	JSONRPC string                     `json:"jsonrpc"`
	Result  TaskPushNotificationConfig `json:"result"`
}

type Task struct {
	Artifacts []Artifact             `json:"artifacts"`
	Contextid string                 `json:"contextId"`
	History   []Message              `json:"history"`
	ID        string                 `json:"id"`
	Kind      string                 `json:"kind"`
	Metadata  map[string]interface{} `json:"metadata"`
	Status    TaskStatus             `json:"status"`
}

// Sent by server during sendStream or subscribe requests
type TaskArtifactUpdateEvent struct {
	Append    bool                   `json:"append"`
	Artifact  Artifact               `json:"artifact"`
	Contextid string                 `json:"contextId"`
	Kind      string                 `json:"kind"`
	Lastchunk bool                   `json:"lastChunk"`
	Metadata  map[string]interface{} `json:"metadata"`
	Taskid    string                 `json:"taskId"`
}

// Parameters containing only a task ID, used for simple task operations.
type TaskIdParams struct {
	ID       string                 `json:"id"`
	Metadata map[string]interface{} `json:"metadata"`
}

// A2A specific error indicating the task is in a state where it cannot be canceled.
type TaskNotCancelableError struct {
	Code    int         `json:"code"`
	Data    interface{} `json:"data"`
	Message string      `json:"message"`
}

// A2A specific error indicating the requested task ID was not found.
type TaskNotFoundError struct {
	Code    int         `json:"code"`
	Data    interface{} `json:"data"`
	Message string      `json:"message"`
}

// Parameters for setting or getting push notification configuration for a task
type TaskPushNotificationConfig struct {
	Pushnotificationconfig PushNotificationConfig `json:"pushNotificationConfig"`
	Taskid                 string                 `json:"taskId"`
}

// Parameters for querying a task, including optional history length.
type TaskQueryParams struct {
	Historylength int                    `json:"historyLength"`
	ID            string                 `json:"id"`
	Metadata      map[string]interface{} `json:"metadata"`
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
	Message   Message   `json:"message"`
	State     TaskState `json:"state"`
	Timestamp string    `json:"timestamp"`
}

// Sent by server during sendStream or subscribe requests
type TaskStatusUpdateEvent struct {
	Contextid string                 `json:"contextId"`
	Final     bool                   `json:"final"`
	Kind      string                 `json:"kind"`
	Metadata  map[string]interface{} `json:"metadata"`
	Status    TaskStatus             `json:"status"`
	Taskid    string                 `json:"taskId"`
}

// Represents a text segment within parts.
type TextPart struct {
	Kind     string                 `json:"kind"`
	Metadata map[string]interface{} `json:"metadata"`
	Text     string                 `json:"text"`
}

// A2A specific error indicating the requested operation is not supported by the agent.
type UnsupportedOperationError struct {
	Code    int         `json:"code"`
	Data    interface{} `json:"data"`
	Message string      `json:"message"`
}
