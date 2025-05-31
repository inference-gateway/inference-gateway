// Code generated from MCP schema. DO NOT EDIT.
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
	Audience []Role  `json:"audience"`
	Priority float64 `json:"priority"`
}

// Audio provided to or from an LLM.
type AudioContent struct {
	Annotations Annotations `json:"annotations"`
	Data        string      `json:"data"`
	Mimetype    string      `json:"mimeType"`
	Type        string      `json:"type"`
}

type BlobResourceContents struct {
	Blob     string `json:"blob"`
	Mimetype string `json:"mimeType"`
	URI      string `json:"uri"`
}

type BooleanSchema struct {
	Default     bool   `json:"default"`
	Description string `json:"description"`
	Title       string `json:"title"`
	Type        string `json:"type"`
}

// Used by the client to invoke a tool provided by the server.
type CallToolRequest struct {
	Method string                 `json:"method"`
	Params map[string]interface{} `json:"params"`
}

// The server's response to a tool call.
type CallToolResult struct {
	Meta              map[string]interface{} `json:"_meta"`
	Content           []interface{}          `json:"content"`
	Iserror           bool                   `json:"isError"`
	Structuredcontent map[string]interface{} `json:"structuredContent"`
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
	Elicitation  map[string]interface{} `json:"elicitation"`
	Experimental map[string]interface{} `json:"experimental"`
	Roots        map[string]interface{} `json:"roots"`
	Sampling     map[string]interface{} `json:"sampling"`
}

type ClientNotification struct {
}

type ClientRequest struct {
}

type ClientResult struct {
}

// A request from the client to the server, to ask for completion options.
type CompleteRequest struct {
	Method string                 `json:"method"`
	Params map[string]interface{} `json:"params"`
}

// The server's response to a completion/complete request
type CompleteResult struct {
	Meta       map[string]interface{} `json:"_meta"`
	Completion map[string]interface{} `json:"completion"`
}

// A request from the server to sample an LLM via the client. The client has full discretion over which model to select. The client should also inform the user before beginning sampling, to allow them to inspect the request (human in the loop) and decide whether to approve it.
type CreateMessageRequest struct {
	Method string                 `json:"method"`
	Params map[string]interface{} `json:"params"`
}

// The client's response to a sampling/create_message request from the server. The client should inform the user before returning the sampled message, to allow them to inspect the response (human in the loop) and decide whether to allow the server to see it.
type CreateMessageResult struct {
	Meta       map[string]interface{} `json:"_meta"`
	Content    interface{}            `json:"content"`
	Model      string                 `json:"model"`
	Role       Role                   `json:"role"`
	Stopreason string                 `json:"stopReason"`
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
	Meta    map[string]interface{} `json:"_meta"`
	Action  string                 `json:"action"`
	Content map[string]interface{} `json:"content"`
}

// The contents of a resource, embedded into a prompt or tool call result.
//
// It is up to the client how best to render embedded resources for the benefit
// of the LLM and/or the user.
type EmbeddedResource struct {
	Annotations Annotations `json:"annotations"`
	Resource    interface{} `json:"resource"`
	Type        string      `json:"type"`
}

type EmptyResult struct {
}

type EnumSchema struct {
	Description string   `json:"description"`
	Enum        []string `json:"enum"`
	Enumnames   []string `json:"enumNames"`
	Title       string   `json:"title"`
	Type        string   `json:"type"`
}

// Used by the client to get a prompt provided by the server.
type GetPromptRequest struct {
	Method string                 `json:"method"`
	Params map[string]interface{} `json:"params"`
}

// The server's response to a prompts/get request from the client.
type GetPromptResult struct {
	Meta        map[string]interface{} `json:"_meta"`
	Description string                 `json:"description"`
	Messages    []PromptMessage        `json:"messages"`
}

// An image provided to or from an LLM.
type ImageContent struct {
	Annotations Annotations `json:"annotations"`
	Data        string      `json:"data"`
	Mimetype    string      `json:"mimeType"`
	Type        string      `json:"type"`
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
	Meta            map[string]interface{} `json:"_meta"`
	Capabilities    ServerCapabilities     `json:"capabilities"`
	Instructions    string                 `json:"instructions"`
	Protocolversion string                 `json:"protocolVersion"`
	Serverinfo      Implementation         `json:"serverInfo"`
}

