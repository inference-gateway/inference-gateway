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
			t.logger.Debug("Unknown provider", "model", model)
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
		if requestBody.Stream {
			responseStr := w.body.String()
			chunks := strings.Split(responseStr, "\n\n")
			// We only care about the chunk before the last one which is [DONE]
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
						t.logger.Error("telemetry middleware - failed to unmarshal response", err)
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
				t.logger.Error("telemetry middleware - failed to unmarshal response", err)
			}

			if chatCompletionResponse.Usage != nil {
				promptTokens = chatCompletionResponse.Usage.PromptTokens
				completionTokens = chatCompletionResponse.Usage.CompletionTokens
				totalTokens = chatCompletionResponse.Usage.TotalTokens
			}
		}

		// t.logger.Debug("Tokens usage",
		// 	"provider", provider,
		// 	"model", model,
		// 	"promptTokens", promptTokens,
		// 	"completionTokens", completionTokens,
		// 	"totalTokens", totalTokens,
		// )

		t.telemetry.RecordTokenUsage(
			c.Request.Context(),
			provider,
			model,
			promptTokens,
			completionTokens,
			totalTokens,
		)
	}
}
