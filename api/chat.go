package api

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"

	middlewares "github.com/inference-gateway/inference-gateway/api/middlewares"
	core "github.com/inference-gateway/inference-gateway/providers/core"
	modelsdev "github.com/inference-gateway/inference-gateway/providers/modelsdev"
	types "github.com/inference-gateway/inference-gateway/providers/types"
)

// ChatCompletionsHandler implements an OpenAI-compatible API endpoint
// that generates text completions in the standard OpenAI format.
//
// Regular response format:
//
//	{
//	  "choices": [
//	    {
//	      "finish_reason": "stop",
//	      "message": {
//	        "content": "Hello, how can I help you today?",
//	        "role": "assistant"
//	      }
//	    }
//	  ],
//	  "created": 1742165657,
//	  "id": "chatcmpl-118",
//	  "model": "deepseek-r1:1.5b",
//	  "object": "chat.completion",
//	  "usage": {
//	    "completion_tokens": 139,
//	    "prompt_tokens": 10,
//	    "total_tokens": 149
//	  }
//	}
//
// Streaming response format:
//
//	{
//	  "choices": [
//	    {
//	      "index": 0,
//	      "finish_reason": "stop",
//	      "delta": {
//	        "content": "Hello",
//	        "role": "assistant"
//	      }
//	    }
//	  ],
//	  "created": 1742165657,
//	  "id": "chatcmpl-118",
//	  "model": "deepseek-r1:1.5b",
//	  "object": "chat.completion.chunk",
//	  "usage": {
//	    "completion_tokens": 139,
//	    "prompt_tokens": 10,
//	    "total_tokens": 149
//	  }
//	}
//
// It returns token completions as chat in the standard OpenAI format, allowing applications
// built for OpenAI's API to work seamlessly with the Inference Gateway's multi-provider
// architecture.
func (router *RouterImpl) ChatCompletionsHandler(w http.ResponseWriter, r *http.Request) {
	var req types.CreateChatCompletionRequest

	if mcpRequest, ok := middlewares.MCPRequestFromContext(r.Context()); ok {
		req = *mcpRequest
	} else {
		maxBodySize := router.cfg.Server.ResolveMaxRequestBodySize()
		r.Body = http.MaxBytesReader(w, r.Body, int64(maxBodySize))
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			var maxBytesErr *http.MaxBytesError
			if errors.As(err, &maxBytesErr) {
				router.logger.Error("request body too large", err)
				middlewares.WriteJSON(w, http.StatusRequestEntityTooLarge, ErrorResponse{Error: "Request body too large"})
				return
			}
			router.logger.Error("failed to decode request", err)
			middlewares.WriteJSON(w, http.StatusBadRequest, ErrorResponse{Error: "Failed to decode request"})
			return
		}
	}

	model := req.Model
	originalModel := req.Model
	providerID := types.Provider(r.URL.Query().Get("provider"))

	var routedProvider, routedModel string
	if router.selector != nil && providerID == "" {
		if dep, ok := router.selector.Select(model); ok {
			providerID = types.Provider(dep.Provider)
			model = dep.Model
			routedProvider, routedModel = dep.Provider, dep.Model
			router.logger.Debug("routed logical model", "alias", originalModel, "provider", dep.Provider, "model", dep.Model)
		}
	}

	if providerID == "" {
		var msg string
		providerID, model, msg = router.resolveProvider(r, model, "openai/gpt-4")
		if msg != "" {
			middlewares.WriteJSON(w, http.StatusBadRequest, ErrorResponse{Error: msg})
			return
		}
	}
	req.Model = model

	if reason := router.modelDenied(originalModel); reason != "" {
		middlewares.WriteJSON(w, http.StatusForbidden, ErrorResponse{Error: reason})
		return
	}

	provider, msg, err := router.buildProvider(providerID)
	if err != nil {
		middlewares.WriteJSON(w, http.StatusBadRequest, ErrorResponse{Error: msg})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), router.cfg.Server.ReadTimeout)
	defer cancel()

	if router.cfg.VisionEnabled {
		hasImageContent := false
		imageCount := 0
		for _, message := range req.Messages {
			if message.HasImageContent() {
				hasImageContent = true
				imageCount++
			}
		}

		if hasImageContent {
			if !modelsdev.AcceptsImages(providerID, req.Model) {
				router.logger.Info("filtering images from non-vision model request",
					"provider", providerID,
					"model", req.Model,
					"messagesWithImages", imageCount)

				for i := range req.Messages {
					if req.Messages[i].HasImageContent() {
						if err := req.Messages[i].StripImageContent(); err != nil {
							router.logger.Error("failed to strip image content from message", err)
							middlewares.WriteJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to process message content"})
							return
						}
					}
				}

				router.logger.Debug("images stripped from request, continuing with text-only content")
			}
		}
	}

	router.logger.Debug("server read timeout", "timeout", router.cfg.Server.ReadTimeout)

	if routedProvider != "" {
		w.Header().Set("X-Selected-Provider", routedProvider)
		w.Header().Set("X-Selected-Model", routedModel)
	}

	if req.Stream != nil && *req.Stream {
		middlewares.SetSSEHeaders(w)

		streamCtx := r.Context()
		streamCh, err := provider.StreamChatCompletions(streamCtx, req)
		if err != nil {
			router.logger.Error("failed to start streaming", err, "provider", providerID)

			writeProviderError(w, err)
			return
		}

		middlewares.StreamResponse(w, func(_ io.Writer) bool {
			select {
			case line, ok := <-streamCh:
				if !ok {
					router.logger.Debug("stream closed", "provider", providerID)
					return false
				}

				middlewares.ResetWriteDeadline(w, router.cfg.Server.WriteTimeout)

				router.logger.Debug("stream chunk",
					"provider", providerID,
					"bytes", len(line),
					"line", string(line))

				if _, err := w.Write(line); err != nil {
					router.logger.Error("failed to write chunk", err)
					return false
				}

				return true
			case <-streamCtx.Done():
				return false
			}
		})
		return
	}

	w.Header().Set("Content-Type", contentTypeJSON)
	response, err := provider.ChatCompletions(ctx, req)
	if err != nil {
		if err == context.DeadlineExceeded || ctx.Err() == context.DeadlineExceeded {
			router.logger.Error("request timed out", err, "provider", providerID)
			middlewares.WriteJSON(w, http.StatusGatewayTimeout, ErrorResponse{Error: "Request timed out"})
			return
		}
		router.logger.Error("failed to generate tokens", err, "provider", providerID)

		writeProviderError(w, err)
		return
	}

	middlewares.WriteJSON(w, http.StatusOK, response)
}

// writeProviderError relays an upstream provider failure to the client with
// the upstream status and its Retry-After header, so clients can tell a quota
// wall from a transient error. Errors without an upstream status become 400.
func writeProviderError(w http.ResponseWriter, err error) {
	status := http.StatusBadRequest
	var httpErr *core.HTTPError
	if errors.As(err, &httpErr) {
		status = httpErr.StatusCode
		if httpErr.RetryAfter != "" {
			w.Header().Set("Retry-After", httpErr.RetryAfter)
		}
	}
	middlewares.WriteJSON(w, status, ErrorResponse{Error: err.Error()})
}
