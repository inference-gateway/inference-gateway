// Code generated from JSON schema. DO NOT EDIT.
package mcp

// The severity of a log message.
//
// These map to syslog message severities, as specified in RFC-5424:
// https://datatracker.ietf.org/doc/html/rfc5424#section-6.2.1
type LoggingLevel string

// LoggingLevel enum values
const (
	LoggingLevelAlert     LoggingLevel = "alert"
	LoggingLevelCritical  LoggingLevel = "critical"
	LoggingLevelDebug     LoggingLevel = "debug"
	LoggingLevelEmergency LoggingLevel = "emergency"
	LoggingLevelError     LoggingLevel = "error"
	LoggingLevelInfo      LoggingLevel = "info"
	LoggingLevelNotice    LoggingLevel = "notice"
	LoggingLevelWarning   LoggingLevel = "warning"
)

// The sender or recipient of messages and data in a conversation.
type Role string

// Role enum values
const (
	RoleAssistant Role = "assistant"
	RoleUser      Role = "user"
)

// Optional annotations for the client. The client can use annotations to inform how objects are used or displayed
type Annotations struct {
	Audience []Role   `json:"audience,omitempty"`
	Priority *float64 `json:"priority,omitempty"`
}

// Audio provided to or from an LLM.
type AudioContent struct {
	Annotations *Annotations `json:"annotations,omitempty"`
	Data        []byte       `json:"data"`
	MIMEType    string       `json:"mimeType"`
	Type        string       `json:"type"`
}

type BlobResourceContents struct {
	Blob     []byte  `json:"blob"`
	MIMEType *string `json:"mimeType,omitempty"`
	URI      string  `json:"uri"`
}

type BooleanSchema struct {
	Default     *bool   `json:"default,omitempty"`
	Description *string `json:"description,omitempty"`
	Title       *string `json:"title,omitempty"`
	Type        string  `json:"type"`
}

// Used by the client to invoke a tool provided by the server.
type CallToolRequest struct {
	Method string                 `json:"method"`
	Params map[string]interface{} `json:"params"`
}

// The server's response to a tool call.
type CallToolResult struct {
	Meta              map[string]interface{} `json:"_meta,omitempty"`
	Content           []interface{}          `json:"content"`
	IsError           *bool                  `json:"isError,omitempty"`
	StructuredContent map[string]interface{} `json:"structuredContent,omitempty"`
}

// This notification can be sent by either side to indicate that it is cancelling a previously-issued request.
//
// The request SHOULD still be in-flight, but due to communication latency, it is always possible that this notification MAY arrive after the request has already finished.
//
// This notification indicates that the result will be unused, so any associated processing SHOULD cease.
//
// A client MUST NOT attempt to cancel its `initialize` request.
type CancelledNotification struct {
	Method string                 `json:"method"`
	Params map[string]interface{} `json:"params"`
}

// Capabilities a client may support. Known capabilities are defined here, in this schema, but this is not a closed set: any client can define its own, additional capabilities.
type ClientCapabilities struct {
	Elicitation  map[string]interface{}            `json:"elicitation,omitempty"`
	Experimental map[string]map[string]interface{} `json:"experimental,omitempty"`
	Roots        map[string]interface{}            `json:"roots,omitempty"`
	Sampling     map[string]interface{}            `json:"sampling,omitempty"`
}

type ClientNotification interface{}

type ClientRequest interface{}

type ClientResult interface{}

// A request from the client to the server, to ask for completion options.
type CompleteRequest struct {
	Method string                 `json:"method"`
	Params map[string]interface{} `json:"params"`
}

// The server's response to a completion/complete request
type CompleteResult struct {
	Meta       map[string]interface{} `json:"_meta,omitempty"`
	Completion map[string]interface{} `json:"completion"`
}

// A request from the server to sample an LLM via the client. The client has full discretion over which model to select. The client should also inform the user before beginning sampling, to allow them to inspect the request (human in the loop) and decide whether to approve it.
type CreateMessageRequest struct {
	Method string                 `json:"method"`
	Params map[string]interface{} `json:"params"`
}

