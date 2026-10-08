package middlewares

import (
	"bytes"
	"cmp"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	codes "go.opentelemetry.io/otel/codes"
	semconv "go.opentelemetry.io/otel/semconv/v1.41.0"
	trace "go.opentelemetry.io/otel/trace"

	config "github.com/inference-gateway/inference-gateway/config"
	mcp "github.com/inference-gateway/inference-gateway/internal/mcp"
	logger "github.com/inference-gateway/inference-gateway/internal/platform/logger"
	otel "github.com/inference-gateway/inference-gateway/internal/platform/otel"
	registry "github.com/inference-gateway/inference-gateway/providers/registry"
	routing "github.com/inference-gateway/inference-gateway/providers/routing"
	types "github.com/inference-gateway/inference-gateway/providers/types"
)

type TelemetryMiddleware struct {
	cfg       config.Config
	telemetry otel.OpenTelemetry
	logger    logger.Logger
}

func NewTelemetryMiddleware(cfg config.Config, telemetry otel.OpenTelemetry, logger logger.Logger) *TelemetryMiddleware {
	return &TelemetryMiddleware{
		cfg:       cfg,
		telemetry: telemetry,
		logger:    logger,
	}
}

const (
	maxCapturedResponseBytes = 1 << 20
	maxTelemetryRequestBytes = 32 << 20
	usageTrailingChunks      = 4
)

// toolTypeStandard is the gen_ai.tool.type of a tool the client declared
// itself; MCP tools use mcp.ToolTypeMCP.
const toolTypeStandard = "standard_tool_use"

// responseBodyWriter is a wrapper for the response writer that captures the body
type responseBodyWriter struct {
	http.ResponseWriter
	body       *bytes.Buffer
	statusCode int
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
	if w.statusCode == 0 {
		w.statusCode = http.StatusOK
	}
	w.body.Write(b)
	if w.body.Len() > maxCapturedResponseBytes {
		w.body.Next(w.body.Len() - maxCapturedResponseBytes)
	}
	return w.ResponseWriter.Write(b)
}

// WriteHeader forwards the status through and records it for the metrics above.
func (w *responseBodyWriter) WriteHeader(code int) {
	w.statusCode = code
	w.ResponseWriter.WriteHeader(code)
}

func (w *responseBodyWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}

func (t *TelemetryMiddleware) Middleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			startTime := time.Now()

			if r.URL.Path != ChatCompletionsPath {
				next.ServeHTTP(w, r)
				return
			}

			var requestBody types.CreateChatCompletionRequest
			bodyBytes, err := io.ReadAll(io.LimitReader(r.Body, maxTelemetryRequestBytes+1))
			if err != nil {
				t.logger.Error("failed to read request body", err)
				WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "failed to read request body"})
				return
			}
			if len(bodyBytes) > maxTelemetryRequestBytes {
				WriteJSON(w, http.StatusRequestEntityTooLarge, map[string]string{"error": "request body too large"})
				return
			}
			r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
			_ = json.Unmarshal(bodyBytes, &requestBody)
			model := requestBody.Model

			provider := "unknown"
			if detected, _, ok := routing.ResolveProvider(r.URL.Query().Get("provider"), model); ok {
				if _, exists := registry.Registry[detected]; exists {
					provider = string(detected)
				}
			}

			rw := &responseBodyWriter{
				ResponseWriter: w,
				body:           &bytes.Buffer{},
			}

			next.ServeHTTP(rw, r)

			if provider == "unknown" {
				t.logger.Warn("unknown provider detected",
					"model", model,
					"path", r.URL.Path,
					"query", r.URL.RawQuery)
				return
			}

			// Post middleware begins
			statusCode := cmp.Or(rw.statusCode, http.StatusOK)
			duration := time.Since(startTime).Seconds()

			errorType := ""
			if statusCode >= http.StatusBadRequest {
				errorType = strconv.Itoa(statusCode)
			}

			span := trace.SpanFromContext(r.Context())
			span.SetAttributes(
				semconv.GenAIProviderNameKey.String(provider),
				semconv.GenAIRequestModel(model),
			)
			if errorType != "" {
				span.SetStatus(codes.Error, errorType)
				span.SetAttributes(semconv.ErrorTypeKey.String(errorType))
			}

			team := otel.TeamUnknown
			t.telemetry.RecordRequestDuration(r.Context(), otel.SourceGateway, team, provider, model, errorType, duration)

			respData := t.parseResponseData(rw.body.Bytes(), requestBody.Stream != nil && *requestBody.Stream, provider, model)

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
				r.Context(),
				otel.SourceGateway,
				team,
				provider,
				model,
				promptTokens,
				completionTokens,
			)

			t.recordToolCallMetrics(r.Context(), team, provider, model, &requestBody, respData)
		})
	}
}

