package middlewares

import (
	"bytes"
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

		t.logger.Debug("Request URL", "url", c.Request.URL.Path)
		if !strings.Contains(c.Request.URL.Path, "/generate") {
			c.Next()
			return
		}

		t.logger.Debug("Intercepting request for token usage")

		var requestBody providers.GenerateRequest
		bodyBytes, _ := io.ReadAll(c.Request.Body)
		c.Request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
		_ = json.Unmarshal(bodyBytes, &requestBody)
		model := requestBody.Model

		provider := "unknown"
		switch {
		case strings.Contains(c.Request.URL.Path, "/openai/"):
			provider = "openai"
		case strings.Contains(c.Request.URL.Path, "/anthropic/"):
			provider = "anthropic"
		case strings.Contains(c.Request.URL.Path, "/groq/"):
			provider = "groq"
		case strings.Contains(c.Request.URL.Path, "/cohere/"):
			provider = "cohere"
		case strings.Contains(c.Request.URL.Path, "/ollama/"):
			provider = "ollama"
		}

		w := &responseBodyWriter{
			ResponseWriter: c.Writer,
			body:           &bytes.Buffer{},
		}
		c.Writer = w

		// For streaming responses, the token counts are in the final message
		// which was already processed if this is SSE
		if requestBody.Stream || strings.Contains(c.GetHeader("Content-Type"), "text/event-stream") {
			return // TODO - need to handle stream responses
		}

		c.Next()

		// Post middleware begins
		statusCode := c.Writer.Status()
		duration := float64(time.Since(startTime).Milliseconds())

		t.telemetry.RecordResponseStatus(c.Request.Context(), provider, c.Request.Method, c.Request.URL.Path, statusCode)
		t.telemetry.RecordRequestDuration(c.Request.Context(), provider, c.Request.Method, c.Request.URL.Path, duration)

		var responseData map[string]any
		if err := json.Unmarshal(w.body.Bytes(), &responseData); err == nil {
			if usage, ok := responseData["usage"].(map[string]any); ok {

				promptTokens := int64(usage["prompt_tokens"].(float64))
				completionTokens := int64(usage["completion_tokens"].(float64))
				totalTokens := int64(usage["total_tokens"].(float64))
				queueTime := usage["queue_time"].(float64)
				promptTime := usage["prompt_time"].(float64)
				compTime := usage["completion_time"].(float64)
				totalTime := usage["total_time"].(float64)

				t.logger.Debug("Tokens usage",
					"provider", provider,
					"model", model,
					"promptTokens", promptTokens,
					"completionTokens", completionTokens,
					"totalTokens", totalTokens,
				)

				t.logger.Debug("Tokens Latency",
					"queueTime", queueTime,
					"promptTime", promptTime,
					"compTime", compTime,
					"totalTime", totalTime,
				)

				t.telemetry.RecordTokenUsage(
					c.Request.Context(),
					provider,
					model,
					promptTokens,
					completionTokens,
					totalTokens,
				)

				t.telemetry.RecordLatency(
					c.Request.Context(),
					provider,
					model,
					queueTime,
					promptTime,
					compTime,
					totalTime,
				)
			}
		}
	}
}
