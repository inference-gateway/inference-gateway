package middlewares

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	gin "github.com/gin-gonic/gin"

	config "github.com/inference-gateway/inference-gateway/config"
	guardrails "github.com/inference-gateway/inference-gateway/internal/guardrails"
	logger "github.com/inference-gateway/inference-gateway/logger"
	otel "github.com/inference-gateway/inference-gateway/otel"
)

// GuardrailsMiddleware defines the interface for guardrails middleware.
type GuardrailsMiddleware interface {
	Middleware() gin.HandlerFunc
}

// GuardrailsMiddlewareImpl implements the guardrails middleware.
type GuardrailsMiddlewareImpl struct {
	evaluator      *guardrails.Evaluator
	externalClient *guardrails.ExternalClient
	detectors      []guardrails.Detector
	logger         logger.Logger
	telemetry      otel.OpenTelemetry
	cfg            config.Config
}

// NoopGuardrailsMiddlewareImpl is a no-op implementation of GuardrailsMiddleware.
type NoopGuardrailsMiddlewareImpl struct{}

// NewGuardrailsMiddleware creates a new guardrails middleware instance.
// Returns a Noop implementation when guardrails are disabled.
func NewGuardrailsMiddleware(
	evaluator *guardrails.Evaluator,
	externalClient *guardrails.ExternalClient,
	detectors []guardrails.Detector,
	log logger.Logger,
	telemetry otel.OpenTelemetry,
	cfg config.Config,
) GuardrailsMiddleware {
	if !cfg.Guardrails.Enabled {
		log.Info("guardrails disabled, using no-op middleware")
		return &NoopGuardrailsMiddlewareImpl{}
	}

	return &GuardrailsMiddlewareImpl{
		evaluator:      evaluator,
		externalClient: externalClient,
		detectors:      detectors,
		logger:         log,
		telemetry:      telemetry,
		cfg:            cfg,
	}
}

// Middleware returns the no-op middleware handler.
func (n *NoopGuardrailsMiddlewareImpl) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()
	}
}

