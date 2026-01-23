package middlewares

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/inference-gateway/inference-gateway/config"
	"github.com/inference-gateway/inference-gateway/logger"
	"github.com/inference-gateway/inference-gateway/otel"
	"github.com/inference-gateway/inference-gateway/providers/types"
)

type Telemetry interface {
	Middleware() gin.HandlerFunc
}

type TelemetryImpl struct {
	cfg       config.Config
	telemetry otel.OpenTelemetry
	logger    logger.Logger
}

func NewTelemetryMiddleware(cfg config.Config, telemetry otel.OpenTelemetry, logger logger.Logger) (Telemetry, error) {
	return &TelemetryImpl{
		cfg:       cfg,
		telemetry: telemetry,
		logger:    logger,
	}, nil
}

// responseBodyWriter is a wrapper for the response writer that captures the body
type responseBodyWriter struct {
	gin.ResponseWriter
	body *bytes.Buffer
}

// responseData holds all information extracted from a single response parse
type responseData struct {
	PromptTokens     int64
	CompletionTokens int64
	TotalTokens      int64
	ToolCalls        []types.ChatCompletionMessageToolCall
}

// Write captures the response body
func (w *responseBodyWriter) Write(b []byte) (int, error) {
	w.body.Write(b)
	return w.ResponseWriter.Write(b)
}

func (t *TelemetryImpl) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		startTime := time.Now()

		if !strings.Contains(c.Request.URL.Path, "/v1/chat/completions") {
			c.Next()
			return
		}

		var requestBody types.CreateChatCompletionRequest
		bodyBytes, _ := io.ReadAll(c.Request.Body)
		c.Request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
		_ = json.Unmarshal(bodyBytes, &requestBody)
		model := requestBody.Model

		provider := "unknown"
		switch {
		case strings.HasPrefix(model, "openai/"):
			provider = "openai"
		case strings.HasPrefix(model, "anthropic/"):
			provider = "anthropic"
		case strings.HasPrefix(model, "groq/"):
			provider = "groq"
		case strings.HasPrefix(model, "cohere/"):
			provider = "cohere"
		case strings.HasPrefix(model, "ollama/"):
			provider = "ollama"
		case strings.HasPrefix(model, "cloudflare/"):
			provider = "cloudflare"
		case strings.HasPrefix(model, "deepseek/"):
			provider = "deepseek"
		case strings.HasPrefix(model, "google/"):
			provider = "google"
		case strings.HasPrefix(model, "mistral/"):
			provider = "mistral"
		case strings.HasPrefix(model, "moonshot/"):
			provider = "moonshot"
		case strings.HasPrefix(model, "ollama_cloud/"):
			provider = "ollama_cloud"
		}

		if provider == "unknown" {
			switch {
			case strings.Contains(c.Request.URL.RawQuery, "openai"):
				provider = "openai"
			case strings.Contains(c.Request.URL.RawQuery, "anthropic"):
				provider = "anthropic"
			case strings.Contains(c.Request.URL.RawQuery, "groq"):
				provider = "groq"
			case strings.Contains(c.Request.URL.RawQuery, "cohere"):
				provider = "cohere"
			case strings.Contains(c.Request.URL.RawQuery, "ollama"):
				provider = "ollama"
			case strings.Contains(c.Request.URL.RawQuery, "cloudflare"):
				provider = "cloudflare"
			case strings.Contains(c.Request.URL.RawQuery, "deepseek"):
				provider = "deepseek"
			case strings.Contains(c.Request.URL.RawQuery, "google"):
				provider = "google"
			case strings.Contains(c.Request.URL.RawQuery, "mistral"):
				provider = "mistral"
			case strings.Contains(c.Request.URL.RawQuery, "moonshot"):
				provider = "moonshot"
			}
		}

		w := &responseBodyWriter{
			ResponseWriter: c.Writer,
			body:           &bytes.Buffer{},
		}
		c.Writer = w

		c.Next()

		if provider == "unknown" {
			t.logger.Warn("unknown provider detected",
				"model", model,
				"path", c.Request.URL.Path,
				"query", c.Request.URL.RawQuery)
			return
		}

		// Post middleware begins
		statusCode := c.Writer.Status()
		duration := float64(time.Since(startTime).Milliseconds())

		t.telemetry.RecordResponseStatus(c.Request.Context(), provider, c.Request.Method, c.Request.URL.Path, statusCode)
		t.telemetry.RecordRequestDuration(c.Request.Context(), provider, c.Request.Method, c.Request.URL.Path, duration)

		respData := t.parseResponseData(w.body.Bytes(), requestBody.Stream != nil && *requestBody.Stream, provider, model)

		promptTokens := respData.PromptTokens
		completionTokens := respData.CompletionTokens
		totalTokens := respData.TotalTokens
		toolCallCount := len(respData.ToolCalls)

		t.logger.Debug("token usage recorded",
			"provider", provider,
			"model", model,
			"prompt_tokens", promptTokens,
			"completion_tokens", completionTokens,
			"total_tokens", totalTokens,
			"tool_calls", toolCallCount,
			"duration_ms", duration,
			"status_code", statusCode,
		)

		t.telemetry.RecordTokenUsage(
			c.Request.Context(),
			provider,
			model,
			promptTokens,
			completionTokens,
			totalTokens,
		)

		t.recordToolCallMetrics(c.Request.Context(), provider, model, &requestBody, respData)
	}
}