// The client's response to a sampling/create_message request from the server. The client should inform the user before returning the sampled message, to allow them to inspect the response (human in the loop) and decide whether to allow the server to see it.
type CreateMessageResult struct {
	Meta       map[string]interface{} `json:"_meta,omitempty"`
	Content    interface{}            `json:"content"`
	Model      string                 `json:"model"`
	Role       Role                   `json:"role"`
	StopReason *string                `json:"stopReason,omitempty"`
}

// An opaque token used to represent a cursor for pagination.
type Cursor struct {
}

// A request from the server to elicit additional information from the user via the client.
type ElicitRequest struct {
	Method string                 `json:"method"`
	Params map[string]interface{} `json:"params"`
}

// The client's response to an elicitation request.
type ElicitResult struct {
	Meta    map[string]interface{} `json:"_meta,omitempty"`
	Action  string                 `json:"action"`
	Content map[string]interface{} `json:"content,omitempty"`
}

// The contents of a resource, embedded into a prompt or tool call result.
//
// It is up to the client how best to render embedded resources for the benefit
// of the LLM and/or the user.
type EmbeddedResource struct {
	Annotations *Annotations `json:"annotations,omitempty"`
	Resource    interface{}  `json:"resource"`
	Type        string       `json:"type"`
}

type EmptyResult struct {
}

type EnumSchema struct {
	Description *string  `json:"description,omitempty"`
	Enum        []string `json:"enum"`
	EnumNames   []string `json:"enumNames,omitempty"`
	Title       *string  `json:"title,omitempty"`
	Type        string   `json:"type"`
}

// Used by the client to get a prompt provided by the server.
type GetPromptRequest struct {
	Method string                 `json:"method"`
	Params map[string]interface{} `json:"params"`
}

// The server's response to a prompts/get request from the client.
type GetPromptResult struct {
	Meta        map[string]interface{} `json:"_meta,omitempty"`
	Description *string                `json:"description,omitempty"`
	Messages    []PromptMessage        `json:"messages"`
}

// An image provided to or from an LLM.
type ImageContent struct {
	Annotations *Annotations `json:"annotations,omitempty"`
	Data        []byte       `json:"data"`
	MIMEType    string       `json:"mimeType"`
	Type        string       `json:"type"`
}

// Describes the name and version of an MCP implementation.
type Implementation struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// This request is sent from the client to the server when it first connects, asking it to begin initialization.
type InitializeRequest struct {
	Method string                 `json:"method"`
	Params map[string]interface{} `json:"params"`
}

// After receiving an initialize request from the client, the server sends this response.
type InitializeResult struct {
	Meta            map[string]interface{} `json:"_meta,omitempty"`
	Capabilities    ServerCapabilities     `json:"capabilities"`
	Instructions    *string                `json:"instructions,omitempty"`
	ProtocolVersion string                 `json:"protocolVersion"`
	ServerInfo      Implementation         `json:"serverInfo"`
}

// This notification is sent from the client to the server after initialization has finished.
type InitializedNotification struct {
	Method string                 `json:"method"`
	Params map[string]interface{} `json:"params,omitempty"`
}

// A JSON-RPC batch request, as described in https://www.jsonrpc.org/specification#batch.
type JSONRPCBatchRequest struct {
}

// A JSON-RPC batch response, as described in https://www.jsonrpc.org/specification#batch.
type JSONRPCBatchResponse struct {
}

// A response to a request that indicates an error occurred.
type JSONRPCError struct {
	Error   map[string]interface{} `json:"error"`
	ID      RequestId              `json:"id"`
	JSONRPC string                 `json:"jsonrpc"`
}

// Refers to any valid JSON-RPC object that can be decoded off the wire, or encoded to be sent.
type JSONRPCMessage interface{}

// A notification which does not expect a response.
type JSONRPCNotification struct {
	JSONRPC string                 `json:"jsonrpc"`
	Method  string                 `json:"method"`
	Params  map[string]interface{} `json:"params,omitempty"`
}

// A request that expects a response.
type JSONRPCRequest struct {
	ID      RequestId              `json:"id"`
	JSONRPC string                 `json:"jsonrpc"`
	Method  string                 `json:"method"`
	Params  map[string]interface{} `json:"params,omitempty"`
}

