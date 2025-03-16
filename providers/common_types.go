package providers

// The authentication type of the specific provider
const (
	AuthTypeBearer  = "bearer"
	AuthTypeXheader = "xheader"
	AuthTypeQuery   = "query"
	AuthTypeNone    = "none"
)

// The default base URLs of each provider
const (
	AnthropicDefaultBaseURL  = "https://api.anthropic.com"
	CloudflareDefaultBaseURL = "https://api.cloudflare.com/client/v4/accounts/{ACCOUNT_ID}"
	CohereDefaultBaseURL     = "https://api.cohere.com"
	GroqDefaultBaseURL       = "https://api.groq.com"
	OllamaDefaultBaseURL     = "http://ollama:8080"
	OpenaiDefaultBaseURL     = "https://api.openai.com"
)

// The ID's of each provider
const (
	AnthropicID  = "anthropic"
	CloudflareID = "cloudflare"
	CohereID     = "cohere"
	GroqID       = "groq"
	OllamaID     = "ollama"
	OpenaiID     = "openai"
)

// Display names for providers
const (
	AnthropicDisplayName  = "Anthropic"
	CloudflareDisplayName = "Cloudflare"
	CohereDisplayName     = "Cohere"
	GroqDisplayName       = "Groq"
	OllamaDisplayName     = "Ollama"
	OpenaiDisplayName     = "Openai"
)

// MessageRole represents the role of a message sender
type MessageRole string

// Message role enum values
const (
	MessageRoleSystem    MessageRole = "system"
	MessageRoleUser      MessageRole = "user"
	MessageRoleAssistant MessageRole = "assistant"
	MessageRoleTool      MessageRole = "tool"
)

// ChatCompletionToolType represents a value type of a Tool in the API
type ChatCompletionToolType string

// ChatCompletionTool represents tool types in the API, currently only function supported
const (
	ChatCompletionToolTypeFunction ChatCompletionToolType = "function"
)

// FinishReason represents the reason for finishing a chat completion
type FinishReason string

// Chat completion finish reasons
const (
	FinishReasonStop          FinishReason = "stop"
	FinishReasonLength        FinishReason = "length"
	FinishReasonToolCalls     FinishReason = "tool_calls"
	FinishReasonContentFilter FinishReason = "content_filter"
)

// ChatCompletionChoice represents a ChatCompletionChoice in the API
type ChatCompletionChoice struct {
	FinishReason FinishReason `json:"finish_reason,omitempty"`
	Index        int          `json:"index,omitempty"`
	Message      Message      `json:"message,omitempty"`
}

// ChatCompletionMessageToolCall represents a ChatCompletionMessageToolCall in the API
type ChatCompletionMessageToolCall struct {
	Function ChatCompletionMessageToolCallFunction `json:"function,omitempty"`
	ID       string                                `json:"id"`
	Type     ChatCompletionToolType                `json:"type,omitempty"`
}

// ChatCompletionMessageToolCallChunk represents a ChatCompletionMessageToolCallChunk in the API
type ChatCompletionMessageToolCallChunk struct {
	Function struct{} `json:"function,omitempty"`
	ID       string   `json:"id"`
	Index    int      `json:"index,omitempty"`
	Type     string   `json:"type,omitempty"`
}

// ChatCompletionMessageToolCallFunction represents a ChatCompletionMessageToolCallFunction in the API
type ChatCompletionMessageToolCallFunction struct {
	Arguments string `json:"arguments,omitempty"`
	Name      string `json:"name,omitempty"`
}

// ChatCompletionStreamChoice represents a ChatCompletionStreamChoice in the API
type ChatCompletionStreamChoice struct {
	Delta        ChatCompletionStreamResponseDelta `json:"delta,omitempty"`
	FinishReason FinishReason                      `json:"finish_reason,omitempty"`
	Index        int                               `json:"index,omitempty"`
	Logprobs     struct{}                          `json:"logprobs,omitempty"`
}

// ChatCompletionStreamResponseDelta represents a ChatCompletionStreamResponseDelta in the API
type ChatCompletionStreamResponseDelta struct {
	Content   string                               `json:"content,omitempty"`
	Refusal   string                               `json:"refusal,omitempty"`
	Role      MessageRole                          `json:"role,omitempty"`
	ToolCalls []ChatCompletionMessageToolCallChunk `json:"tool_calls,omitempty"`
}

// ChatCompletionTool represents a ChatCompletionTool in the API
type ChatCompletionTool struct {
	Function FunctionObject         `json:"function,omitempty"`
	Type     ChatCompletionToolType `json:"type,omitempty"`
}

