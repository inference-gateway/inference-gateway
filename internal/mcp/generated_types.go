// Code generated from JSON schema. DO NOT EDIT.
package mcp

// The user action in response to the elicitation.
// - `"accept"`: User submitted the form/confirmed the action
// - `"decline"`: User explicitly declined the action
// - `"cancel"`: User dismissed without making an explicit choice
type Action string

// Action enum values
const (
	ActionAccept  Action = "accept"
	ActionCancel  Action = "cancel"
	ActionDecline Action = "decline"
)

// Indicates the intended scope of the cached response, analogous to HTTP
// `Cache-Control: public` vs `Cache-Control: private`.
//
// - `"public"`: The response does not contain user-specific data. Any
// client or intermediary (e.g., shared gateway, caching proxy) MAY cache
// the response and serve it across authorization contexts.
// - `"private"`: The response MAY be cached and reused only within the
// same authorization context. Caches MUST NOT be shared across
// authorization contexts (e.g., a different access token requires a
// different cache).
type CacheScope string

// CacheScope enum values
const (
	CacheScopePrivate CacheScope = "private"
	CacheScopePublic  CacheScope = "public"
)

type Format string

// Format enum values
const (
	FormatDate     Format = "date"
	FormatDateTime Format = "date-time"
	FormatEmail    Format = "email"
	FormatURI      Format = "uri"
)

// A request to include context from one or more MCP servers (including the caller), to be attached to the prompt.
// The client MAY ignore this request.
//
// Default is `"none"`. The values `"thisServer"` and `"allServers"` are deprecated (SEP-2596): servers SHOULD
// omit this field or use `"none"`, and SHOULD only use the deprecated values if the client declares
// {@link ClientCapabilities.sampling.context}.
type IncludeContext string

// IncludeContext enum values
const (
	IncludeContextAllServers IncludeContext = "allServers"
	IncludeContextNone       IncludeContext = "none"
	IncludeContextThisServer IncludeContext = "thisServer"
)

// Controls the tool use ability of the model:
// - `"auto"`: Model decides whether to use tools (default)
// - `"required"`: Model MUST use at least one tool before completing
// - `"none"`: Model MUST NOT use any tools
type Mode string

// Mode enum values
const (
	ModeAuto     Mode = "auto"
	ModeNone     Mode = "none"
	ModeRequired Mode = "required"
)

// Optional specifier for the theme this icon is designed for. `"light"` indicates
// the icon is designed to be used with a light background, and `"dark"` indicates
// the icon is designed to be used with a dark background.
//
// If not provided, the client should assume the icon can be used with any theme.
type Theme string

// Theme enum values
const (
	ThemeDark  Theme = "dark"
	ThemeLight Theme = "light"
)

type Type string

// Type enum values
const (
	TypeInteger Type = "integer"
	TypeNumber  Type = "number"
)

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
	Audience     []Role   `json:"audience,omitempty"`
	LastModified *string  `json:"lastModified,omitempty"`
	Priority     *float64 `json:"priority,omitempty"`
}

// Audio provided to or from an LLM.
type AudioContent struct {
	Meta        *MetaObject  `json:"_meta,omitempty"`
	Annotations *Annotations `json:"annotations,omitempty"`
	Data        []byte       `json:"data"`
	MIMEType    string       `json:"mimeType"`
	Type        string       `json:"type"`
}

// Base interface for metadata with name (identifier) and title (display name) properties.
type BaseMetadata struct {
	Name  string  `json:"name"`
	Title *string `json:"title,omitempty"`
}

type BlobResourceContents struct {
	Meta     *MetaObject `json:"_meta,omitempty"`
	Blob     []byte      `json:"blob"`
	MIMEType *string     `json:"mimeType,omitempty"`
	URI      string      `json:"uri"`
}

type BooleanSchema struct {
	Default     *bool   `json:"default,omitempty"`
	Description *string `json:"description,omitempty"`
	Title       *string `json:"title,omitempty"`
	Type        string  `json:"type"`
}

// A result that supports a time-to-live (TTL) hint for client-side caching.
type CacheableResult struct {
	Meta       *MetaObject `json:"_meta,omitempty"`
	CacheScope CacheScope  `json:"cacheScope"`
	ResultType string      `json:"resultType"`
	TtlMs      int         `json:"ttlMs"`
}

// Used by the client to invoke a tool provided by the server.
type CallToolRequest struct {
	ID      RequestId             `json:"id"`
	JSONRPC string                `json:"jsonrpc"`
	Method  string                `json:"method"`
	Params  CallToolRequestParams `json:"params"`
}

// Parameters for a `tools/call` request.
type CallToolRequestParams struct {
	Meta           RequestMetaObject `json:"_meta"`
	Arguments      map[string]any    `json:"arguments,omitempty"`
	InputResponses *InputResponses   `json:"inputResponses,omitempty"`
	Name           string            `json:"name"`
	RequestState   *string           `json:"requestState,omitempty"`
}

// The result returned by the server for a {@link CallToolRequesttools/call} request.
type CallToolResult struct {
	Meta              *MetaObject    `json:"_meta,omitempty"`
	Content           []ContentBlock `json:"content"`
	IsError           *bool          `json:"isError,omitempty"`
	ResultType        string         `json:"resultType"`
	StructuredContent *any           `json:"structuredContent,omitempty"`
}

// A successful response from the server for a {@link CallToolRequesttools/call} request.
type CallToolResultResponse struct {
	ID      RequestId `json:"id"`
	JSONRPC string    `json:"jsonrpc"`
	Result  any       `json:"result"`
}

// This notification is sent by the client to indicate that it is cancelling a request it previously issued.
//
// On stdio, the server also sends this notification, solely to terminate a {@link SubscriptionsListenRequestsubscriptions/listen} stream: it references the ID of the `subscriptions/listen` request that opened the stream. Servers MUST NOT use this notification to cancel any other request.
//
// The request SHOULD still be in-flight, but due to communication latency, it is always possible that this notification MAY arrive after the request has already finished.
//
// This notification indicates that the result will be unused, so any associated processing SHOULD cease.
type CancelledNotification struct {
	JSONRPC string                      `json:"jsonrpc"`
	Method  string                      `json:"method"`
	Params  CancelledNotificationParams `json:"params"`
}

