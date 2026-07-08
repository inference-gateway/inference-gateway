package middlewares

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	gin "github.com/gin-gonic/gin"

	config "github.com/inference-gateway/inference-gateway/config"
	logger "github.com/inference-gateway/inference-gateway/logger"
	otel "github.com/inference-gateway/inference-gateway/otel"
	registry "github.com/inference-gateway/inference-gateway/providers/registry"
	routing "github.com/inference-gateway/inference-gateway/providers/routing"
	types "github.com/inference-gateway/inference-gateway/providers/types"
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

const (
	maxCapturedResponseBytes = 1 << 20
	maxTelemetryRequestBytes = 32 << 20
)

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
	if w.body.Len() > maxCapturedResponseBytes {
		w.body.Next(w.body.Len() - maxCapturedResponseBytes)
	}
	return w.ResponseWriter.Write(b)
}

func (w *responseBodyWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}

func (t *TelemetryImpl) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		startTime := time.Now()

		if !strings.Contains(c.Request.URL.Path, "/v1/chat/completions") {
			c.Next()
			return
		}

		var requestBody types.CreateChatCompletionRequest
		bodyBytes, err := io.ReadAll(io.LimitReader(c.Request.Body, maxTelemetryRequestBytes+1))
		if err != nil {
			t.logger.Error("failed to read request body", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": "failed to read request body"})
			c.Abort()
			return
		}
		if len(bodyBytes) > maxTelemetryRequestBytes {
			c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": "request body too large"})
			c.Abort()
			return
		}
		c.Request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
		_ = json.Unmarshal(bodyBytes, &requestBody)
		model := requestBody.Model

		provider := "unknown"
		if detected, _ := routing.DetermineProviderAndModelName(model); detected != nil {
			provider = string(*detected)
		} else if queried := types.Provider(c.Query("provider")); queried != "" {
			if _, exists := registry.Registry[queried]; exists {
				provider = string(queried)
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
		duration := time.Since(startTime).Seconds()

		errorType := ""
		if statusCode >= 400 {
			errorType = strconv.Itoa(statusCode)
		}
		team := otel.TeamUnknown
		t.telemetry.RecordRequestDuration(c.Request.Context(), otel.SourceGateway, team, provider, model, errorType, duration)

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
			"duration_seconds", duration,
			"status_code", statusCode,
		)

		t.telemetry.RecordTokenUsage(
			c.Request.Context(),
			otel.SourceGateway,
			team,
			provider,
			model,
			promptTokens,
			completionTokens,
		)

		t.recordToolCallMetrics(c.Request.Context(), team, provider, model, &requestBody, respData)
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

	return types.AccumulateStreamingToolCalls(responseStr)
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
func (t *TelemetryImpl) recordToolCallMetrics(ctx context.Context, team, provider, model string, request *types.CreateChatCompletionRequest, respData *responseData) {
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

		t.telemetry.RecordToolCall(ctx, otel.SourceGateway, team, provider, model, toolType, toolCall.Function.Name)
	}
}

// classifyToolType determines the tool type based on the tool name
func (t *TelemetryImpl) classifyToolType(toolName string) string {
	if strings.HasPrefix(toolName, "mcp_") {
		return "mcp"
	}

	return "standard_tool_use"
}