// parseResponseData extracts all needed information from response in a single pass
func (t *TelemetryMiddleware) parseResponseData(responseBytes []byte, isStreaming bool, provider, model string) *responseData {
	if isStreaming {
		return t.parseStreamingResponse(responseBytes, provider, model)
	}
	return t.parseNonStreamingResponse(responseBytes, provider, model)
}

// parseStreamingResponse handles streaming response parsing for both tokens and tool calls
func (t *TelemetryMiddleware) parseStreamingResponse(responseBytes []byte, provider, model string) *responseData {
	data := &responseData{}
	responseStr := string(responseBytes)
	chunks := strings.Split(responseStr, "\n\n")

	usageChunks := chunks
	if len(chunks) > usageTrailingChunks {
		usageChunks = chunks[len(chunks)-usageTrailingChunks:]
	}

	for _, chunk := range usageChunks {
		if chunk == "" || !strings.HasPrefix(chunk, types.SSEDataPrefix) {
			continue
		}

		chunk = strings.TrimPrefix(chunk, types.SSEDataPrefix)
		if chunk == types.SSEDoneData {
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

		data.setUsage(streamResponse.Usage)
	}

	data.ToolCalls = types.AccumulateStreamingToolCalls(responseStr)
	return data
}

// parseNonStreamingResponse handles non-streaming response parsing for both tokens and tool calls
func (t *TelemetryMiddleware) parseNonStreamingResponse(responseBytes []byte, provider, model string) *responseData {
	data := &responseData{}
	var chatCompletionResponse types.CreateChatCompletionResponse
	if err := json.Unmarshal(responseBytes, &chatCompletionResponse); err != nil {
		t.logger.Error("failed to unmarshal non-streaming response", err,
			"provider", provider,
			"model", model,
			"response_length", len(responseBytes))
		return data
	}

	data.setUsage(chatCompletionResponse.Usage)

	if len(chatCompletionResponse.Choices) > 0 && chatCompletionResponse.Choices[0].Message.ToolCalls != nil {
		data.ToolCalls = *chatCompletionResponse.Choices[0].Message.ToolCalls
	}
	return data
}

// setUsage copies token counts from usage when present.
func (d *responseData) setUsage(usage *types.CompletionUsage) {
	if usage == nil {
		return
	}
	d.PromptTokens = usage.PromptTokens
	d.CompletionTokens = usage.CompletionTokens
	d.TotalTokens = usage.TotalTokens
}

// recordToolCallMetrics analyzes the request and response to record comprehensive tool call metrics
func (t *TelemetryMiddleware) recordToolCallMetrics(ctx context.Context, team, provider, model string, request *types.CreateChatCompletionRequest, respData *responseData) {
	availableTools := make(map[string]string)
	if request.Tools != nil {
		for _, tool := range *request.Tools {
			toolType := classifyToolType(tool.Function.Name)
			availableTools[tool.Function.Name] = toolType
		}
	}

	for _, toolCall := range respData.ToolCalls {
		toolType, exists := availableTools[toolCall.Function.Name]
		if !exists {
			toolType = classifyToolType(toolCall.Function.Name)
		}

		t.telemetry.RecordToolCall(ctx, otel.SourceGateway, team, provider, model, toolType, toolCall.Function.Name)
	}
}

// classifyToolType determines the tool type based on the tool name
func classifyToolType(toolName string) string {
	if strings.HasPrefix(toolName, mcp.ToolNamePrefix) {
		return mcp.ToolTypeMCP
	}

	return toolTypeStandard
}