// A successful (non-error) response to a request.
type JSONRPCResponse struct {
	ID      RequestId `json:"id"`
	JSONRPC string    `json:"jsonrpc"`
	Result  Result    `json:"result"`
}

// Sent from the client to request a list of prompts and prompt templates the server has.
type ListPromptsRequest struct {
	Method string                 `json:"method"`
	Params map[string]interface{} `json:"params,omitempty"`
}

// The server's response to a prompts/list request from the client.
type ListPromptsResult struct {
	Meta       map[string]interface{} `json:"_meta,omitempty"`
	NextCursor *string                `json:"nextCursor,omitempty"`
	Prompts    []Prompt               `json:"prompts"`
}

// Sent from the client to request a list of resource templates the server has.
type ListResourceTemplatesRequest struct {
	Method string                 `json:"method"`
	Params map[string]interface{} `json:"params,omitempty"`
}

// The server's response to a resources/templates/list request from the client.
type ListResourceTemplatesResult struct {
	Meta              map[string]interface{} `json:"_meta,omitempty"`
	NextCursor        *string                `json:"nextCursor,omitempty"`
	ResourceTemplates []ResourceTemplate     `json:"resourceTemplates"`
}

// Sent from the client to request a list of resources the server has.
type ListResourcesRequest struct {
	Method string                 `json:"method"`
	Params map[string]interface{} `json:"params,omitempty"`
}

// The server's response to a resources/list request from the client.
type ListResourcesResult struct {
	Meta       map[string]interface{} `json:"_meta,omitempty"`
	NextCursor *string                `json:"nextCursor,omitempty"`
	Resources  []Resource             `json:"resources"`
}

// Sent from the server to request a list of root URIs from the client. Roots allow
// servers to ask for specific directories or files to operate on. A common example
// for roots is providing a set of repositories or directories a server should operate
// on.
//
// This request is typically used when the server needs to understand the file system
// structure or access specific locations that the client has permission to read from.
type ListRootsRequest struct {
	Method string                 `json:"method"`
	Params map[string]interface{} `json:"params,omitempty"`
}

// The client's response to a roots/list request from the server.
// This result contains an array of Root objects, each representing a root directory
// or file that the server can operate on.
type ListRootsResult struct {
	Meta  map[string]interface{} `json:"_meta,omitempty"`
	Roots []Root                 `json:"roots"`
}

// Sent from the client to request a list of tools the server has.
type ListToolsRequest struct {
	Method string                 `json:"method"`
	Params map[string]interface{} `json:"params,omitempty"`
}

// The server's response to a tools/list request from the client.
type ListToolsResult struct {
	Meta       map[string]interface{} `json:"_meta,omitempty"`
	NextCursor *string                `json:"nextCursor,omitempty"`
	Tools      []Tool                 `json:"tools"`
}

// Notification of a log message passed from server to client. If no logging/setLevel request has been sent from the client, the server MAY decide which messages to send automatically.
type LoggingMessageNotification struct {
	Method string                 `json:"method"`
	Params map[string]interface{} `json:"params"`
}

// Hints to use for model selection.
//
// Keys not declared here are currently left unspecified by the spec and are up
// to the client to interpret.
type ModelHint struct {
	Name *string `json:"name,omitempty"`
}

// The server's preferences for model selection, requested of the client during sampling.
//
// Because LLMs can vary along multiple dimensions, choosing the "best" model is
// rarely straightforward.  Different models excel in different areas—some are
// faster but less capable, others are more capable but more expensive, and so
// on. This interface allows servers to express their priorities across multiple
// dimensions to help clients make an appropriate selection for their use case.
//
// These preferences are always advisory. The client MAY ignore them. It is also
// up to the client to decide how to interpret these preferences and how to
// balance them against other considerations.
type ModelPreferences struct {
	CostPriority         *float64    `json:"costPriority,omitempty"`
	Hints                []ModelHint `json:"hints,omitempty"`
	IntelligencePriority *float64    `json:"intelligencePriority,omitempty"`
	SpeedPriority        *float64    `json:"speedPriority,omitempty"`
}

type Notification struct {
	Method string                 `json:"method"`
	Params map[string]interface{} `json:"params,omitempty"`
}