// This notification is sent from the client to the server after initialization has finished.
type InitializedNotification struct {
	Method string                 `json:"method"`
	Params map[string]interface{} `json:"params"`
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
type JSONRPCMessage struct {
}

// A notification which does not expect a response.
type JSONRPCNotification struct {
	JSONRPC string                 `json:"jsonrpc"`
	Method  string                 `json:"method"`
	Params  map[string]interface{} `json:"params"`
}

// A request that expects a response.
type JSONRPCRequest struct {
	ID      RequestId              `json:"id"`
	JSONRPC string                 `json:"jsonrpc"`
	Method  string                 `json:"method"`
	Params  map[string]interface{} `json:"params"`
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
	Params map[string]interface{} `json:"params"`
}

// The server's response to a prompts/list request from the client.
type ListPromptsResult struct {
	Meta       map[string]interface{} `json:"_meta"`
	Nextcursor string                 `json:"nextCursor"`
	Prompts    []Prompt               `json:"prompts"`
}

// Sent from the client to request a list of resource templates the server has.
type ListResourceTemplatesRequest struct {
	Method string                 `json:"method"`
	Params map[string]interface{} `json:"params"`
}

// The server's response to a resources/templates/list request from the client.
type ListResourceTemplatesResult struct {
	Meta              map[string]interface{} `json:"_meta"`
	Nextcursor        string                 `json:"nextCursor"`
	Resourcetemplates []ResourceTemplate     `json:"resourceTemplates"`
}

// Sent from the client to request a list of resources the server has.
type ListResourcesRequest struct {
	Method string                 `json:"method"`
	Params map[string]interface{} `json:"params"`
}

// The server's response to a resources/list request from the client.
type ListResourcesResult struct {
	Meta       map[string]interface{} `json:"_meta"`
	Nextcursor string                 `json:"nextCursor"`
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
	Params map[string]interface{} `json:"params"`
}

// The client's response to a roots/list request from the server.
// This result contains an array of Root objects, each representing a root directory
// or file that the server can operate on.
type ListRootsResult struct {
	Meta  map[string]interface{} `json:"_meta"`
	Roots []Root                 `json:"roots"`
}

// Sent from the client to request a list of tools the server has.
type ListToolsRequest struct {
	Method string                 `json:"method"`
	Params map[string]interface{} `json:"params"`
}

// The server's response to a tools/list request from the client.
type ListToolsResult struct {
	Meta       map[string]interface{} `json:"_meta"`
	Nextcursor string                 `json:"nextCursor"`
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
	Name string `json:"name"`
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
	Costpriority         float64     `json:"costPriority"`
	Hints                []ModelHint `json:"hints"`
	Intelligencepriority float64     `json:"intelligencePriority"`
	Speedpriority        float64     `json:"speedPriority"`
}

type Notification struct {
	Method string                 `json:"method"`
	Params map[string]interface{} `json:"params"`
}

type NumberSchema struct {
	Description string `json:"description"`
	Maximum     int    `json:"maximum"`
	Minimum     int    `json:"minimum"`
	Title       string `json:"title"`
	Type        string `json:"type"`
}

type PaginatedRequest struct {
	Method string                 `json:"method"`
	Params map[string]interface{} `json:"params"`
}

type PaginatedResult struct {
	Meta       map[string]interface{} `json:"_meta"`
	Nextcursor string                 `json:"nextCursor"`
}

// A ping, issued by either the server or the client, to check that the other party is still alive. The receiver must promptly respond, or else may be disconnected.
type PingRequest struct {
	Method string                 `json:"method"`
	Params map[string]interface{} `json:"params"`
}

// Restricted schema definitions that only allow primitive types
// without nested objects or arrays.
type PrimitiveSchemaDefinition struct {
}

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
	Arguments   []PromptArgument `json:"arguments"`
	Description string           `json:"description"`
	Name        string           `json:"name"`
}

// Describes an argument that a prompt can accept.
type PromptArgument struct {
	Description string `json:"description"`
	Name        string `json:"name"`
	Required    bool   `json:"required"`
}

// An optional notification from the server to the client, informing it that the list of prompts it offers has changed. This may be issued by servers without any previous subscription from the client.
type PromptListChangedNotification struct {
	Method string                 `json:"method"`
	Params map[string]interface{} `json:"params"`
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
	Meta     map[string]interface{} `json:"_meta"`
	Contents []interface{}          `json:"contents"`
}

type Request struct {
	Method string                 `json:"method"`
	Params map[string]interface{} `json:"params"`
}

// A uniquely identifying ID for a request in JSON-RPC.
type RequestId struct {
}

// A known resource that the server is capable of reading.
type Resource struct {
	Annotations Annotations `json:"annotations"`
	Description string      `json:"description"`
	Mimetype    string      `json:"mimeType"`
	Name        string      `json:"name"`
	Size        int         `json:"size"`
	URI         string      `json:"uri"`
}

