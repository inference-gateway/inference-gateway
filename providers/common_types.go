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
	AnthropicDefaultBaseURL  = "https://api.anthropic.com/v1"
	CloudflareDefaultBaseURL = "https://api.cloudflare.com/client/v4/accounts/{ACCOUNT_ID}/ai"
	CohereDefaultBaseURL     = "https://api.cohere.ai"
	GroqDefaultBaseURL       = "https://api.groq.com/openai/v1"
	OllamaDefaultBaseURL     = "http://ollama:8080/v1"
	OpenaiDefaultBaseURL     = "https://api.openai.com/v1"
)

// The default endpoints of each provider
const (
	AnthropicModelsEndpoint  = "/models"
	AnthropicChatEndpoint    = "/chat/completions"
	CloudflareModelsEndpoint = "/finetunes/public?limit=1000"
	CloudflareChatEndpoint   = "/v1/chat/completions"
	CohereModelsEndpoint     = "/v1/models"
	CohereChatEndpoint       = "/compatibility/v1/chat/completions"
	GroqModelsEndpoint       = "/models"
	GroqChatEndpoint         = "/chat/completions"
	OllamaModelsEndpoint     = "/models"
	OllamaChatEndpoint       = "/chat/completions"
	OpenaiModelsEndpoint     = "/models"
	OpenaiChatEndpoint       = "/chat/completions"
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

// ListModelsTransformer interface for transforming provider-specific responses
type ListModelsTransformer interface {
	Transform() ListModelsResponse
}

// ChatCompletionChoice represents a ChatCompletionChoice in the API
type ChatCompletionChoice struct {
	FinishReason FinishReason `json:"finish_reason"`
	Index        int          `json:"index"`
	Message      *Message     `json:"message"`
}

// ChatCompletionMessageToolCall represents a ChatCompletionMessageToolCall in the API
type ChatCompletionMessageToolCall struct {
	Function *ChatCompletionMessageToolCallFunction `json:"function"`
	ID       string                                 `json:"id"`
	Type     *ChatCompletionToolType                `json:"type"`
}

// ChatCompletionMessageToolCallChunk represents a ChatCompletionMessageToolCallChunk in the API
type ChatCompletionMessageToolCallChunk struct {
	Function struct{} `json:"function,omitempty"`
	ID       string   `json:"id,omitempty"`
	Index    int      `json:"index"`
	Type     string   `json:"type,omitempty"`
}

// ChatCompletionMessageToolCallFunction represents a ChatCompletionMessageToolCallFunction in the API
type ChatCompletionMessageToolCallFunction struct {
	Arguments string `json:"arguments"`
	Name      string `json:"name"`
}

// ChatCompletionStreamChoice represents a ChatCompletionStreamChoice in the API
type ChatCompletionStreamChoice struct {
	Delta        *ChatCompletionStreamResponseDelta `json:"delta"`
	FinishReason *FinishReason                      `json:"finish_reason"`
	Index        int                                `json:"index"`
	Logprobs     struct{}                           `json:"logprobs,omitempty"`
}

// ChatCompletionStreamOptions represents a ChatCompletionStreamOptions in the API
type ChatCompletionStreamOptions struct {
	IncludeUsage bool `json:"include_usage,omitempty"`
}

// ChatCompletionStreamResponseDelta represents a ChatCompletionStreamResponseDelta in the API
type ChatCompletionStreamResponseDelta struct {
	Content   string                                `json:"content,omitempty"`
	Refusal   string                                `json:"refusal,omitempty"`
	Role      *MessageRole                          `json:"role,omitempty"`
	ToolCalls []*ChatCompletionMessageToolCallChunk `json:"tool_calls,omitempty"`
}

// ChatCompletionTool represents a ChatCompletionTool in the API
type ChatCompletionTool struct {
	Function *FunctionObject         `json:"function"`
	Type     *ChatCompletionToolType `json:"type"`
}

// CompletionUsage represents a CompletionUsage in the API
type CompletionUsage struct {
	CompletionTokens int64 `json:"completion_tokens"`
	PromptTokens     int64 `json:"prompt_tokens"`
	TotalTokens      int64 `json:"total_tokens"`
}

// CreateChatCompletionRequest represents a CreateChatCompletionRequest in the API
type CreateChatCompletionRequest struct {
	MaxTokens     int                          `json:"max_tokens,omitempty"`
	Messages      []*Message                   `json:"messages"`
	Model         string                       `json:"model"`
	Stream        bool                         `json:"stream,omitempty"`
	StreamOptions *ChatCompletionStreamOptions `json:"stream_options,omitempty"`
	Tools         []*ChatCompletionTool        `json:"tools,omitempty"`
}

// CreateChatCompletionResponse represents a CreateChatCompletionResponse in the API
type CreateChatCompletionResponse struct {
	Choices []*ChatCompletionChoice `json:"choices"`
	Created int                     `json:"created"`
	ID      string                  `json:"id"`
	Model   string                  `json:"model"`
	Object  string                  `json:"object"`
	Usage   *CompletionUsage        `json:"usage,omitempty"`
}

// CreateChatCompletionStreamResponse represents a CreateChatCompletionStreamResponse in the API
type CreateChatCompletionStreamResponse struct {
	Choices           []*ChatCompletionStreamChoice `json:"choices"`
	Created           int                           `json:"created"`
	ID                string                        `json:"id"`
	Model             string                        `json:"model"`
	Object            string                        `json:"object"`
	SystemFingerprint string                        `json:"system_fingerprint,omitempty"`
	Usage             *CompletionUsage              `json:"usage,omitempty"`
}

// Endpoints represents a Endpoints in the API
type Endpoints struct {
	Chat   string `json:"chat,omitempty"`
	Models string `json:"models,omitempty"`
}

// Error represents a Error in the API
type Error struct {
	Error string `json:"error,omitempty"`
}

// FunctionObject represents a FunctionObject in the API
type FunctionObject struct {
	Description string              `json:"description,omitempty"`
	Name        string              `json:"name"`
	Parameters  *FunctionParameters `json:"parameters,omitempty"`
	Strict      bool                `json:"strict,omitempty"`
}

// FunctionParameters represents a FunctionParameters in the API
type FunctionParameters struct {
	Additionalproperties bool                   `json:"additionalProperties,omitempty"`
	Properties           map[string]interface{} `json:"properties,omitempty"`
	Required             []string               `json:"required,omitempty"`
	Type                 string                 `json:"type,omitempty"`
}

// ListModelsResponse represents a ListModelsResponse in the API
type ListModelsResponse struct {
	Data     []*Model `json:"data,omitempty"`
	Object   string   `json:"object,omitempty"`
	Provider string   `json:"provider,omitempty"`
}

// Message represents a Message in the API
type Message struct {
	Content    string                           `json:"content"`
	Reasoning  string                           `json:"reasoning,omitempty"`
	Role       *MessageRole                     `json:"role"`
	ToolCallId string                           `json:"tool_call_id,omitempty"`
	ToolCalls  []*ChatCompletionMessageToolCall `json:"tool_calls,omitempty"`
}

// Model represents a Model in the API
type Model struct {
	Created  int64  `json:"created,omitempty"`
	ID       string `json:"id,omitempty"`
	Object   string `json:"object,omitempty"`
	OwnedBy  string `json:"owned_by,omitempty"`
	ServedBy string `json:"served_by,omitempty"`
}

// Transform converts provider-specific response to common format
func (p *CreateChatCompletionResponse) Transform() CreateChatCompletionResponse {
	return *p
}
