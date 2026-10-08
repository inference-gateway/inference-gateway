package api

import (
	"context"
	"encoding/json"
	"io"
	"math"
	"net/http"
	"strconv"
	"time"

	types "github.com/inference-gateway/adk/types"

	middlewares "github.com/inference-gateway/inference-gateway/api/middlewares"
	config "github.com/inference-gateway/inference-gateway/config"
	a2a "github.com/inference-gateway/inference-gateway/internal/a2a"
	logger "github.com/inference-gateway/inference-gateway/internal/platform/logger"
	otel "github.com/inference-gateway/inference-gateway/internal/platform/otel"
)

const (
	jsonRPCVersion = "2.0"

	a2aStatusOK = "ok"

	errMsgA2AParse      = "parse error"
	errMsgA2AInvalidReq = "invalid request: jsonrpc must be \"2.0\" and method is required"
)

// A2AHandler serves the gateway's A2A server surface: its agent card, the
// JSON-RPC relay and the registry listing. It holds no task state; every call
// is forwarded to the one agent the request names.
type A2AHandler struct {
	cfg       config.Config
	logger    logger.Logger
	registry  *a2a.Registry
	telemetry otel.OpenTelemetry
	version   string
}

// NewA2AHandler wires the A2A routes over a registry; telemetry may be nil.
func NewA2AHandler(cfg config.Config, log logger.Logger, registry *a2a.Registry, telemetry otel.OpenTelemetry, version string) *A2AHandler {
	return &A2AHandler{cfg: cfg, logger: log, registry: registry, telemetry: telemetry, version: version}
}

// AgentCard serves GET /.well-known/agent-card.json: the merged card of every
// registered agent, pointing at this gateway's /a2a endpoint.
func (h *A2AHandler) AgentCard(w http.ResponseWriter, r *http.Request) {
	middlewares.WriteJSON(w, http.StatusOK, h.registry.Card(middlewares.A2AResourceURL(h.cfg.A2A, r), h.version))
}

// Agents serves GET /a2a/agents, the diagnostic view of the registry.
func (h *A2AHandler) Agents(w http.ResponseWriter, r *http.Request) {
	middlewares.WriteJSON(w, http.StatusOK, map[string]any{"agents": h.registry.Agents()})
}

// ProtectedResourceMetadata serves the RFC 9728 document for POST /a2a under
// the same conditions as the /mcp one: auth enabled and the surface exposed.
func (h *A2AHandler) ProtectedResourceMetadata(w http.ResponseWriter, r *http.Request) {
	auth := h.cfg.Auth
	if auth == nil || !auth.Enabled {
		middlewares.WriteJSON(w, http.StatusNotFound, ErrorResponse{Error: "Requested route is not found"})
		return
	}
	middlewares.WriteJSON(w, http.StatusOK, map[string]any{
		"resource":                 middlewares.A2AResourceURL(h.cfg.A2A, r),
		"authorization_servers":    []string{auth.OidcIssuer},
		"bearer_methods_supported": []string{bearerMethodHeader},
	})
}

// JSONRPC serves POST /a2a. Every method in the ADK enum is relayed to exactly
// one agent; the two streaming methods pipe the upstream SSE stream through.
func (h *A2AHandler) JSONRPC(w http.ResponseWriter, r *http.Request) {
	started := time.Now()
	req, rpcErr := h.readRequest(r)
	if rpcErr != nil {
		h.respondError(w, nil, rpcErr)
		return
	}
	if req.ID == nil {
		w.WriteHeader(http.StatusAccepted)
		return
	}

	params := types.Struct{}
	if req.Params != nil {
		params = *req.Params
	}

	if a2a.IsStreamingMethod(string(req.Method)) {
		h.relayStream(w, r, req, params, started)
		return
	}

	alias, result, rpcErr := h.registry.Call(r.Context(), req.Method, params)
	h.record(r, alias, req.Method, rpcErr, started)
	if rpcErr != nil {
		h.logger.Error("a2a call failed", nil, "method", string(req.Method), "agent", alias, "code", rpcErr.Code, "message", rpcErr.Message)
		h.respondError(w, req, rpcErr)
		return
	}
	middlewares.WriteJSON(w, http.StatusOK, types.JSONRPCSuccessResponse{ID: *req.ID, JSONRPC: jsonRPCVersion, Result: result})
}

