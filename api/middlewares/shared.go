package middlewares

import (
	"bytes"
	"net/http"
	"net/url"
	"strings"
	"time"

	gin "github.com/gin-gonic/gin"

	config "github.com/inference-gateway/inference-gateway/config"
)

const (
	ChatCompletionsPath      = "/v1/chat/completions"
	ResponsesPath            = "/v1/responses"
	MetricsIngestPath        = "/v1/metrics"
	HealthPath               = "/health"
	MCPPath                  = "/mcp"
	ProtectedResourcePath    = "/.well-known/oauth-protected-resource"
	MCPProtectedResourcePath = ProtectedResourcePath + MCPPath
)

// ForwardedProtoHeader carries the scheme a terminating proxy received on,
// which is the part of the public URL the gateway cannot otherwise see.
const ForwardedProtoHeader = "X-Forwarded-Proto"

// MCPExposed reports whether the gateway serves POST /mcp as an MCP server.
func MCPExposed(mcp *config.MCPConfig) bool {
	return mcp != nil && mcp.Enabled && mcp.Expose
}

// MCPResourceURL is the canonical public URL of POST /mcp, published as the
// resource of the RFC 9728 metadata document. MCP_RESOURCE_URL wins; without
// it the URL is derived from the request, which is only right when nothing
// between the client and the gateway rewrites the scheme or the host.
func MCPResourceURL(mcp *config.MCPConfig, r *http.Request) string {
	if mcp != nil && mcp.ResourceUrl != "" {
		return mcp.ResourceUrl
	}
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	if forwarded, _, _ := strings.Cut(r.Header.Get(ForwardedProtoHeader), ","); forwarded != "" {
		scheme = strings.TrimSpace(forwarded)
	}
	return scheme + "://" + r.Host + MCPPath
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
func SetSSEHeaders(c *gin.Context) {
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("Transfer-Encoding", "chunked")
	c.Header("X-Accel-Buffering", "no")
}

// ResetWriteDeadline extends the response write deadline by d so streaming
// responses are not cut off by the server's global write timeout
func ResetWriteDeadline(c *gin.Context, d time.Duration) {
	resetWriteDeadline(c.Writer, d)
}

func resetWriteDeadline(w http.ResponseWriter, d time.Duration) {
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
	gin.ResponseWriter
	Timeout time.Duration
}

func (w *DeadlineResetWriter) Write(b []byte) (int, error) {
	resetWriteDeadline(w.ResponseWriter, w.Timeout)
	return w.ResponseWriter.Write(b)
}

func (w *DeadlineResetWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}

// customResponseWriter captures the response body but doesn't write it
// to the client until we're ready, allowing us to intercept tool calls
type customResponseWriter struct {
	gin.ResponseWriter
	body          *bytes.Buffer
	statusCode    int
	writeToClient bool
}

// captureResponse swaps a buffering writer into c.Writer so the downstream
// handler's response can be inspected before anything reaches the client.
func captureResponse(c *gin.Context) *customResponseWriter {
	w := &customResponseWriter{
		ResponseWriter: c.Writer,
		body:           &bytes.Buffer{},
		statusCode:     http.StatusOK,
		writeToClient:  false,
	}
	c.Writer = w
	return w
}

// replay restores the original writer and sends the captured response verbatim.
func (w *customResponseWriter) replay(c *gin.Context) {
	c.Writer = w.ResponseWriter
	c.Data(w.statusCode, w.Header().Get("Content-Type"), w.body.Bytes())
}

// WriteHeader captures the status code but doesn't write it to the client
// unless writeToClient is true
func (w *customResponseWriter) WriteHeader(code int) {
	w.statusCode = code
	if w.writeToClient {
		w.ResponseWriter.WriteHeader(code)
	}
}

// Write captures the response body but doesn't write it to the client
// unless writeToClient is true
func (w *customResponseWriter) Write(b []byte) (int, error) {
	w.body.Write(b)
	if w.writeToClient {
		return w.ResponseWriter.Write(b)
	}
	return len(b), nil
}

func (w *customResponseWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}