type NumberSchema struct {
	Description *string `json:"description,omitempty"`
	Maximum     *int    `json:"maximum,omitempty"`
	Minimum     *int    `json:"minimum,omitempty"`
	Title       *string `json:"title,omitempty"`
	Type        string  `json:"type"`
}

type PaginatedRequest struct {
	Method string                 `json:"method"`
	Params map[string]interface{} `json:"params,omitempty"`
}

type PaginatedResult struct {
	Meta       map[string]interface{} `json:"_meta,omitempty"`
	NextCursor *string                `json:"nextCursor,omitempty"`
}

// A ping, issued by either the server or the client, to check that the other party is still alive. The receiver must promptly respond, or else may be disconnected.
type PingRequest struct {
	Method string                 `json:"method"`
	Params map[string]interface{} `json:"params,omitempty"`
}

// Restricted schema definitions that only allow primitive types
// without nested objects or arrays.
type PrimitiveSchemaDefinition interface{}

// An out-of-band notification used to inform the receiver of a progress update for a long-running request.
type ProgressNotification struct {
	Method string                 `json:"method"`
	Params map[string]interface{} `json:"params"`
}

// A progress token, used to associate progress notifications with the original request.
type ProgressToken struct {
}

// A prompt or prompt template that the server offers.
type Prompt struct {
	Arguments   []PromptArgument `json:"arguments,omitempty"`
	Description *string          `json:"description,omitempty"`
	Name        string           `json:"name"`
}

// Describes an argument that a prompt can accept.
type PromptArgument struct {
	Description *string `json:"description,omitempty"`
	Name        string  `json:"name"`
	Required    *bool   `json:"required,omitempty"`
}

// An optional notification from the server to the client, informing it that the list of prompts it offers has changed. This may be issued by servers without any previous subscription from the client.
type PromptListChangedNotification struct {
	Method string                 `json:"method"`
	Params map[string]interface{} `json:"params,omitempty"`
}

// Describes a message returned as part of a prompt.
//
// This is similar to `SamplingMessage`, but also supports the embedding of
// resources from the MCP server.
type PromptMessage struct {
	Content interface{} `json:"content"`
	Role    Role        `json:"role"`
}

// Identifies a prompt.
type PromptReference struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

// Sent from the client to the server, to read a specific resource URI.
type ReadResourceRequest struct {
	Method string                 `json:"method"`
	Params map[string]interface{} `json:"params"`
}

// The server's response to a resources/read request from the client.
type ReadResourceResult struct {
	Meta     map[string]interface{} `json:"_meta,omitempty"`
	Contents []interface{}          `json:"contents"`
}

type Request struct {
	Method string                 `json:"method"`
	Params map[string]interface{} `json:"params,omitempty"`
}

// A uniquely identifying ID for a request in JSON-RPC.
type RequestId struct {
}

// A known resource that the server is capable of reading.
type Resource struct {
	Annotations *Annotations `json:"annotations,omitempty"`
	Description *string      `json:"description,omitempty"`
	MIMEType    *string      `json:"mimeType,omitempty"`
	Name        string       `json:"name"`
	Size        *int         `json:"size,omitempty"`
	URI         string       `json:"uri"`
}

// The contents of a specific resource or sub-resource.
type ResourceContents struct {
	MIMEType *string `json:"mimeType,omitempty"`
	URI      string  `json:"uri"`
}

// An optional notification from the server to the client, informing it that the list of resources it can read from has changed. This may be issued by servers without any previous subscription from the client.
type ResourceListChangedNotification struct {
	Method string                 `json:"method"`
	Params map[string]interface{} `json:"params,omitempty"`
}

// A reference to a resource or resource template definition.
type ResourceReference struct {
	Type string `json:"type"`
	URI  string `json:"uri"`
}

// A template description for resources available on the server.
type ResourceTemplate struct {
	Annotations *Annotations `json:"annotations,omitempty"`
	Description *string      `json:"description,omitempty"`
	MIMEType    *string      `json:"mimeType,omitempty"`
	Name        string       `json:"name"`
	URITemplate string       `json:"uriTemplate"`
}

// A notification from the server to the client, informing it that a resource has changed and may need to be read again. This should only be sent if the client previously sent a resources/subscribe request.
type ResourceUpdatedNotification struct {
	Method string                 `json:"method"`
	Params map[string]interface{} `json:"params"`
}