// Middleware returns the guardrails middleware handler.
func (m *GuardrailsMiddlewareImpl) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Guardrails apply to all paths - they sit before MCP in the chain,
		// so X-MCP-Bypass is irrelevant (guardrails never check it).
		path := c.Request.URL.Path

		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, int64(m.cfg.Server.MaxRequestBodySize))
		bodyBytes, err := c.GetRawData()
		if err != nil {
			m.logger.Error("guardrails: failed to read request body", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": "failed to read request body"})
			c.Abort()
			return
		}
		c.Request.Body = io.NopCloser(bytes.NewReader(bodyBytes))

		model := extractModel(bodyBytes, path)
		input := &guardrails.Input{
			Method: c.Request.Method,
			Path:   path,
			Phase:  guardrails.PhasePreCall,
			Request: &guardrails.Req{
				Body:  string(bodyBytes),
				Model: model,
			},
		}

		dec, err := m.evaluate(c.Request.Context(), input)
		if err != nil {
			m.logger.Error("guardrails: pre_call evaluation error", err)
			if m.cfg.Guardrails.FailMode == guardrails.FailModeClosed {
				c.JSON(http.StatusForbidden, gin.H{
					"error":   "guardrail evaluation failed",
					"message": "request blocked by guardrails",
				})
				c.Abort()
				return
			}
			m.logger.Warn("guardrails: pre_call evaluation error, allowing in open mode", "error", err.Error())
			c.Next()
			return
		}

		if dec.Action == guardrails.ActionBlock {
			m.logger.Info("guardrails: request blocked", "path", path, "message", dec.Message)
			if m.telemetry != nil {
				m.telemetry.RecordGuardrail(c.Request.Context(), otel.SourceGateway, guardrails.PhasePreCall, guardrails.ActionBlock, path, model)
			}
			c.JSON(http.StatusForbidden, gin.H{
				"error":   "request blocked by guardrails",
				"message": dec.Message,
			})
			c.Abort()
			return
		}

		if dec.Action == guardrails.ActionRedact {
			redacted := guardrails.RedactSensitive(string(bodyBytes), m.detectors)
			c.Request.Body = http.MaxBytesReader(c.Writer, io.NopCloser(bytes.NewReader([]byte(redacted))), int64(m.cfg.Server.MaxRequestBodySize))
			m.logger.Debug("guardrails: request body redacted", "path", path)
		}

		if m.telemetry != nil {
			m.telemetry.RecordGuardrail(c.Request.Context(), otel.SourceGateway, guardrails.PhasePreCall, dec.Action, path, model)
		}

		if path == ChatCompletionsPath && !isStreamingRequest(bodyBytes) {
			customWriter := &customResponseWriter{
				ResponseWriter: c.Writer,
				body:           &bytes.Buffer{},
				statusCode:     http.StatusOK,
				writeToClient:  false,
			}
			c.Writer = customWriter

			c.Next()

			if customWriter.statusCode >= http.StatusBadRequest {
				c.Writer = customWriter.ResponseWriter
				c.Data(customWriter.statusCode, customWriter.Header().Get("Content-Type"), customWriter.body.Bytes())
				return
			}

			respInput := &guardrails.Input{
				Method: c.Request.Method,
				Path:   path,
				Phase:  guardrails.PhasePostCall,
				Request: &guardrails.Req{
					Body:  customWriter.body.String(),
					Model: model,
				},
			}

			respDec, respErr := m.evaluate(c.Request.Context(), respInput)
			if respErr != nil {
				m.logger.Error("guardrails: post_call evaluation error", respErr)
				if m.cfg.Guardrails.FailMode == guardrails.FailModeClosed {
					c.Writer = customWriter.ResponseWriter
					c.JSON(http.StatusForbidden, gin.H{
						"error":   "guardrail evaluation failed",
						"message": "response blocked by guardrails",
					})
					return
				}
				m.logger.Warn("guardrails: post_call evaluation error, allowing in open mode", "error", respErr.Error())
			}

			if respDec.Action == guardrails.ActionBlock {
				m.logger.Info("guardrails: response blocked", "path", path, "message", respDec.Message)
				if m.telemetry != nil {
					m.telemetry.RecordGuardrail(c.Request.Context(), otel.SourceGateway, guardrails.PhasePostCall, guardrails.ActionBlock, path, model)
				}
				c.Writer = customWriter.ResponseWriter
				c.JSON(http.StatusForbidden, gin.H{
					"error":   "response blocked by guardrails",
					"message": respDec.Message,
				})
				return
			}

			if respDec.Action == guardrails.ActionRedact {
				redactedBody := guardrails.RedactSensitive(customWriter.body.String(), m.detectors)
				c.Writer = customWriter.ResponseWriter
				c.Data(customWriter.statusCode, customWriter.Header().Get("Content-Type"), []byte(redactedBody))
				if m.telemetry != nil {
					m.telemetry.RecordGuardrail(c.Request.Context(), otel.SourceGateway, guardrails.PhasePostCall, guardrails.ActionRedact, path, model)
				}
				return
			}

			if m.telemetry != nil {
				m.telemetry.RecordGuardrail(c.Request.Context(), otel.SourceGateway, guardrails.PhasePostCall, respDec.Action, path, model)
			}

			c.Writer = customWriter.ResponseWriter
			c.Data(customWriter.statusCode, customWriter.Header().Get("Content-Type"), customWriter.body.Bytes())
			return
		}

		c.Next()
	}
}

// evaluate runs the policy evaluator and external guardrail check.
func (m *GuardrailsMiddlewareImpl) evaluate(ctx context.Context, input *guardrails.Input) (guardrails.Decision, error) {
	dec, err := m.evaluator.Eval(ctx, input)
	if err != nil {
		return guardrails.Decision{}, err
	}

	if m.externalClient != nil {
		extDec, extErr := m.externalClient.Check(ctx, input)
		if extErr != nil {
			return guardrails.Decision{}, extErr
		}
		if extDec.Action == guardrails.ActionBlock {
			return *extDec, nil
		}
	}

	return dec, nil
}

// extractModel attempts to extract the model name from the request body or path.
func extractModel(body []byte, path string) string {
	if path == ChatCompletionsPath || strings.Contains(path, "/v1/responses") {
		var req struct {
			Model string `json:"model"`
		}
		if err := json.Unmarshal(body, &req); err == nil && req.Model != "" {
			return req.Model
		}
	}
	return ""
}

// isStreamingRequest checks if the request has stream=true.
func isStreamingRequest(body []byte) bool {
	var req struct {
		Stream *bool `json:"stream"`
	}
	if err := json.Unmarshal(body, &req); err == nil && req.Stream != nil && *req.Stream {
		return true
	}
	return false
}
