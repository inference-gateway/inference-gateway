package middlewares

import (
	"bytes"
	"net/http"
	"time"

	gin "github.com/gin-gonic/gin"
)

const (
	// ChatCompletionsPath is the endpoint path for chat completions
	ChatCompletionsPath = "/v1/chat/completions"
)

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

type ErrorResponse struct {
	Error string `json:"error"`
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