type Result struct {
	Meta map[string]interface{} `json:"_meta,omitempty"`
}

// Represents a root directory or file that the server can operate on.
type Root struct {
	Name *string `json:"name,omitempty"`
	URI  string  `json:"uri"`
}

// A notification from the client to the server, informing it that the list of roots has changed.
// This notification should be sent whenever the client adds, removes, or modifies any root.
// The server should then request an updated list of roots using the ListRootsRequest.
type RootsListChangedNotification struct {
	Method string                 `json:"method"`
	Params map[string]interface{} `json:"params,omitempty"`
}

// Describes a message issued to or received from an LLM API.
type SamplingMessage struct {
	Content interface{} `json:"content"`
	Role    Role        `json:"role"`
}

// Capabilities that a server may support. Known capabilities are defined here, in this schema, but this is not a closed set: any server can define its own, additional capabilities.
type ServerCapabilities struct {
	Completions  map[string]interface{}            `json:"completions,omitempty"`
	Experimental map[string]map[string]interface{} `json:"experimental,omitempty"`
	Logging      map[string]interface{}            `json:"logging,omitempty"`
	Prompts      map[string]interface{}            `json:"prompts,omitempty"`
	Resources    map[string]interface{}            `json:"resources,omitempty"`
	Tools        map[string]interface{}            `json:"tools,omitempty"`
}

type ServerNotification interface{}

type ServerRequest interface{}

type ServerResult interface{}

// A request from the client to the server, to enable or adjust logging.
type SetLevelRequest struct {
	Method string                 `json:"method"`
	Params map[string]interface{} `json:"params"`
}

type StringSchema struct {
	Description *string `json:"description,omitempty"`
	Format      *string `json:"format,omitempty"`
	MaxLength   *int    `json:"maxLength,omitempty"`
	MinLength   *int    `json:"minLength,omitempty"`
	Title       *string `json:"title,omitempty"`
	Type        string  `json:"type"`
}

// Sent from the client to request resources/updated notifications from the server whenever a particular resource changes.
type SubscribeRequest struct {
	Method string                 `json:"method"`
	Params map[string]interface{} `json:"params"`
}

// Text provided to or from an LLM.
type TextContent struct {
	Annotations *Annotations `json:"annotations,omitempty"`
	Text        string       `json:"text"`
	Type        string       `json:"type"`
}

type TextResourceContents struct {
	MIMEType *string `json:"mimeType,omitempty"`
	Text     string  `json:"text"`
	URI      string  `json:"uri"`
}

// Definition for a tool the client can call.
type Tool struct {
	Annotations  *ToolAnnotations       `json:"annotations,omitempty"`
	Description  *string                `json:"description,omitempty"`
	InputSchema  map[string]interface{} `json:"inputSchema"`
	Name         string                 `json:"name"`
	OutputSchema map[string]interface{} `json:"outputSchema,omitempty"`
}

// Additional properties describing a Tool to clients.
//
// NOTE: all properties in ToolAnnotations are **hints**.
// They are not guaranteed to provide a faithful description of
// tool behavior (including descriptive properties like `title`).
//
// Clients should never make tool use decisions based on ToolAnnotations
// received from untrusted servers.
type ToolAnnotations struct {
	DestructiveHint *bool   `json:"destructiveHint,omitempty"`
	IdempotentHint  *bool   `json:"idempotentHint,omitempty"`
	OpenWorldHint   *bool   `json:"openWorldHint,omitempty"`
	ReadOnlyHint    *bool   `json:"readOnlyHint,omitempty"`
	Title           *string `json:"title,omitempty"`
}

// An optional notification from the server to the client, informing it that the list of tools it offers has changed. This may be issued by servers without any previous subscription from the client.
type ToolListChangedNotification struct {
	Method string                 `json:"method"`
	Params map[string]interface{} `json:"params,omitempty"`
}

// Sent from the client to request cancellation of resources/updated notifications from the server. This should follow a previous resources/subscribe request.
type UnsubscribeRequest struct {
	Method string                 `json:"method"`
	Params map[string]interface{} `json:"params"`
}
