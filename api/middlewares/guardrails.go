package middlewares

import (
	"bytes"
	"cmp"
	"context"
	"encoding/json"
	"io"
	"net/http"

	gin "github.com/gin-gonic/gin"

	config "github.com/inference-gateway/inference-gateway/config"
	a2a "github.com/inference-gateway/inference-gateway/internal/a2a"
	guardrails "github.com/inference-gateway/inference-gateway/internal/guardrails"
	logger "github.com/inference-gateway/inference-gateway/internal/platform/logger"
	otel "github.com/inference-gateway/inference-gateway/internal/platform/otel"
	types "github.com/inference-gateway/inference-gateway/providers/types"
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
		path := c.Request.URL.Path

		maxBodySize := int64(m.cfg.Server.ResolveMaxRequestBodySize())
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxBodySize)
		bodyBytes, err := c.GetRawData()
		if err != nil {
			m.logger.Error("guardrails: failed to read request body", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": "failed to read request body"})
			c.Abort()
			return
		}
		c.Request.Body = io.NopCloser(bytes.NewReader(bodyBytes))

		model := extractModel(bodyBytes, path)
		claims, _ := c.Request.Context().Value(types.ClaimsContextKey).(map[string]any)
		input := &guardrails.Input{
			Method: c.Request.Method,
			Path:   path,
			Phase:  guardrails.PhasePreCall,
			Request: &guardrails.Req{
				Body:  string(bodyBytes),
				Model: model,
			},
			Identity: claims,
		}

		dec, err := m.evaluate(c.Request.Context(), input)
		if err != nil {
			m.logger.Error("guardrails: pre_call evaluation error", err)
			if m.cfg.Guardrails.FailMode == guardrails.FailModeClosed {
				m.block(c, path, bodyBytes, guardrails.MsgEvaluationFailed, guardrails.MsgBlocked)
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
				m.telemetry.RecordGuardrail(c.Request.Context(), otel.SourceGateway, string(guardrails.PhasePreCall), guardrails.ActionBlock, path, model)
			}
			m.block(c, path, bodyBytes, guardrails.MsgBlocked, cmp.Or(dec.Message, guardrails.MsgBlocked))
			c.Abort()
			return
		}

		if dec.Action == guardrails.ActionRedact {
			redacted := guardrails.RedactSensitive(string(bodyBytes), m.detectors)
			c.Request.Body = http.MaxBytesReader(c.Writer, io.NopCloser(bytes.NewReader([]byte(redacted))), maxBodySize)
			m.logger.Debug("guardrails: request body redacted", "path", path)
		}

		if m.telemetry != nil {
			m.telemetry.RecordGuardrail(c.Request.Context(), otel.SourceGateway, string(guardrails.PhasePreCall), dec.Action, path, model)
		}

		if !capturesResponse(path, bodyBytes) {
			c.Next()
			return
		}

		customWriter := captureResponse(c)
		c.Next()
		if customWriter.statusCode >= http.StatusBadRequest {
			customWriter.replay(c)
			return
		}
		m.evaluateResponse(c, customWriter, path, model, claims, bodyBytes)
	}
}

