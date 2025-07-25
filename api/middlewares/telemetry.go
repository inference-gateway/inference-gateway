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
	"github.com/inference-gateway/inference-gateway/providers"
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

		var requestBody providers.CreateChatCompletionRequest
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

		var promptTokens int64
		var completionTokens int64
		var totalTokens int64
		if requestBody.Stream != nil && *requestBody.Stream {
			responseStr := w.body.String()
			chunks := strings.Split(responseStr, "\n\n")

			if len(chunks) > 4 {
				chunks = chunks[len(chunks)-4:]
			}

			var chatCompletionStreamResponse providers.CreateChatCompletionStreamResponse
			for _, chunk := range chunks {
				if chunk == "" {
					continue
				}

				if strings.HasPrefix(chunk, "data: ") {
					chunk = strings.TrimPrefix(chunk, "data: ")

					if chunk == "[DONE]" {
						break
					}

					if err := json.Unmarshal([]byte(chunk), &chatCompletionStreamResponse); err != nil {
						t.logger.Error("failed to unmarshal streaming response chunk", err,
							"provider", provider,
							"model", model,
							"chunk_length", len(chunk))
						break
					}

					if chatCompletionStreamResponse.Usage != nil {
						promptTokens = chatCompletionStreamResponse.Usage.PromptTokens
						completionTokens = chatCompletionStreamResponse.Usage.CompletionTokens
						totalTokens = chatCompletionStreamResponse.Usage.TotalTokens
						break
					}
				}
			}
		} else {
			var chatCompletionResponse providers.CreateChatCompletionResponse
			if err := json.Unmarshal(w.body.Bytes(), &chatCompletionResponse); err != nil {
				t.logger.Error("failed to unmarshal non-streaming response", err,
					"provider", provider,
					"model", model,
					"response_length", w.body.Len(),
					"status_code", statusCode)
			}

			if chatCompletionResponse.Usage != nil {
				promptTokens = chatCompletionResponse.Usage.PromptTokens
				completionTokens = chatCompletionResponse.Usage.CompletionTokens
				totalTokens = chatCompletionResponse.Usage.TotalTokens
			}
		}

		var toolCallCount int
		if requestBody.Stream != nil && *requestBody.Stream {
			responseStr := w.body.String()
			chunks := strings.Split(responseStr, "\n\n")
			for _, chunk := range chunks {
				if strings.HasPrefix(chunk, "data: ") {
					chunk = strings.TrimPrefix(chunk, "data: ")
					if chunk == "[DONE]" {
						continue
					}
					var streamResponse providers.CreateChatCompletionStreamResponse
					if err := json.Unmarshal([]byte(chunk), &streamResponse); err == nil {
						if len(streamResponse.Choices) > 0 && streamResponse.Choices[0].Delta.ToolCalls != nil {
							toolCallCount += len(*streamResponse.Choices[0].Delta.ToolCalls)
						}
					}
				}
			}
		} else {
			var chatCompletionResponse providers.CreateChatCompletionResponse
			if err := json.Unmarshal(w.body.Bytes(), &chatCompletionResponse); err == nil {
				if len(chatCompletionResponse.Choices) > 0 && chatCompletionResponse.Choices[0].Message.ToolCalls != nil {
					toolCallCount = len(*chatCompletionResponse.Choices[0].Message.ToolCalls)
				}
			}
		}

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

		t.recordToolCallMetrics(c.Request.Context(), provider, model, &requestBody, w.body.Bytes())
	}
}

// recordToolCallMetrics analyzes the request and response to record comprehensive tool call metrics
func (t *TelemetryImpl) recordToolCallMetrics(ctx context.Context, provider, model string, request *providers.CreateChatCompletionRequest, responseBytes []byte) {
	availableTools := make(map[string]string) // tool_name -> tool_type
	if request.Tools != nil {
		for _, tool := range *request.Tools {
			toolType := t.classifyToolType(tool.Function.Name)
			availableTools[tool.Function.Name] = toolType
		}
	}

	var actualToolCalls []providers.ChatCompletionMessageToolCall
	if request.Stream != nil && *request.Stream {
		actualToolCalls = t.parseStreamingToolCalls(responseBytes)
	} else {
		actualToolCalls = t.parseNonStreamingToolCalls(responseBytes)
	}

	for _, toolCall := range actualToolCalls {
		toolType, exists := availableTools[toolCall.Function.Name]
		if !exists {
			toolType = t.classifyToolType(toolCall.Function.Name)
		}

		t.telemetry.RecordToolCallCount(ctx, provider, model, toolType, toolCall.Function.Name)
	}
}

// classifyToolType determines the tool type based on the tool name
func (t *TelemetryImpl) classifyToolType(toolName string) string {
	if strings.HasPrefix(toolName, "a2a_") {
		return "a2a"
	}

	if strings.HasPrefix(toolName, "mcp_") {
		return "mcp"
	}

	return "llm_response"
}

// parseStreamingToolCalls extracts tool calls from streaming response
func (t *TelemetryImpl) parseStreamingToolCalls(responseBytes []byte) []providers.ChatCompletionMessageToolCall {
	responseStr := string(responseBytes)
	chunks := strings.Split(responseStr, "\n\n")
	toolCallsMap := make(map[int]*providers.ChatCompletionMessageToolCall)

	for _, chunk := range chunks {
		if !strings.HasPrefix(chunk, "data: ") {
			continue
		}
		chunk = strings.TrimPrefix(chunk, "data: ")
		if chunk == "[DONE]" || chunk == "" {
			continue
		}

		var streamResponse providers.CreateChatCompletionStreamResponse
		if err := json.Unmarshal([]byte(chunk), &streamResponse); err != nil {
			continue
		}

		if len(streamResponse.Choices) == 0 || streamResponse.Choices[0].Delta.ToolCalls == nil {
			continue
		}

		for _, toolCallChunk := range *streamResponse.Choices[0].Delta.ToolCalls {
			index := toolCallChunk.Index
			if _, exists := toolCallsMap[index]; !exists {
				toolCallsMap[index] = &providers.ChatCompletionMessageToolCall{
					ID:       "",
					Type:     providers.ChatCompletionToolTypeFunction,
					Function: providers.ChatCompletionMessageToolCallFunction{Name: "", Arguments: ""},
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

	var toolCalls []providers.ChatCompletionMessageToolCall
	for i := 0; i < len(toolCallsMap); i++ {
		if toolCall, exists := toolCallsMap[i]; exists && toolCall.Function.Name != "" {
			toolCalls = append(toolCalls, *toolCall)
		}
	}

	return toolCalls
}

// parseNonStreamingToolCalls extracts tool calls from non-streaming response
func (t *TelemetryImpl) parseNonStreamingToolCalls(responseBytes []byte) []providers.ChatCompletionMessageToolCall {
	var chatCompletionResponse providers.CreateChatCompletionResponse
	if err := json.Unmarshal(responseBytes, &chatCompletionResponse); err != nil {
		return nil
	}

	if len(chatCompletionResponse.Choices) == 0 || chatCompletionResponse.Choices[0].Message.ToolCalls == nil {
		return nil
	}

	return *chatCompletionResponse.Choices[0].Message.ToolCalls
}