// Parameters for a `notifications/cancelled` notification.
type CancelledNotificationParams struct {
	Meta      *NotificationMetaObject `json:"_meta,omitempty"`
	Reason    *string                 `json:"reason,omitempty"`
	RequestID RequestId               `json:"requestId"`
}

// Capabilities a client may support. Known capabilities are defined here, in this schema, but this is not a closed set: any client can define its own, additional capabilities.
type ClientCapabilities struct {
	Elicitation  map[string]any        `json:"elicitation,omitempty"`
	Experimental map[string]JSONObject `json:"experimental,omitempty"`
	Extensions   map[string]JSONObject `json:"extensions,omitempty"`
	Roots        map[string]any        `json:"roots,omitempty"`
	Sampling     map[string]any        `json:"sampling,omitempty"`
}

// This notification is sent by the client to indicate that it is cancelling a request it previously issued.
//
// On stdio, the server also sends this notification, solely to terminate a {@link SubscriptionsListenRequestsubscriptions/listen} stream: it references the ID of the `subscriptions/listen` request that opened the stream. Servers MUST NOT use this notification to cancel any other request.
//
// The request SHOULD still be in-flight, but due to communication latency, it is always possible that this notification MAY arrive after the request has already finished.
//
// This notification indicates that the result will be unused, so any associated processing SHOULD cease.
type ClientNotification struct {
	JSONRPC string                      `json:"jsonrpc"`
	Method  string                      `json:"method"`
	Params  CancelledNotificationParams `json:"params"`
}

type ClientRequest any

// Common result fields.
type ClientResult struct {
}

// A request from the client to the server, to ask for completion options.
type CompleteRequest struct {
	ID      RequestId             `json:"id"`
	JSONRPC string                `json:"jsonrpc"`
	Method  string                `json:"method"`
	Params  CompleteRequestParams `json:"params"`
}

// Parameters for a `completion/complete` request.
type CompleteRequestParams struct {
	Meta     RequestMetaObject `json:"_meta"`
	Argument map[string]any    `json:"argument"`
	Context  map[string]any    `json:"context,omitempty"`
	Ref      any               `json:"ref"`
}

// The result returned by the server for a {@link CompleteRequestcompletion/complete} request.
type CompleteResult struct {
	Meta       *MetaObject    `json:"_meta,omitempty"`
	Completion map[string]any `json:"completion"`
	ResultType string         `json:"resultType"`
}

// A successful response from the server for a {@link CompleteRequestcompletion/complete} request.
type CompleteResultResponse struct {
	ID      RequestId      `json:"id"`
	JSONRPC string         `json:"jsonrpc"`
	Result  CompleteResult `json:"result"`
}

type ContentBlock any

// A request from the server to sample an LLM via the client. The client has full discretion over which model to select. The client should also inform the user before beginning sampling, to allow them to inspect the request (human in the loop) and decide whether to approve it.
type CreateMessageRequest struct {
	Method string                     `json:"method"`
	Params CreateMessageRequestParams `json:"params"`
}

// Parameters for a `sampling/createMessage` request.
type CreateMessageRequestParams struct {
	IncludeContext   *IncludeContext   `json:"includeContext,omitempty"`
	MaxTokens        int               `json:"maxTokens"`
	Messages         []SamplingMessage `json:"messages"`
	Metadata         *JSONObject       `json:"metadata,omitempty"`
	ModelPreferences *ModelPreferences `json:"modelPreferences,omitempty"`
	StopSequences    []string          `json:"stopSequences,omitempty"`
	SystemPrompt     *string           `json:"systemPrompt,omitempty"`
	Temperature      *float64          `json:"temperature,omitempty"`
	ToolChoice       *ToolChoice       `json:"toolChoice,omitempty"`
	Tools            []Tool            `json:"tools,omitempty"`
}

// The result returned by the client for a {@link CreateMessageRequestsampling/createMessage} request.
// The client should inform the user before returning the sampled message, to allow them
// to inspect the response (human in the loop) and decide whether to allow the server to see it.
type CreateMessageResult struct {
	Meta       *MetaObject `json:"_meta,omitempty"`
	Content    any         `json:"content"`
	Model      string      `json:"model"`
	Role       Role        `json:"role"`
	StopReason *string     `json:"stopReason,omitempty"`
}

// An opaque token used to represent a cursor for pagination.
type Cursor = string

// A request from the client asking the server to advertise its supported
// protocol versions, capabilities, and other metadata. Servers **MUST**
// implement `server/discover`. Clients **MAY** call it but are not required
// to — version negotiation can also happen inline via per-request `_meta`.
type DiscoverRequest struct {
	ID      RequestId     `json:"id"`
	JSONRPC string        `json:"jsonrpc"`
	Method  string        `json:"method"`
	Params  RequestParams `json:"params"`
}

// The result returned by the server for a {@link DiscoverRequestserver/discover} request.
type DiscoverResult struct {
	Meta              *MetaObject        `json:"_meta,omitempty"`
	CacheScope        CacheScope         `json:"cacheScope"`
	Capabilities      ServerCapabilities `json:"capabilities"`
	Instructions      *string            `json:"instructions,omitempty"`
	ResultType        string             `json:"resultType"`
	ServerInfo        Implementation     `json:"serverInfo"`
	SupportedVersions []string           `json:"supportedVersions"`
	TtlMs             int                `json:"ttlMs"`
}

// A successful response from the server for a {@link DiscoverRequestserver/discover} request.
type DiscoverResultResponse struct {
	ID      RequestId      `json:"id"`
	JSONRPC string         `json:"jsonrpc"`
	Result  DiscoverResult `json:"result"`
}