// CompletionUsage represents a CompletionUsage in the API
type CompletionUsage struct {
	CompletionTokens int `json:"completion_tokens,omitempty"`
	PromptTokens     int `json:"prompt_tokens,omitempty"`
	TotalTokens      int `json:"total_tokens,omitempty"`
}

// CreateChatCompletionRequest represents a CreateChatCompletionRequest in the API
type CreateChatCompletionRequest struct {
	MaxCompletionTokens int                  `json:"max_completion_tokens,omitempty"`
	Messages            []Message            `json:"messages,omitempty"`
	Model               string               `json:"model,omitempty"`
	Stream              bool                 `json:"stream,omitempty"`
	Tools               []ChatCompletionTool `json:"tools,omitempty"`
}

// CreateChatCompletionResponse represents a CreateChatCompletionResponse in the API
type CreateChatCompletionResponse struct {
	Choices []ChatCompletionChoice `json:"choices,omitempty"`
	Created int                    `json:"created,omitempty"`
	ID      string                 `json:"id"`
	Model   string                 `json:"model,omitempty"`
	Object  string                 `json:"object,omitempty"`
	Usage   CompletionUsage        `json:"usage,omitempty"`
}

// CreateChatCompletionStreamResponse represents a CreateChatCompletionStreamResponse in the API
type CreateChatCompletionStreamResponse struct {
	Choices           []ChatCompletionStreamChoice `json:"choices,omitempty"`
	Created           int                          `json:"created,omitempty"`
	ID                string                       `json:"id"`
	Model             string                       `json:"model,omitempty"`
	Object            string                       `json:"object,omitempty"`
	SystemFingerprint string                       `json:"system_fingerprint,omitempty"`
	Usage             CompletionUsage              `json:"usage,omitempty"`
}

// Error represents a Error in the API
type Error struct {
	Error string `json:"error,omitempty"`
}

// FunctionObject represents a FunctionObject in the API
type FunctionObject struct {
	Description string             `json:"description,omitempty"`
	Name        string             `json:"name,omitempty"`
	Parameters  FunctionParameters `json:"parameters,omitempty"`
	Strict      bool               `json:"strict,omitempty"`
}

// FunctionParameters represents a FunctionParameters in the API
type FunctionParameters struct {
	Additionalproperties bool                   `json:"additionalProperties,omitempty"`
	Properties           map[string]interface{} `json:"properties,omitempty"`
	Required             []string               `json:"required,omitempty"`
	Type                 string                 `json:"type,omitempty"`
}

// GenerateRequest represents a GenerateRequest in the API
type GenerateRequest struct {
	MaxTokens int                  `json:"max_tokens,omitempty"`
	Messages  []Message            `json:"messages,omitempty"`
	Model     string               `json:"model,omitempty"`
	Ssevents  bool                 `json:"ssevents,omitempty"`
	Stream    bool                 `json:"stream,omitempty"`
	Tools     []ChatCompletionTool `json:"tools,omitempty"`
}

// GenerateResponse represents a GenerateResponse in the API
type GenerateResponse struct {
	EventType EventType       `json:"event_type,omitempty"`
	Provider  string          `json:"provider,omitempty"`
	Response  ResponseTokens  `json:"response,omitempty"`
	Usage     CompletionUsage `json:"usage,omitempty"`
}

// ListModelsResponse represents a ListModelsResponse in the API
type ListModelsResponse struct {
	Data     []Model `json:"data,omitempty"`
	Object   string  `json:"object,omitempty"`
	Provider string  `json:"provider,omitempty"`
}

// Message represents a Message in the API
type Message struct {
	Content   string                          `json:"content,omitempty"`
	Role      MessageRole                     `json:"role,omitempty"`
	ToolCalls []ChatCompletionMessageToolCall `json:"tool_calls,omitempty"`
}

// Model represents a Model in the API
type Model struct {
	Created  int64  `json:"created,omitempty"`
	ID       string `json:"id"`
	Object   string `json:"object,omitempty"`
	OwnedBy  string `json:"owned_by,omitempty"`
	ServedBy string `json:"served_by,omitempty"`
}

// ResponseTokens represents a ResponseTokens in the API
type ResponseTokens struct {
	Content   string                          `json:"content,omitempty"`
	Model     string                          `json:"model,omitempty"`
	Role      MessageRole                     `json:"role,omitempty"`
	ToolCalls []ChatCompletionMessageToolCall `json:"tool_calls,omitempty"`
}

// Transform converts provider-specific response to common format
func (p *CreateChatCompletionResponse) Transform() CreateChatCompletionResponse {
	return *p
}
