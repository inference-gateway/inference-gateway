package middlewares

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	config "github.com/inference-gateway/inference-gateway/config"
)

const (
	ChatCompletionsPath      = "/v1/chat/completions"
	ResponsesPath            = "/v1/responses"
	MetricsIngestPath        = "/v1/metrics"
	HealthPath               = "/health"
	MCPPath                  = "/mcp"
	A2APath                  = "/a2a"
	A2AAgentsPath            = A2APath + "/agents"
	A2AAgentCardPath         = "/.well-known/agent-card.json"
	ProtectedResourcePath    = "/.well-known/oauth-protected-resource"
	MCPProtectedResourcePath = ProtectedResourcePath + MCPPath
	A2AProtectedResourcePath = ProtectedResourcePath + A2APath
)

// contentTypeJSONWithCharset is the Content-Type gin rendered JSON with, kept
// for byte-identical responses on every error path.
const contentTypeJSONWithCharset = "application/json; charset=utf-8"

// WriteJSON marshals v into the response body with the given status. The
// Content-Type is only set when the response does not already carry one, and
// the body has no trailing newline, exactly like gin's JSON rendering.
func WriteJSON(w http.ResponseWriter, status int, v any) {
	if len(w.Header()["Content-Type"]) == 0 {
		w.Header().Set("Content-Type", contentTypeJSONWithCharset)
	}
	data, err := json.Marshal(v)
	if err != nil {
		http.Error(w, "failed to marshal JSON response", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(status)
	_, _ = w.Write(data)
}

// StreamResponse repeatedly invokes step until it returns false, flushing the
// response after each round so streamed lines reach the client immediately,
// like gin's Context.Stream did.
func StreamResponse(w http.ResponseWriter, step func(io.Writer) bool) {
	for {
		keepOpen := step(w)
		_ = http.NewResponseController(w).Flush()
		if !keepOpen {
			return
		}
	}
}

// ForwardedProtoHeader carries the scheme a terminating proxy received on,
// which is the part of the public URL the gateway cannot otherwise see.
const ForwardedProtoHeader = "X-Forwarded-Proto"

// MCPExposed reports whether the gateway serves POST /mcp as an MCP server.
func MCPExposed(mcp *config.MCPConfig) bool {
	return mcp != nil && mcp.Enabled && mcp.Expose
}

// A2AEnabled reports whether the gateway serves POST /a2a as an A2A server.
func A2AEnabled(a2a *config.A2AConfig) bool {
	return a2a != nil && a2a.Enabled
}

// MCPResourceURL is the canonical public URL of POST /mcp, published as the
// resource of the RFC 9728 metadata document. MCP_RESOURCE_URL wins; without
// it the URL is derived from the request, which is only right when nothing
// between the client and the gateway rewrites the scheme or the host.
func MCPResourceURL(mcp *config.MCPConfig, r *http.Request) string {
	if mcp != nil && mcp.ResourceUrl != "" {
		return mcp.ResourceUrl
	}
	return requestResourceURL(r, MCPPath)
}

// A2AResourceURL is the canonical public URL of POST /a2a, published on the
// gateway agent card and as the resource of its RFC 9728 metadata document.
// A2A_RESOURCE_URL wins over the request-derived URL.
func A2AResourceURL(a2a *config.A2AConfig, r *http.Request) string {
	if a2a != nil && a2a.ResourceUrl != "" {
		return a2a.ResourceUrl
	}
	return requestResourceURL(r, A2APath)
}

func requestResourceURL(r *http.Request, path string) string {
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	if forwarded, _, _ := strings.Cut(r.Header.Get(ForwardedProtoHeader), ","); forwarded != "" {
		scheme = strings.TrimSpace(forwarded)
	}
	return scheme + "://" + r.Host + path
}

// ProtectedResourceMetadataURL maps a resource URL to the URL of the RFC 9728
// document describing it, by inserting the well-known prefix before its path.
func ProtectedResourceMetadataURL(resource string) string {
	u, err := url.Parse(resource)
	if err != nil {
		return resource
	}
	u.Path = ProtectedResourcePath + strings.TrimSuffix(u.Path, "/")
	return u.String()
}

// JSONRPCGuardrailBlocked is the server-defined JSON-RPC error code (the
// -32000..-32099 range) for a guardrails block on /mcp, so a client can tell a
// policy refusal from an upstream failure (-32603).
const JSONRPCGuardrailBlocked = -32001

// SetSSEHeaders sets the response headers required for server-sent event streaming
func SetSSEHeaders(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Transfer-Encoding", "chunked")
	w.Header().Set("X-Accel-Buffering", "no")
}

// ResetWriteDeadline extends the response write deadline by d so streaming
// responses are not cut off by the server's global write timeout
func ResetWriteDeadline(w http.ResponseWriter, d time.Duration) {
	var deadline time.Time
	if d > 0 {
		deadline = time.Now().Add(d)
	}
	_ = http.NewResponseController(w).SetWriteDeadline(deadline)
}

// DeadlineResetWriter resets the write deadline before every write so that
// proxied streaming responses are not cut off by the server's write timeout.
// Wrap the writer handed to httputil.ReverseProxy, which offers no per-write hook.
type DeadlineResetWriter struct {
	http.ResponseWriter
	Timeout time.Duration
}

func (w *DeadlineResetWriter) Write(b []byte) (int, error) {
	ResetWriteDeadline(w.ResponseWriter, w.Timeout)
	return w.ResponseWriter.Write(b)
}

func (w *DeadlineResetWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}

// customResponseWriter captures the downstream response so the wrapping
// middleware can inspect, redact or refuse it before anything reaches the client.
type customResponseWriter struct {
	http.ResponseWriter
	body       *bytes.Buffer
	statusCode int
}

// WriteHeader captures the status code but doesn't write it to the client.
func (w *customResponseWriter) WriteHeader(code int) {
	w.statusCode = code
}

// Write captures the response body but doesn't write it to the client.
func (w *customResponseWriter) Write(b []byte) (int, error) {
	return w.body.Write(b)
}

// replay sends the captured status and body through the underlying writer.
func (w *customResponseWriter) replayTo(inner http.ResponseWriter) {
	inner.WriteHeader(w.statusCode)
	_, _ = inner.Write(w.body.Bytes())
}

func (w *customResponseWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}