// A request from the server to elicit additional information from the user via the client.
type ElicitRequest struct {
	Method string              `json:"method"`
	Params ElicitRequestParams `json:"params"`
}

// The parameters for a request to elicit non-sensitive information from the user via a form in the client.
type ElicitRequestFormParams struct {
	Message         string         `json:"message"`
	Mode            *string        `json:"mode,omitempty"`
	RequestedSchema map[string]any `json:"requestedSchema"`
}

// The parameters for a request to elicit additional information from the user via the client.
type ElicitRequestParams any

// The parameters for a request to elicit information from the user via a URL in the client.
type ElicitRequestURLParams struct {
	Message string `json:"message"`
	Mode    string `json:"mode"`
	URL     string `json:"url"`
}

// The result returned by the client for an {@link ElicitRequestelicitation/create} request.
type ElicitResult struct {
	Action  Action         `json:"action"`
	Content map[string]any `json:"content,omitempty"`
}

// The contents of a resource, embedded into a prompt or tool call result.
//
// It is up to the client how best to render embedded resources for the benefit
// of the LLM and/or the user.
type EmbeddedResource struct {
	Meta        *MetaObject  `json:"_meta,omitempty"`
	Annotations *Annotations `json:"annotations,omitempty"`
	Resource    any          `json:"resource"`
	Type        string       `json:"type"`
}

// Common result fields.
type EmptyResult struct {
}

type EnumSchema any

type Error struct {
	Code    int    `json:"code"`
	Data    *any   `json:"data,omitempty"`
	Message string `json:"message"`
}

// Used by the client to get a prompt provided by the server.
type GetPromptRequest struct {
	ID      RequestId              `json:"id"`
	JSONRPC string                 `json:"jsonrpc"`
	Method  string                 `json:"method"`
	Params  GetPromptRequestParams `json:"params"`
}

// Parameters for a `prompts/get` request.
type GetPromptRequestParams struct {
	Meta           RequestMetaObject `json:"_meta"`
	Arguments      map[string]string `json:"arguments,omitempty"`
	InputResponses *InputResponses   `json:"inputResponses,omitempty"`
	Name           string            `json:"name"`
	RequestState   *string           `json:"requestState,omitempty"`
}

// The result returned by the server for a {@link GetPromptRequestprompts/get} request.
type GetPromptResult struct {
	Meta        *MetaObject     `json:"_meta,omitempty"`
	Description *string         `json:"description,omitempty"`
	Messages    []PromptMessage `json:"messages"`
	ResultType  string          `json:"resultType"`
}

// A successful response from the server for a {@link GetPromptRequestprompts/get} request.
type GetPromptResultResponse struct {
	ID      RequestId `json:"id"`
	JSONRPC string    `json:"jsonrpc"`
	Result  any       `json:"result"`
}

// Returned when a server rejects a request because the values in the HTTP
// headers do not match the corresponding values in the request body, or
// because required headers are missing or malformed. For HTTP, the response
// status code MUST be `400 Bad Request`.
type HeaderMismatchError struct {
	Error   any        `json:"error"`
	ID      *RequestId `json:"id,omitempty"`
	JSONRPC string     `json:"jsonrpc"`
}

// An optionally-sized icon that can be displayed in a user interface.
type Icon struct {
	MIMEType *string  `json:"mimeType,omitempty"`
	Sizes    []string `json:"sizes,omitempty"`
	Src      string   `json:"src"`
	Theme    *Theme   `json:"theme,omitempty"`
}

// Base interface to add `icons` property.
type Icons struct {
	Icons []Icon `json:"icons,omitempty"`
}

// An image provided to or from an LLM.
type ImageContent struct {
	Meta        *MetaObject  `json:"_meta,omitempty"`
	Annotations *Annotations `json:"annotations,omitempty"`
	Data        []byte       `json:"data"`
	MIMEType    string       `json:"mimeType"`
	Type        string       `json:"type"`
}

// Describes the MCP implementation.
type Implementation struct {
	Description *string `json:"description,omitempty"`
	Icons       []Icon  `json:"icons,omitempty"`
	Name        string  `json:"name"`
	Title       *string `json:"title,omitempty"`
	Version     string  `json:"version"`
	WebsiteURL  *string `json:"websiteUrl,omitempty"`
}

type InputRequest any

// A map of server-initiated requests that the client must fulfill.
// Keys are server-assigned identifiers; values are the request objects.
type InputRequests = map[string]InputRequest

// An InputRequiredResult sent by the server to indicate that additional input is needed
// before the request can be completed.
//
// At least one of `inputRequests` or `requestState` MUST be present.
type InputRequiredResult struct {
	Meta          *MetaObject    `json:"_meta,omitempty"`
	InputRequests *InputRequests `json:"inputRequests,omitempty"`
	RequestState  *string        `json:"requestState,omitempty"`
	ResultType    string         `json:"resultType"`
}

type InputResponse any

type InputResponseRequestParams struct {
	Meta           RequestMetaObject `json:"_meta"`
	InputResponses *InputResponses   `json:"inputResponses,omitempty"`
	RequestState   *string           `json:"requestState,omitempty"`
}

// A map of client responses to server-initiated requests.
// Keys correspond to the keys in the {@link InputRequests} map;
// values are the client's result for each request.
type InputResponses = map[string]InputResponse

// A JSON-RPC error indicating that an internal error occurred on the receiver. This error is returned when the receiver encounters an unexpected condition that prevents it from fulfilling the request.
type InternalError struct {
	Code    int    `json:"code"`
	Data    *any   `json:"data,omitempty"`
	Message string `json:"message"`
}

// A JSON-RPC error indicating that the method parameters are invalid or malformed.
//
// In MCP, this error is returned in various contexts when request parameters fail validation:
//
// - **Tools**: Unknown tool name or invalid tool arguments
// - **Prompts**: Unknown prompt name or missing required arguments
// - **Pagination**: Invalid or expired cursor values
// - **Logging**: Invalid log level
// - **Elicitation**: Server requests an elicitation mode not declared in client capabilities
// - **Sampling**: Missing tool result or tool results mixed with other content
type InvalidParamsError struct {
	Code    int    `json:"code"`
	Data    *any   `json:"data,omitempty"`
	Message string `json:"message"`
}

