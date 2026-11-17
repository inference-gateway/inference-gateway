package api

import (
	"bytes"
	"encoding/json"
	"net/http"

	semconv "go.opentelemetry.io/otel/semconv/v1.41.0"
	trace "go.opentelemetry.io/otel/trace"

	middlewares "github.com/inference-gateway/inference-gateway/api/middlewares"
	constants "github.com/inference-gateway/inference-gateway/providers/constants"
	types "github.com/inference-gateway/inference-gateway/providers/types"
)

// messagesError writes a gateway-generated error in the Anthropic error
// envelope ({"type": "error", "error": {"type": ..., "message": ...}}), which
// is what native Messages API clients expect to parse.
func messagesError(w http.ResponseWriter, status int, errType, message string) {
	resp := types.MessagesError{Type: types.MessagesErrorTypeError}
	resp.Error.Type = errType
	resp.Error.Message = message
	middlewares.WriteJSON(w, status, resp)
}

// MessagesHandler serves the Anthropic-compatible POST /v1/messages. The body is
// forwarded byte-for-byte with only the model prefix stripped, so cache_control
// and future fields pass through, and the response (cache usage, SSE envelope) is
// relayed verbatim. Providers without a native Messages API receive a 400 in the
// Anthropic error envelope (MessagesNotSupported).
func (router *RouterImpl) MessagesHandler(w http.ResponseWriter, r *http.Request) {
	body, tooLarge, err := router.readBoundedBody(r)
	if err != nil {
		router.logger.Error("failed to read request body", err)
		messagesError(w, http.StatusBadRequest, "invalid_request_error", "Failed to read request")
		return
	}
	if tooLarge {
		messagesError(w, http.StatusRequestEntityTooLarge, "invalid_request_error", "Request body too large")
		return
	}

	var req struct {
		Model  string `json:"model"`
		Stream *bool  `json:"stream"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		router.logger.Error("failed to decode request", err)
		messagesError(w, http.StatusBadRequest, "invalid_request_error", "Failed to decode request")
		return
	}

	originalModel := req.Model
	model := req.Model
	providerID, model, msg := router.resolveProvider(r, model, "anthropic/claude-sonnet-4-5")
	if msg != "" {
		messagesError(w, http.StatusBadRequest, "invalid_request_error", msg)
		return
	}

	span := trace.SpanFromContext(r.Context())
	span.SetAttributes(
		semconv.GenAIProviderNameKey.String(string(providerID)),
		semconv.GenAIRequestModel(originalModel),
	)

	if reason := router.modelDenied(originalModel); reason != "" {
		messagesError(w, http.StatusForbidden, "invalid_request_error", reason)
		return
	}

	if providerID != constants.AnthropicID {
		router.logger.Error("messages api not supported by provider", nil, "provider", providerID)
		messagesError(w, http.StatusBadRequest, "not_supported_error", "The Messages API is not supported by this provider yet.")
		return
	}

	provider, msg, err := router.buildProvider(providerID)
	if err != nil {
		messagesError(w, http.StatusBadRequest, "invalid_request_error", msg)
		return
	}

	if model != originalModel {
		if body, err = rewriteModelField(body, model); err != nil {
			status, msg := router.rewriteFailure(err)
			errType := "invalid_request_error"
			if status == http.StatusInternalServerError {
				errType = "api_error"
			}
			messagesError(w, status, errType, msg)
			return
		}
	}

	isStreaming := req.Stream != nil && *req.Stream
	if f := router.forwardUpstream(w, r, provider, upstreamRequest{
		endpointPath: "/messages",
		body:         bytes.NewReader(body),
		contentType:  contentTypeJSON,
		accept:       acceptHeaderFor(isStreaming),
		streaming:    isStreaming,
	}); f != nil {
		messagesError(w, f.status, "api_error", f.message)
	}
}