// relayStream writes each upstream event as an SSE data line under the client's
// request id, until the upstream closes, the client goes away or the stream
// stays idle for A2A_STREAM_IDLE_TIMEOUT.
func (h *A2AHandler) relayStream(w http.ResponseWriter, r *http.Request, req *types.JSONRPCRequest, params types.Struct, started time.Time) {
	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	alias, events, rpcErr := h.registry.Stream(ctx, req.Method, params)
	h.record(r, alias, req.Method, rpcErr, started)
	if rpcErr != nil {
		h.logger.Error("a2a stream failed", nil, "method", string(req.Method), "agent", alias, "code", rpcErr.Code, "message", rpcErr.Message)
		h.respondError(w, req, rpcErr)
		return
	}

	middlewares.SetSSEHeaders(w)
	w.WriteHeader(http.StatusOK)
	_ = http.NewResponseController(w).Flush()

	idle := h.cfg.A2A.StreamIdleTimeout
	cutoff := time.NewTimer(idleCutoff(idle))
	defer cutoff.Stop()
	for {
		select {
		case result, ok := <-events:
			if !ok {
				return
			}
			if !h.writeEvent(w, types.JSONRPCSuccessResponse{ID: *req.ID, JSONRPC: jsonRPCVersion, Result: result}, idle) {
				return
			}
			cutoff.Reset(idleCutoff(idle))
		case <-cutoff.C:
			h.logger.Warn("a2a stream closed after idle timeout", "method", string(req.Method), "agent", alias, "idle", idle.String())
			return
		case <-ctx.Done():
			return
		}
	}
}

// idleCutoff maps a non-positive A2A_STREAM_IDLE_TIMEOUT to a timer that never
// fires within the lifetime of a stream.
func idleCutoff(idle time.Duration) time.Duration {
	if idle <= 0 {
		return math.MaxInt64
	}
	return idle
}

func (h *A2AHandler) writeEvent(w http.ResponseWriter, event types.JSONRPCSuccessResponse, idle time.Duration) bool {
	payload, err := json.Marshal(event)
	if err != nil {
		h.logger.Error("failed to encode a2a stream event", err)
		return false
	}
	middlewares.ResetWriteDeadline(w, idle)
	if _, err := w.Write([]byte("data: " + string(payload) + "\n\n")); err != nil {
		return false
	}
	_ = http.NewResponseController(w).Flush()
	return true
}

// readRequest reads and validates the JSON-RPC envelope; the method must be
// one of the eleven the ADK enum knows.
func (h *A2AHandler) readRequest(r *http.Request) (*types.JSONRPCRequest, *a2a.Error) {
	maxBodySize := h.cfg.Server.ResolveMaxRequestBodySize()
	body, err := io.ReadAll(io.LimitReader(r.Body, int64(maxBodySize)+1))
	if err != nil {
		h.logger.Error("failed to read a2a request body", err)
		return nil, &a2a.Error{Code: a2a.CodeParseError, Message: errMsgA2AParse}
	}
	if len(body) > maxBodySize {
		return nil, &a2a.Error{Code: a2a.CodeInvalidRequest, Message: "request body too large"}
	}

	var req types.JSONRPCRequest
	if err := json.Unmarshal(body, &req); err != nil {
		return nil, &a2a.Error{Code: a2a.CodeParseError, Message: errMsgA2AParse}
	}
	if req.JSONRPC != jsonRPCVersion || req.Method == "" {
		return &req, &a2a.Error{Code: a2a.CodeInvalidRequest, Message: errMsgA2AInvalidReq}
	}
	if !req.Method.Valid() {
		return &req, &a2a.Error{Code: a2a.CodeMethodNotFound, Message: "method not found: " + string(req.Method)}
	}
	return &req, nil
}

// respondError writes a JSON-RPC error envelope with HTTP 200, the status A2A
// clients decode the body on; the id echoes the request when it had one.
func (h *A2AHandler) respondError(w http.ResponseWriter, req *types.JSONRPCRequest, rpcErr *a2a.Error) {
	var id types.Value
	if req != nil && req.ID != nil {
		id = *req.ID
	}
	middlewares.WriteJSON(w, http.StatusOK, types.JSONRPCErrorResponse{ID: id, JSONRPC: jsonRPCVersion, Error: *rpcErr})
}

func (h *A2AHandler) record(r *http.Request, alias string, method types.A2AMethod, rpcErr *a2a.Error, started time.Time) {
	if h.telemetry == nil {
		return
	}
	status := a2aStatusOK
	if rpcErr != nil {
		status = strconv.Itoa(rpcErr.Code)
	}
	h.telemetry.RecordA2ARequest(r.Context(), alias, string(method), status, time.Since(started).Seconds())
}