// A JSON-RPC error indicating that the request is not a valid request object. This error is returned when the message structure does not conform to the JSON-RPC 2.0 specification requirements for a request (e.g., missing required fields like `jsonrpc` or `method`, or using invalid types for these fields).
type InvalidRequestError struct {
	Code    int    `json:"code"`
	Data    *any   `json:"data,omitempty"`
	Message string `json:"message"`
}

type JSONArray = []JSONValue

type JSONObject = map[string]JSONValue

// A response to a request that indicates an error occurred.
type JSONRPCErrorResponse struct {
	Error   Error      `json:"error"`
	ID      *RequestId `json:"id,omitempty"`
	JSONRPC string     `json:"jsonrpc"`
}

// Refers to any valid JSON-RPC object that can be decoded off the wire, or encoded to be sent.
type JSONRPCMessage any

// A notification which does not expect a response.
type JSONRPCNotification struct {
	JSONRPC string         `json:"jsonrpc"`
	Method  string         `json:"method"`
	Params  map[string]any `json:"params,omitempty"`
}

// A request that expects a response.
type JSONRPCRequest struct {
	ID      RequestId      `json:"id"`
	JSONRPC string         `json:"jsonrpc"`
	Method  string         `json:"method"`
	Params  map[string]any `json:"params,omitempty"`
}

// A response to a request, containing either the result or error.
type JSONRPCResponse any

// A successful (non-error) response to a request.
type JSONRPCResultResponse struct {
	ID      RequestId `json:"id"`
	JSONRPC string    `json:"jsonrpc"`
	Result  Result    `json:"result"`
}

type JSONValue any

// Use {@link TitledSingleSelectEnumSchema} instead.
// This interface will be removed in a future version.
type LegacyTitledEnumSchema struct {
	Default     *string  `json:"default,omitempty"`
	Description *string  `json:"description,omitempty"`
	Enum        []string `json:"enum"`
	EnumNames   []string `json:"enumNames,omitempty"`
	Title       *string  `json:"title,omitempty"`
	Type        string   `json:"type"`
}

// Sent from the client to request a list of prompts and prompt templates the server has.
type ListPromptsRequest struct {
	ID      RequestId              `json:"id"`
	JSONRPC string                 `json:"jsonrpc"`
	Method  string                 `json:"method"`
	Params  PaginatedRequestParams `json:"params"`
}

// The result returned by the server for a {@link ListPromptsRequestprompts/list} request.
type ListPromptsResult struct {
	Meta       *MetaObject `json:"_meta,omitempty"`
	CacheScope CacheScope  `json:"cacheScope"`
	NextCursor *string     `json:"nextCursor,omitempty"`
	Prompts    []Prompt    `json:"prompts"`
	ResultType string      `json:"resultType"`
	TtlMs      int         `json:"ttlMs"`
}

// A successful response from the server for a {@link ListPromptsRequestprompts/list} request.
type ListPromptsResultResponse struct {
	ID      RequestId         `json:"id"`
	JSONRPC string            `json:"jsonrpc"`
	Result  ListPromptsResult `json:"result"`
}

// Sent from the client to request a list of resource templates the server has.
type ListResourceTemplatesRequest struct {
	ID      RequestId              `json:"id"`
	JSONRPC string                 `json:"jsonrpc"`
	Method  string                 `json:"method"`
	Params  PaginatedRequestParams `json:"params"`
}

// The result returned by the server for a {@link ListResourceTemplatesRequestresources/templates/list} request.
type ListResourceTemplatesResult struct {
	Meta              *MetaObject        `json:"_meta,omitempty"`
	CacheScope        CacheScope         `json:"cacheScope"`
	NextCursor        *string            `json:"nextCursor,omitempty"`
	ResourceTemplates []ResourceTemplate `json:"resourceTemplates"`
	ResultType        string             `json:"resultType"`
	TtlMs             int                `json:"ttlMs"`
}

// A successful response from the server for a {@link ListResourceTemplatesRequestresources/templates/list} request.
type ListResourceTemplatesResultResponse struct {
	ID      RequestId                   `json:"id"`
	JSONRPC string                      `json:"jsonrpc"`
	Result  ListResourceTemplatesResult `json:"result"`
}

// Sent from the client to request a list of resources the server has.
type ListResourcesRequest struct {
	ID      RequestId              `json:"id"`
	JSONRPC string                 `json:"jsonrpc"`
	Method  string                 `json:"method"`
	Params  PaginatedRequestParams `json:"params"`
}

// The result returned by the server for a {@link ListResourcesRequestresources/list} request.
type ListResourcesResult struct {
	Meta       *MetaObject `json:"_meta,omitempty"`
	CacheScope CacheScope  `json:"cacheScope"`
	NextCursor *string     `json:"nextCursor,omitempty"`
	Resources  []Resource  `json:"resources"`
	ResultType string      `json:"resultType"`
	TtlMs      int         `json:"ttlMs"`
}

// A successful response from the server for a {@link ListResourcesRequestresources/list} request.
type ListResourcesResultResponse struct {
	ID      RequestId           `json:"id"`
	JSONRPC string              `json:"jsonrpc"`
	Result  ListResourcesResult `json:"result"`
}

// Sent from the server to request a list of root URIs from the client. Roots allow
// servers to ask for specific directories or files to operate on. A common example
// for roots is providing a set of repositories or directories a server should operate
// on.
//
// This request is typically used when the server needs to understand the file system
// structure or access specific locations that the client has permission to read from.
type ListRootsRequest struct {
	Method string         `json:"method"`
	Params map[string]any `json:"params,omitempty"`
}