// parseResponseData extracts all needed information from response in a single pass
func (t *TelemetryImpl) parseResponseData(responseBytes []byte, isStreaming bool, provider, model string) *responseData {
	data := &responseData{}

	if isStreaming {
		data.ToolCalls = t.parseStreamingResponse(responseBytes, &data.PromptTokens, &data.CompletionTokens, &data.TotalTokens, provider, model)
	} else {
		data.ToolCalls = t.parseNonStreamingResponse(responseBytes, &data.PromptTokens, &data.CompletionTokens, &data.TotalTokens, provider, model)
	}

	return data
}

// parseStreamingResponse handles streaming response parsing for both tokens and tool calls
func (t *TelemetryImpl) parseStreamingResponse(responseBytes []byte, promptTokens, completionTokens, totalTokens *int64, provider, model string) []types.ChatCompletionMessageToolCall {
	responseStr := string(responseBytes)
	chunks := strings.Split(responseStr, "\n\n")
	toolCallsMap := make(map[int]*types.ChatCompletionMessageToolCall)

	usageChunks := chunks
	if len(chunks) > 4 {
		usageChunks = chunks[len(chunks)-4:]
	}

	for _, chunk := range usageChunks {
		if chunk == "" || !strings.HasPrefix(chunk, "data: ") {
			continue
		}

		chunk = strings.TrimPrefix(chunk, "data: ")
		if chunk == "[DONE]" {
			continue
		}

		var streamResponse types.CreateChatCompletionStreamResponse
		if err := json.Unmarshal([]byte(chunk), &streamResponse); err != nil {
			t.logger.Error("failed to unmarshal streaming response chunk", err,
				"provider", provider,
				"model", model,
				"chunk_length", len(chunk))
			continue
		}

		if streamResponse.Usage != nil {
			*promptTokens = streamResponse.Usage.PromptTokens
			*completionTokens = streamResponse.Usage.CompletionTokens
			*totalTokens = streamResponse.Usage.TotalTokens
		}
	}

	for _, chunk := range chunks {
		if !strings.HasPrefix(chunk, "data: ") {
			continue
		}
		chunk = strings.TrimPrefix(chunk, "data: ")
		if chunk == "[DONE]" || chunk == "" {
			continue
		}

		var streamResponse types.CreateChatCompletionStreamResponse
		if err := json.Unmarshal([]byte(chunk), &streamResponse); err != nil {
			continue
		}

		if len(streamResponse.Choices) == 0 || streamResponse.Choices[0].Delta.ToolCalls == nil {
			continue
		}

		for _, toolCallChunk := range *streamResponse.Choices[0].Delta.ToolCalls {
			index := toolCallChunk.Index
			if _, exists := toolCallsMap[index]; !exists {
				toolCallsMap[index] = &types.ChatCompletionMessageToolCall{
					ID:       "",
					Type:     types.Function,
					Function: types.ChatCompletionMessageToolCallFunction{Name: "", Arguments: ""},
				}
			}

			toolCall := toolCallsMap[index]
			if toolCallChunk.ID != nil {
				toolCall.ID = *toolCallChunk.ID
			}
			if toolCallChunk.Function != nil {
				if toolCallChunk.Function.Name != "" {
					toolCall.Function.Name = toolCallChunk.Function.Name
				}
				if toolCallChunk.Function.Arguments != "" {
					toolCall.Function.Arguments += toolCallChunk.Function.Arguments
				}
			}
		}
	}

	var toolCalls []types.ChatCompletionMessageToolCall
	for i := 0; i < len(toolCallsMap); i++ {
		if toolCall, exists := toolCallsMap[i]; exists && toolCall.Function.Name != "" {
			toolCalls = append(toolCalls, *toolCall)
		}
	}

	return toolCalls
}

// parseNonStreamingResponse handles non-streaming response parsing for both tokens and tool calls
func (t *TelemetryImpl) parseNonStreamingResponse(responseBytes []byte, promptTokens, completionTokens, totalTokens *int64, provider, model string) []types.ChatCompletionMessageToolCall {
	var chatCompletionResponse types.CreateChatCompletionResponse
	if err := json.Unmarshal(responseBytes, &chatCompletionResponse); err != nil {
		t.logger.Error("failed to unmarshal non-streaming response", err,
			"provider", provider,
			"model", model,
			"response_length", len(responseBytes))
		return nil
	}

	if chatCompletionResponse.Usage != nil {
		*promptTokens = chatCompletionResponse.Usage.PromptTokens
		*completionTokens = chatCompletionResponse.Usage.CompletionTokens
		*totalTokens = chatCompletionResponse.Usage.TotalTokens
	}

	if len(chatCompletionResponse.Choices) == 0 || chatCompletionResponse.Choices[0].Message.ToolCalls == nil {
		return nil
	}

	return *chatCompletionResponse.Choices[0].Message.ToolCalls
}

// recordToolCallMetrics analyzes the request and response to record comprehensive tool call metrics
func (t *TelemetryImpl) recordToolCallMetrics(ctx context.Context, provider, model string, request *types.CreateChatCompletionRequest, respData *responseData) {
	availableTools := make(map[string]string) // tool_name -> tool_type
	if request.Tools != nil {
		for _, tool := range *request.Tools {
			toolType := t.classifyToolType(tool.Function.Name)
			availableTools[tool.Function.Name] = toolType
		}
	}

	for _, toolCall := range respData.ToolCalls {
		toolType, exists := availableTools[toolCall.Function.Name]
		if !exists {
			toolType = t.classifyToolType(toolCall.Function.Name)
		}

		t.telemetry.RecordToolCallCount(ctx, provider, model, toolType, toolCall.Function.Name)
	}
}

// classifyToolType determines the tool type based on the tool name
func (t *TelemetryImpl) classifyToolType(toolName string) string {
	if strings.HasPrefix(toolName, "mcp_") {
		return "mcp"
	}

	return "standard_tool_use"
}