// evaluateResponse runs post_call over the captured response and replays,
// redacts or blocks it.
func (m *GuardrailsMiddlewareImpl) evaluateResponse(c *gin.Context, customWriter *customResponseWriter, path, model string, claims map[string]any, requestBody []byte) {
	respInput := &guardrails.Input{
		Method: c.Request.Method,
		Path:   path,
		Phase:  guardrails.PhasePostCall,
		Request: &guardrails.Req{
			Body:  customWriter.body.String(),
			Model: model,
		},
		Identity: claims,
	}

	respDec, respErr := m.evaluate(c.Request.Context(), respInput)
	if respErr != nil {
		m.logger.Error("guardrails: post_call evaluation error", respErr)
		if m.cfg.Guardrails.FailMode == guardrails.FailModeClosed {
			c.Writer = customWriter.ResponseWriter
			m.block(c, path, requestBody, msgResponseEvaluationFailed, msgResponseBlocked)
			return
		}
		m.logger.Warn("guardrails: post_call evaluation error, allowing in open mode", "error", respErr.Error())
	}

	if respDec.Action == guardrails.ActionBlock {
		m.logger.Info("guardrails: response blocked", "path", path, "message", respDec.Message)
		if m.telemetry != nil {
			m.telemetry.RecordGuardrail(c.Request.Context(), otel.SourceGateway, string(guardrails.PhasePostCall), guardrails.ActionBlock, path, model)
		}
		c.Writer = customWriter.ResponseWriter
		m.block(c, path, requestBody, msgResponseBlocked, cmp.Or(respDec.Message, msgResponseBlocked))
		return
	}

	if respDec.Action == guardrails.ActionRedact {
		redactedBody := guardrails.RedactSensitive(customWriter.body.String(), m.detectors)
		c.Writer = customWriter.ResponseWriter
		c.Data(customWriter.statusCode, customWriter.Header().Get("Content-Type"), []byte(redactedBody))
		if m.telemetry != nil {
			m.telemetry.RecordGuardrail(c.Request.Context(), otel.SourceGateway, string(guardrails.PhasePostCall), guardrails.ActionRedact, path, model)
		}
		return
	}

	if m.telemetry != nil {
		m.telemetry.RecordGuardrail(c.Request.Context(), otel.SourceGateway, string(guardrails.PhasePostCall), respDec.Action, path, model)
	}

	customWriter.replay(c)
}

const (
	msgResponseEvaluationFailed = "guardrail evaluation failed"
	msgResponseBlocked          = "response blocked by guardrails"
)

// block writes the 403 for a guardrails refusal: a JSON-RPC error envelope on
// the /mcp and /a2a server endpoints, a plain error object everywhere else.
// The envelope carries the policy message when there is one and the cause
// (evaluation failure or generic block) otherwise.
func (m *GuardrailsMiddlewareImpl) block(c *gin.Context, path string, requestBody []byte, errorMsg, message string) {
	if isJSONRPCPath(path) {
		rpcMessage := message
		if message == guardrails.MsgBlocked || message == msgResponseBlocked {
			rpcMessage = errorMsg
		}
		writeJSONRPCBlocked(c, requestBody, rpcMessage)
		return
	}
	c.JSON(http.StatusForbidden, gin.H{
		"error":   errorMsg,
		"message": message,
	})
}

// isJSONRPCPath reports whether path is one of the gateway's JSON-RPC server
// endpoints, whose clients expect refusals in the JSON-RPC error envelope.
func isJSONRPCPath(path string) bool {
	return path == MCPPath || path == A2APath
}

// capturesResponse reports whether post_call runs on path: non-streaming chat
// completions and non-streaming A2A methods, whose whole response is one body.
func capturesResponse(path string, body []byte) bool {
	switch path {
	case ChatCompletionsPath:
		return !isStreamingRequest(body)
	case A2APath:
		return !a2a.IsStreamingMethod(jsonRPCMethod(body))
	}
	return false
}

// writeJSONRPCBlocked refuses a /mcp or /a2a request with a JSON-RPC error
// envelope echoing the request id, so the client can parse the refusal instead
// of getting a plain error object it does not understand.
func writeJSONRPCBlocked(c *gin.Context, body []byte, message string) {
	var req struct {
		ID json.RawMessage `json:"id"`
	}
	_ = json.Unmarshal(body, &req)

	var id types.MCPJSONRPCResponse_ID
	if len(req.ID) > 0 {
		_ = id.UnmarshalJSON(req.ID)
	}

	c.JSON(http.StatusForbidden, types.MCPJSONRPCResponse{
		Jsonrpc: types.MCPJSONRPCResponseJsonrpcN20,
		ID:      id,
		Error:   &types.MCPJSONRPCError{Code: JSONRPCGuardrailBlocked, Message: message},
	})
}

func jsonRPCMethod(body []byte) string {
	var req struct {
		Method string `json:"method"`
	}
	_ = json.Unmarshal(body, &req)
	return req.Method
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
	if path == ChatCompletionsPath || path == ResponsesPath {
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