// The result returned by the client for a {@link ListRootsRequestroots/list} request.
// This result contains an array of {@link Root} objects, each representing a root directory
// or file that the server can operate on.
type ListRootsResult struct {
	Roots []Root `json:"roots"`
}

// Sent from the client to request a list of tools the server has.
type ListToolsRequest struct {
	ID      RequestId              `json:"id"`
	JSONRPC string                 `json:"jsonrpc"`
	Method  string                 `json:"method"`
	Params  PaginatedRequestParams `json:"params"`
}

// The result returned by the server for a {@link ListToolsRequesttools/list} request.
type ListToolsResult struct {
	Meta       *MetaObject `json:"_meta,omitempty"`
	CacheScope CacheScope  `json:"cacheScope"`
	NextCursor *string     `json:"nextCursor,omitempty"`
	ResultType string      `json:"resultType"`
	Tools      []Tool      `json:"tools"`
	TtlMs      int         `json:"ttlMs"`
}

// A successful response from the server for a {@link ListToolsRequesttools/list} request.
type ListToolsResultResponse struct {
	ID      RequestId       `json:"id"`
	JSONRPC string          `json:"jsonrpc"`
	Result  ListToolsResult `json:"result"`
}

// JSONRPCNotification of a log message passed from server to client. The client opts in by setting `"io.modelcontextprotocol/logLevel"` in a request's `_meta`.
type LoggingMessageNotification struct {
	JSONRPC string                           `json:"jsonrpc"`
	Method  string                           `json:"method"`
	Params  LoggingMessageNotificationParams `json:"params"`
}

// Parameters for a `notifications/message` notification.
type LoggingMessageNotificationParams struct {
	Meta   *NotificationMetaObject `json:"_meta,omitempty"`
	Data   any                     `json:"data"`
	Level  LoggingLevel            `json:"level"`
	Logger *string                 `json:"logger,omitempty"`
}

// Represents the contents of a `_meta` field, which clients and servers use to attach additional metadata to their interactions.
//
// Certain key names are reserved by MCP for protocol-level metadata; implementations MUST NOT make assumptions about values at these keys. Additionally, specific schema definitions may reserve particular names for purpose-specific metadata, as declared in those definitions.
//
// Valid keys have two segments:
//
// **Prefix:**
// - Optional — if specified, MUST be a series of _labels_ separated by dots (`.`), followed by a slash (`/`).
// - Labels MUST start with a letter and end with a letter or digit. Interior characters may be letters, digits, or hyphens (`-`).
// - Implementations SHOULD use reverse DNS notation (e.g., `com.example/` rather than `example.com/`).
// - Any prefix where the second label is `modelcontextprotocol` or `mcp` is **reserved** for MCP use. For example: `io.modelcontextprotocol/`, `dev.mcp/`, `org.modelcontextprotocol.api/`, and `com.mcp.tools/` are all reserved. However, `com.example.mcp/` is NOT reserved, as the second label is `example`.
//
// **Name:**
// - Unless empty, MUST start and end with an alphanumeric character (`[a-z0-9A-Z]`).
// - Interior characters may be alphanumeric, hyphens (`-`), underscores (`_`), or dots (`.`).
type MetaObject = map[string]any

// A JSON-RPC error indicating that the requested method does not exist or is not available.
//
// In MCP, a server returns this error when a client invokes a method the server does not implement — either a genuinely unknown method, or one gated behind a server capability the server did not advertise (e.g., calling `prompts/list` when the `prompts` capability was not advertised).
//
// A request that requires a client capability the client did not declare is signalled instead by {@link MissingRequiredClientCapabilityError} (`-32021`).
type MethodNotFoundError struct {
	Code    int    `json:"code"`
	Data    *any   `json:"data,omitempty"`
	Message string `json:"message"`
}

