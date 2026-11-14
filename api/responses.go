package api

import (
	"bytes"
	"encoding/json"
	"net/http"

	semconv "go.opentelemetry.io/otel/semconv/v1.41.0"
	trace "go.opentelemetry.io/otel/trace"

	middlewares "github.com/inference-gateway/inference-gateway/api/middlewares"
	constants "github.com/inference-gateway/inference-gateway/providers/constants"
)

// ResponsesHandler implements an OpenAI-compatible POST /v1/responses
// endpoint: https://platform.openai.com/docs/api-reference/responses
//
// The request body is forwarded to the upstream provider byte-for-byte (only
// the `model` field is rewritten when the provider prefix is stripped), so
// all Responses API fields pass through untouched, and the upstream response
// - including ResponseStreamEvent frames when streaming - is relayed verbatim.
//
// Only providers that natively implement the Responses API are supported
// (currently OpenAI); other providers receive a 400, mirroring the schema's
// ResponsesNotSupported response.
func (router *RouterImpl) ResponsesHandler(w http.ResponseWriter, r *http.Request) {
	body, tooLarge, err := router.readBoundedBody(r)
	if err != nil {
		router.logger.Error("failed to read request body", err)
		middlewares.WriteJSON(w, http.StatusBadRequest, ErrorResponse{Error: "Failed to read request"})
		return
	}
	if tooLarge {
		middlewares.WriteJSON(w, http.StatusRequestEntityTooLarge, ErrorResponse{Error: "Request body too large"})
		return
	}

	var req struct {
		Model  string `json:"model"`
		Stream *bool  `json:"stream"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		router.logger.Error("failed to decode request", err)
		middlewares.WriteJSON(w, http.StatusBadRequest, ErrorResponse{Error: "Failed to decode request"})
		return
	}

	originalModel := req.Model
	model := req.Model
	providerID, model, msg := router.resolveProvider(r, model, "openai/gpt-4o")
	if msg != "" {
		middlewares.WriteJSON(w, http.StatusBadRequest, ErrorResponse{Error: msg})
		return
	}

	span := trace.SpanFromContext(r.Context())
	span.SetAttributes(
		semconv.GenAIProviderNameKey.String(string(providerID)),
		semconv.GenAIRequestModel(originalModel),
	)

	if reason := router.modelDenied(originalModel); reason != "" {
		middlewares.WriteJSON(w, http.StatusForbidden, ErrorResponse{Error: reason})
		return
	}

	if providerID != constants.OpenaiID {
		router.logger.Error("responses api not supported by provider", nil, "provider", providerID)
		middlewares.WriteJSON(w, http.StatusBadRequest, ErrorResponse{Error: "The Responses API is not supported by this provider yet. Use /v1/chat/completions instead."})
		return
	}

	provider, msg, err := router.buildProvider(providerID)
	if err != nil {
		middlewares.WriteJSON(w, http.StatusBadRequest, ErrorResponse{Error: msg})
		return
	}

	endpoint := provider.GetEndpoints().Responses
	if endpoint == nil || *endpoint == "" {
		router.logger.Error("responses api not supported by provider", nil, "provider", providerID)
		middlewares.WriteJSON(w, http.StatusBadRequest, ErrorResponse{Error: "The Responses API is not supported by this provider yet. Use /v1/chat/completions instead."})
		return
	}

	if model != originalModel {
		if body, err = router.rewriteModelOrRespond(w, body, model); err != nil {
			return
		}
	}

	isStreaming := req.Stream != nil && *req.Stream
	if f := router.forwardUpstream(w, r, provider, upstreamRequest{
		endpointPath: *endpoint,
		body:         bytes.NewReader(body),
		contentType:  contentTypeJSON,
		accept:       acceptHeaderFor(isStreaming),
		streaming:    isStreaming,
	}); f != nil {
		middlewares.WriteJSON(w, f.status, ErrorResponse{Error: f.message})
	}
}