// The contents of a specific resource or sub-resource.
type ResourceContents struct {
	Mimetype string `json:"mimeType"`
	URI      string `json:"uri"`
}

// An optional notification from the server to the client, informing it that the list of resources it can read from has changed. This may be issued by servers without any previous subscription from the client.
type ResourceListChangedNotification struct {
	Method string                 `json:"method"`
	Params map[string]interface{} `json:"params"`
}

// A reference to a resource or resource template definition.
type ResourceReference struct {
	Type string `json:"type"`
	URI  string `json:"uri"`
}

// A template description for resources available on the server.
type ResourceTemplate struct {
	Annotations Annotations `json:"annotations"`
	Description string      `json:"description"`
	Mimetype    string      `json:"mimeType"`
	Name        string      `json:"name"`
	Uritemplate string      `json:"uriTemplate"`
}

// A notification from the server to the client, informing it that a resource has changed and may need to be read again. This should only be sent if the client previously sent a resources/subscribe request.
type ResourceUpdatedNotification struct {
	Method string                 `json:"method"`
	Params map[string]interface{} `json:"params"`
}

type Result struct {
	Meta map[string]interface{} `json:"_meta"`
}

// Represents a root directory or file that the server can operate on.
type Root struct {
	Name string `json:"name"`
	URI  string `json:"uri"`
}

// A notification from the client to the server, informing it that the list of roots has changed.
// This notification should be sent whenever the client adds, removes, or modifies any root.
// The server should then request an updated list of roots using the ListRootsRequest.
type RootsListChangedNotification struct {
	Method string                 `json:"method"`
	Params map[string]interface{} `json:"params"`
}

// Describes a message issued to or received from an LLM API.
type SamplingMessage struct {
	Content interface{} `json:"content"`
	Role    Role        `json:"role"`
}

// Capabilities that a server may support. Known capabilities are defined here, in this schema, but this is not a closed set: any server can define its own, additional capabilities.
type ServerCapabilities struct {
	Completions  map[string]interface{} `json:"completions"`
	Experimental map[string]interface{} `json:"experimental"`
	Logging      map[string]interface{} `json:"logging"`
	Prompts      map[string]interface{} `json:"prompts"`
	Resources    map[string]interface{} `json:"resources"`
	Tools        map[string]interface{} `json:"tools"`
}

type ServerNotification struct {
}

type ServerRequest struct {
}

type ServerResult struct {
}

// A request from the client to the server, to enable or adjust logging.
type SetLevelRequest struct {
	Method string                 `json:"method"`
	Params map[string]interface{} `json:"params"`
}

type StringSchema struct {
	Description string `json:"description"`
	Format      string `json:"format"`
	Maxlength   int    `json:"maxLength"`
	Minlength   int    `json:"minLength"`
	Title       string `json:"title"`
	Type        string `json:"type"`
}

// Sent from the client to request resources/updated notifications from the server whenever a particular resource changes.
type SubscribeRequest struct {
	Method string                 `json:"method"`
	Params map[string]interface{} `json:"params"`
}

// Text provided to or from an LLM.
type TextContent struct {
	Annotations Annotations `json:"annotations"`
	Text        string      `json:"text"`
	Type        string      `json:"type"`
}

type TextResourceContents struct {
	Mimetype string `json:"mimeType"`
	Text     string `json:"text"`
	URI      string `json:"uri"`
}

// Definition for a tool the client can call.
type Tool struct {
	Annotations  ToolAnnotations        `json:"annotations"`
	Description  string                 `json:"description"`
	Inputschema  map[string]interface{} `json:"inputSchema"`
	Name         string                 `json:"name"`
	Outputschema map[string]interface{} `json:"outputSchema"`
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
	Destructivehint bool   `json:"destructiveHint"`
	Idempotenthint  bool   `json:"idempotentHint"`
	Openworldhint   bool   `json:"openWorldHint"`
	Readonlyhint    bool   `json:"readOnlyHint"`
	Title           string `json:"title"`
}

// An optional notification from the server to the client, informing it that the list of tools it offers has changed. This may be issued by servers without any previous subscription from the client.
type ToolListChangedNotification struct {
	Method string                 `json:"method"`
	Params map[string]interface{} `json:"params"`
}

// Sent from the client to request cancellation of resources/updated notifications from the server. This should follow a previous resources/subscribe request.
type UnsubscribeRequest struct {
	Method string                 `json:"method"`
	Params map[string]interface{} `json:"params"`
}