// Returned when processing a request requires a capability the client did not
// declare in `clientCapabilities`. For HTTP, the response status code MUST be
// `400 Bad Request`.
type MissingRequiredClientCapabilityError struct {
	Error   any        `json:"error"`
	ID      *RequestId `json:"id,omitempty"`
	JSONRPC string     `json:"jsonrpc"`
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

type MultiSelectEnumSchema any

type Notification struct {
	Method string         `json:"method"`
	Params map[string]any `json:"params,omitempty"`
}

// Extends {@link MetaObject} with additional notification-specific fields. All key naming rules from `MetaObject` apply.
type NotificationMetaObject struct {
	IoModelcontextprotocolsubscriptionID *RequestId `json:"io.modelcontextprotocol/subscriptionId,omitempty"`
}

// Common params for any notification.
type NotificationParams struct {
	Meta *NotificationMetaObject `json:"_meta,omitempty"`
}

type NumberSchema struct {
	Default     *float64 `json:"default,omitempty"`
	Description *string  `json:"description,omitempty"`
	Maximum     *float64 `json:"maximum,omitempty"`
	Minimum     *float64 `json:"minimum,omitempty"`
	Title       *string  `json:"title,omitempty"`
	Type        Type     `json:"type"`
}

type PaginatedRequest struct {
	ID      RequestId              `json:"id"`
	JSONRPC string                 `json:"jsonrpc"`
	Method  string                 `json:"method"`
	Params  PaginatedRequestParams `json:"params"`
}

// Common params for paginated requests.
type PaginatedRequestParams struct {
	Meta   RequestMetaObject `json:"_meta"`
	Cursor *string           `json:"cursor,omitempty"`
}

type PaginatedResult struct {
	Meta       *MetaObject `json:"_meta,omitempty"`
	NextCursor *string     `json:"nextCursor,omitempty"`
	ResultType string      `json:"resultType"`
}

// A JSON-RPC error indicating that invalid JSON was received by the server. This error is returned when the server cannot parse the JSON text of a message.
type ParseError struct {
	Code    int    `json:"code"`
	Data    *any   `json:"data,omitempty"`
	Message string `json:"message"`
}

// Restricted schema definitions that only allow primitive types
// without nested objects or arrays.
type PrimitiveSchemaDefinition any

// An out-of-band notification used to inform the receiver of a progress update for a long-running request.
type ProgressNotification struct {
	JSONRPC string                     `json:"jsonrpc"`
	Method  string                     `json:"method"`
	Params  ProgressNotificationParams `json:"params"`
}

// Parameters for a {@link ProgressNotificationnotifications/progress} notification.
type ProgressNotificationParams struct {
	Meta          *NotificationMetaObject `json:"_meta,omitempty"`
	Message       *string                 `json:"message,omitempty"`
	Progress      float64                 `json:"progress"`
	ProgressToken ProgressToken           `json:"progressToken"`
	Total         *float64                `json:"total,omitempty"`
}

// A progress token, used to associate progress notifications with the original request.
type ProgressToken struct {
}

// A prompt or prompt template that the server offers.
type Prompt struct {
	Meta        *MetaObject      `json:"_meta,omitempty"`
	Arguments   []PromptArgument `json:"arguments,omitempty"`
	Description *string          `json:"description,omitempty"`
	Icons       []Icon           `json:"icons,omitempty"`
	Name        string           `json:"name"`
	Title       *string          `json:"title,omitempty"`
}

// Describes an argument that a prompt can accept.
type PromptArgument struct {
	Description *string `json:"description,omitempty"`
	Name        string  `json:"name"`
	Required    *bool   `json:"required,omitempty"`
	Title       *string `json:"title,omitempty"`
}

// An optional notification from the server to the client, informing it that the list of prompts it offers has changed. This is only delivered on a {@link SubscriptionsListenRequestsubscriptions/listen} stream when the client requested it via the `promptsListChanged` filter field.
type PromptListChangedNotification struct {
	JSONRPC string              `json:"jsonrpc"`
	Method  string              `json:"method"`
	Params  *NotificationParams `json:"params,omitempty"`
}

// Describes a message returned as part of a prompt.
//
// This is similar to {@link SamplingMessage}, but also supports the embedding of
// resources from the MCP server.
type PromptMessage struct {
	Content ContentBlock `json:"content"`
	Role    Role         `json:"role"`
}

// Identifies a prompt.
type PromptReference struct {
	Name  string  `json:"name"`
	Title *string `json:"title,omitempty"`
	Type  string  `json:"type"`
}

// Sent from the client to the server, to read a specific resource URI.
type ReadResourceRequest struct {
	ID      RequestId                 `json:"id"`
	JSONRPC string                    `json:"jsonrpc"`
	Method  string                    `json:"method"`
	Params  ReadResourceRequestParams `json:"params"`
}

// Parameters for a `resources/read` request.
type ReadResourceRequestParams struct {
	Meta           RequestMetaObject `json:"_meta"`
	InputResponses *InputResponses   `json:"inputResponses,omitempty"`
	RequestState   *string           `json:"requestState,omitempty"`
	URI            string            `json:"uri"`
}

// The result returned by the server for a {@link ReadResourceRequestresources/read} request.
type ReadResourceResult struct {
	Meta       *MetaObject `json:"_meta,omitempty"`
	CacheScope CacheScope  `json:"cacheScope"`
	Contents   []any       `json:"contents"`
	ResultType string      `json:"resultType"`
	TtlMs      int         `json:"ttlMs"`
}

// A successful response from the server for a {@link ReadResourceRequestresources/read} request.
type ReadResourceResultResponse struct {
	ID      RequestId `json:"id"`
	JSONRPC string    `json:"jsonrpc"`
	Result  any       `json:"result"`
}

type Request struct {
	Method string         `json:"method"`
	Params map[string]any `json:"params,omitempty"`
}

// A uniquely identifying ID for a request in JSON-RPC.
type RequestId struct {
}

// Extends {@link MetaObject} with additional request-specific fields. All key naming rules from `MetaObject` apply.
type RequestMetaObject struct {
	IoModelcontextprotocolclientCapabilities ClientCapabilities `json:"io.modelcontextprotocol/clientCapabilities"`
	IoModelcontextprotocolclientInfo         Implementation     `json:"io.modelcontextprotocol/clientInfo"`
	IoModelcontextprotocollogLevel           *LoggingLevel      `json:"io.modelcontextprotocol/logLevel,omitempty"`
	IoModelcontextprotocolprotocolVersion    string             `json:"io.modelcontextprotocol/protocolVersion"`
	ProgressToken                            *ProgressToken     `json:"progressToken,omitempty"`
}

// Common params for any request.
type RequestParams struct {
	Meta RequestMetaObject `json:"_meta"`
}

// A known resource that the server is capable of reading.
type Resource struct {
	Meta        *MetaObject  `json:"_meta,omitempty"`
	Annotations *Annotations `json:"annotations,omitempty"`
	Description *string      `json:"description,omitempty"`
	Icons       []Icon       `json:"icons,omitempty"`
	MIMEType    *string      `json:"mimeType,omitempty"`
	Name        string       `json:"name"`
	Size        *int         `json:"size,omitempty"`
	Title       *string      `json:"title,omitempty"`
	URI         string       `json:"uri"`
}

// The contents of a specific resource or sub-resource.
type ResourceContents struct {
	Meta     *MetaObject `json:"_meta,omitempty"`
	MIMEType *string     `json:"mimeType,omitempty"`
	URI      string      `json:"uri"`
}

// A resource that the server is capable of reading, included in a prompt or tool call result.
//
// Note: resource links returned by tools are not guaranteed to appear in the results of {@link ListResourcesRequestresources/list} requests.
type ResourceLink struct {
	Meta        *MetaObject  `json:"_meta,omitempty"`
	Annotations *Annotations `json:"annotations,omitempty"`
	Description *string      `json:"description,omitempty"`
	Icons       []Icon       `json:"icons,omitempty"`
	MIMEType    *string      `json:"mimeType,omitempty"`
	Name        string       `json:"name"`
	Size        *int         `json:"size,omitempty"`
	Title       *string      `json:"title,omitempty"`
	Type        string       `json:"type"`
	URI         string       `json:"uri"`
}

// An optional notification from the server to the client, informing it that the list of resources it can read from has changed. This is only delivered on a {@link SubscriptionsListenRequestsubscriptions/listen} stream when the client requested it via the `resourcesListChanged` filter field.
type ResourceListChangedNotification struct {
	JSONRPC string              `json:"jsonrpc"`
	Method  string              `json:"method"`
	Params  *NotificationParams `json:"params,omitempty"`
}

// Common params for resource-related requests.
type ResourceRequestParams struct {
	Meta RequestMetaObject `json:"_meta"`
	URI  string            `json:"uri"`
}

// A template description for resources available on the server.
type ResourceTemplate struct {
	Meta        *MetaObject  `json:"_meta,omitempty"`
	Annotations *Annotations `json:"annotations,omitempty"`
	Description *string      `json:"description,omitempty"`
	Icons       []Icon       `json:"icons,omitempty"`
	MIMEType    *string      `json:"mimeType,omitempty"`
	Name        string       `json:"name"`
	Title       *string      `json:"title,omitempty"`
	URITemplate string       `json:"uriTemplate"`
}

// A reference to a resource or resource template definition.
type ResourceTemplateReference struct {
	Type string `json:"type"`
	URI  string `json:"uri"`
}

// A notification from the server to the client, informing it that a resource has changed and may need to be read again. This is only sent for resources the client opted in to via the `resourceSubscriptions` field of a {@link SubscriptionsListenRequestsubscriptions/listen} request.
type ResourceUpdatedNotification struct {
	JSONRPC string                            `json:"jsonrpc"`
	Method  string                            `json:"method"`
	Params  ResourceUpdatedNotificationParams `json:"params"`
}

// Parameters for a `notifications/resources/updated` notification.
type ResourceUpdatedNotificationParams struct {
	Meta *NotificationMetaObject `json:"_meta,omitempty"`
	URI  string                  `json:"uri"`
}

// Common result fields.
type Result struct {
	Meta       *MetaObject `json:"_meta,omitempty"`
	ResultType string      `json:"resultType"`
}

// Indicates the type of a {@link Result} object, allowing the client to
// determine how to parse the response.
//
// complete - the request completed successfully and the result contains the final content.
// input_required - the request requires additional input and the result contains an {@link InputRequiredResult} object with instructions for the client to provide additional input before retrying the original request.
type ResultType = string

// Represents a root directory or file that the server can operate on.
type Root struct {
	Meta *MetaObject `json:"_meta,omitempty"`
	Name *string     `json:"name,omitempty"`
	URI  string      `json:"uri"`
}

// Describes a message issued to or received from an LLM API.
type SamplingMessage struct {
	Meta    *MetaObject `json:"_meta,omitempty"`
	Content any         `json:"content"`
	Role    Role        `json:"role"`
}

type SamplingMessageContentBlock any

// Capabilities that a server may support. Known capabilities are defined here, in this schema, but this is not a closed set: any server can define its own, additional capabilities.
type ServerCapabilities struct {
	Completions  *JSONObject           `json:"completions,omitempty"`
	Experimental map[string]JSONObject `json:"experimental,omitempty"`
	Extensions   map[string]JSONObject `json:"extensions,omitempty"`
	Logging      *JSONObject           `json:"logging,omitempty"`
	Prompts      map[string]any        `json:"prompts,omitempty"`
	Resources    map[string]any        `json:"resources,omitempty"`
	Tools        map[string]any        `json:"tools,omitempty"`
}

type ServerNotification any

type ServerResult any

type SingleSelectEnumSchema any

type StringSchema struct {
	Default     *string `json:"default,omitempty"`
	Description *string `json:"description,omitempty"`
	Format      *Format `json:"format,omitempty"`
	MaxLength   *int    `json:"maxLength,omitempty"`
	MinLength   *int    `json:"minLength,omitempty"`
	Title       *string `json:"title,omitempty"`
	Type        string  `json:"type"`
}

// The set of notification types a client may opt in to on a
// {@link SubscriptionsListenRequestsubscriptions/listen} request.
//
// Each notification type is **opt-in**; the server **MUST NOT** send
// notification types the client has not explicitly requested here.
type SubscriptionFilter struct {
	PromptsListChanged    *bool    `json:"promptsListChanged,omitempty"`
	ResourceSubscriptions []string `json:"resourceSubscriptions,omitempty"`
	ResourcesListChanged  *bool    `json:"resourcesListChanged,omitempty"`
	ToolsListChanged      *bool    `json:"toolsListChanged,omitempty"`
}

// Sent by the server as the first message on a
// {@link SubscriptionsListenRequestsubscriptions/listen} stream to acknowledge
// that the subscription has been established and to report which notification
// types it agreed to honor.
type SubscriptionsAcknowledgedNotification struct {
	JSONRPC string                                      `json:"jsonrpc"`
	Method  string                                      `json:"method"`
	Params  SubscriptionsAcknowledgedNotificationParams `json:"params"`
}

// Parameters for a {@link SubscriptionsAcknowledgedNotificationnotifications/subscriptions/acknowledged} notification.
type SubscriptionsAcknowledgedNotificationParams struct {
	Meta          *NotificationMetaObject `json:"_meta,omitempty"`
	Notifications SubscriptionFilter      `json:"notifications"`
}

// Sent from the client to open a long-lived channel for receiving notifications
// outside the context of a specific request. Replaces the previous HTTP GET
// endpoint and ensures consistent behavior between HTTP and STDIO.
type SubscriptionsListenRequest struct {
	ID      RequestId                        `json:"id"`
	JSONRPC string                           `json:"jsonrpc"`
	Method  string                           `json:"method"`
	Params  SubscriptionsListenRequestParams `json:"params"`
}

// Parameters for a {@link SubscriptionsListenRequestsubscriptions/listen} request.
type SubscriptionsListenRequestParams struct {
	Meta          RequestMetaObject  `json:"_meta"`
	Notifications SubscriptionFilter `json:"notifications"`
}

// The response to a {@link SubscriptionsListenRequestsubscriptions/listen}
// request, signalling that the subscription has ended gracefully (for example,
// during server shutdown). Because the listen stream is long-lived, this result
// is sent only when the server tears the subscription down; an abrupt transport
// close carries no response. The result body is otherwise empty.
type SubscriptionsListenResult struct {
	Meta       SubscriptionsListenResultMeta `json:"_meta"`
	ResultType string                        `json:"resultType"`
}

// Extends {@link MetaObject} with the subscription-stream identifier carried by a
// {@link SubscriptionsListenResult}. All key naming rules from `MetaObject` apply.
type SubscriptionsListenResultMeta struct {
	IoModelcontextprotocolsubscriptionID RequestId `json:"io.modelcontextprotocol/subscriptionId"`
}

// Text provided to or from an LLM.
type TextContent struct {
	Meta        *MetaObject  `json:"_meta,omitempty"`
	Annotations *Annotations `json:"annotations,omitempty"`
	Text        string       `json:"text"`
	Type        string       `json:"type"`
}

type TextResourceContents struct {
	Meta     *MetaObject `json:"_meta,omitempty"`
	MIMEType *string     `json:"mimeType,omitempty"`
	Text     string      `json:"text"`
	URI      string      `json:"uri"`
}

// Schema for multiple-selection enumeration with display titles for each option.
type TitledMultiSelectEnumSchema struct {
	Default     []string       `json:"default,omitempty"`
	Description *string        `json:"description,omitempty"`
	Items       map[string]any `json:"items"`
	MaxItems    *int           `json:"maxItems,omitempty"`
	MinItems    *int           `json:"minItems,omitempty"`
	Title       *string        `json:"title,omitempty"`
	Type        string         `json:"type"`
}

// Schema for single-selection enumeration with display titles for each option.
type TitledSingleSelectEnumSchema struct {
	Default     *string          `json:"default,omitempty"`
	Description *string          `json:"description,omitempty"`
	OneOf       []map[string]any `json:"oneOf"`
	Title       *string          `json:"title,omitempty"`
	Type        string           `json:"type"`
}

// Definition for a tool the client can call.
type Tool struct {
	Meta         *MetaObject      `json:"_meta,omitempty"`
	Annotations  *ToolAnnotations `json:"annotations,omitempty"`
	Description  *string          `json:"description,omitempty"`
	Icons        []Icon           `json:"icons,omitempty"`
	InputSchema  map[string]any   `json:"inputSchema"`
	Name         string           `json:"name"`
	OutputSchema map[string]any   `json:"outputSchema,omitempty"`
	Title        *string          `json:"title,omitempty"`
}

// Additional properties describing a {@link Tool} to clients.
//
// NOTE: all properties in `ToolAnnotations` are **hints**.
// They are not guaranteed to provide a faithful description of
// tool behavior (including descriptive properties like `title`).
//
// Clients should never make tool use decisions based on `ToolAnnotations`
// received from untrusted servers.
type ToolAnnotations struct {
	DestructiveHint *bool   `json:"destructiveHint,omitempty"`
	IdempotentHint  *bool   `json:"idempotentHint,omitempty"`
	OpenWorldHint   *bool   `json:"openWorldHint,omitempty"`
	ReadOnlyHint    *bool   `json:"readOnlyHint,omitempty"`
	Title           *string `json:"title,omitempty"`
}

// Controls tool selection behavior for sampling requests.
type ToolChoice struct {
	Mode *Mode `json:"mode,omitempty"`
}

// An optional notification from the server to the client, informing it that the list of tools it offers has changed. This is only delivered on a {@link SubscriptionsListenRequestsubscriptions/listen} stream when the client requested it via the `toolsListChanged` filter field.
type ToolListChangedNotification struct {
	JSONRPC string              `json:"jsonrpc"`
	Method  string              `json:"method"`
	Params  *NotificationParams `json:"params,omitempty"`
}

// The result of a tool use, provided by the user back to the assistant.
type ToolResultContent struct {
	Meta              *MetaObject    `json:"_meta,omitempty"`
	Content           []ContentBlock `json:"content"`
	IsError           *bool          `json:"isError,omitempty"`
	StructuredContent *any           `json:"structuredContent,omitempty"`
	ToolUseID         string         `json:"toolUseId"`
	Type              string         `json:"type"`
}

// A request from the assistant to call a tool.
type ToolUseContent struct {
	Meta  *MetaObject    `json:"_meta,omitempty"`
	ID    string         `json:"id"`
	Input map[string]any `json:"input"`
	Name  string         `json:"name"`
	Type  string         `json:"type"`
}

// Returned when the request's protocol version is unknown to the server or
// unsupported (e.g., a known experimental or draft version the server has
// chosen not to implement). For HTTP, the response status code MUST be
// `400 Bad Request`.
type UnsupportedProtocolVersionError struct {
	Error   any        `json:"error"`
	ID      *RequestId `json:"id,omitempty"`
	JSONRPC string     `json:"jsonrpc"`
}

// Schema for multiple-selection enumeration without display titles for options.
type UntitledMultiSelectEnumSchema struct {
	Default     []string       `json:"default,omitempty"`
	Description *string        `json:"description,omitempty"`
	Items       map[string]any `json:"items"`
	MaxItems    *int           `json:"maxItems,omitempty"`
	MinItems    *int           `json:"minItems,omitempty"`
	Title       *string        `json:"title,omitempty"`
	Type        string         `json:"type"`
}

// Schema for single-selection enumeration without display titles for options.
type UntitledSingleSelectEnumSchema struct {
	Default     *string  `json:"default,omitempty"`
	Description *string  `json:"description,omitempty"`
	Enum        []string `json:"enum"`
	Title       *string  `json:"title,omitempty"`
	Type        string   `json:"type"`
}
