package mcp

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	m "github.com/metoro-io/mcp-golang"
	transport "github.com/metoro-io/mcp-golang/transport/http"
	otelapi "go.opentelemetry.io/otel"
	propagation "go.opentelemetry.io/otel/propagation"
)

// TransportMode represents the type of transport being used
type TransportMode string

const (
	TransportModeStreamableHTTP TransportMode = "streamable-http"
	TransportModeSSE            TransportMode = "sse"
)

// customRoundTripper wraps http.RoundTripper to add streaming headers and handle SSE responses
type customRoundTripper struct {
	base        http.RoundTripper
	fallbackURL string

	mu        sync.Mutex
	sessionID string
	mode      TransportMode
}

// parseSSEResponse extracts JSON data from SSE formatted response
func parseSSEResponse(responseBody string) (string, error) {
	lines := strings.Split(responseBody, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "data: ") {
			jsonData := strings.TrimPrefix(line, "data: ")
			if jsonData != "" && jsonData != "[DONE]" {
				return jsonData, nil
			}
		}
	}

	return "", fmt.Errorf("no valid JSON data found in SSE response")
}

func (c *customRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	req = req.Clone(req.Context())

	req.Header.Del("Authorization")
	req.Header.Del("Cookie")
	req.Header.Del("X-API-Key")

	otelapi.GetTextMapPropagator().Inject(req.Context(), propagation.HeaderCarrier(req.Header))

	c.mu.Lock()
	mode := c.mode
	sessionID := c.sessionID
	c.mu.Unlock()

	switch mode {
	case TransportModeStreamableHTTP:
		req.Header.Set("Accept", "application/json, text/event-stream")
		req.Header.Set("Cache-Control", "no-cache")
		req.Header.Set("Connection", "keep-alive")
	case TransportModeSSE:
		req.Header.Set("Accept", "text/event-stream")
		req.Header.Set("Cache-Control", "no-cache")
		req.Header.Set("Connection", "keep-alive")
	default:
		req.Header.Set("Accept", "application/json, text/event-stream")
		req.Header.Set("Cache-Control", "no-cache")
		req.Header.Set("Connection", "keep-alive")
	}

	if sessionID != "" {
		req.Header.Set("mcp-session-id", sessionID)
	}

	var bodyBytes []byte
	if req.Method == "POST" && req.Body != nil {
		var err error
		bodyBytes, err = io.ReadAll(req.Body)
		if err != nil {
			return nil, err
		}
		req.Body.Close()

		var jsonBody map[string]any
		if err := json.Unmarshal(bodyBytes, &jsonBody); err == nil {
			if params, ok := jsonBody["params"].(map[string]any); ok {
				if cursor, exists := params["cursor"]; exists && cursor == nil {
					delete(params, "cursor")
					if modifiedBody, err := json.Marshal(jsonBody); err == nil {
						bodyBytes = modifiedBody
					}
				}
			}
		}

		req.Body = io.NopCloser(strings.NewReader(string(bodyBytes)))
		req.ContentLength = int64(len(bodyBytes))
	}

	resp, err := c.base.RoundTrip(req)
	if err != nil {
		return resp, err
	}

	if respSessionID := resp.Header.Get("mcp-session-id"); respSessionID != "" {
		c.mu.Lock()
		c.sessionID = respSessionID
		c.mu.Unlock()
	}

	if resp.StatusCode >= 400 && resp.StatusCode < 500 && mode == TransportModeStreamableHTTP {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
		resp.Body.Close()
		return c.attemptSSEFallback(req, bodyBytes)
	}

	contentType := resp.Header.Get("Content-Type")
	if strings.Contains(contentType, "text/event-stream") ||
		strings.Contains(contentType, "text/plain") {

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return resp, err
		}
		resp.Body.Close()

		bodyStr := string(body)
		if strings.Contains(bodyStr, "data: ") {
			jsonData, err := parseSSEResponse(bodyStr)
			if err != nil {
				return resp, fmt.Errorf("failed to parse SSE response: %v", err)
			}

			resp.Body = io.NopCloser(strings.NewReader(jsonData))
			resp.Header.Set("Content-Type", "application/json")
			resp.ContentLength = int64(len(jsonData))
		} else {
			resp.Body = io.NopCloser(strings.NewReader(bodyStr))
		}
	}

	return resp, nil
}

// attemptSSEFallback tries to fallback to SSE transport when Streamable HTTP fails
func (c *customRoundTripper) attemptSSEFallback(req *http.Request, bodyBytes []byte) (*http.Response, error) {
	c.mu.Lock()
	c.mode = TransportModeSSE
	c.mu.Unlock()

	if c.fallbackURL != "" {
		originalURL := req.URL
		fallbackURL, err := url.Parse(c.fallbackURL)
		if err == nil {
			req.URL = fallbackURL
			req.Header.Set("Accept", "text/event-stream")

			if bodyBytes != nil {
				req.Body = io.NopCloser(bytes.NewReader(bodyBytes))
				req.ContentLength = int64(len(bodyBytes))
			}

			resp, err := c.base.RoundTrip(req)
			if err != nil {
				req.URL = originalURL
				return nil, fmt.Errorf("both streamable HTTP and SSE transports failed: %v", err)
			}
			return resp, nil
		}
	}

	return nil, fmt.Errorf("streamable HTTP transport failed and no SSE fallback URL configured")
}

// NewClientWithTransport creates a new MCP client with specific transport mode
func (mc *MCPClient) NewClientWithTransport(serverURL string, mode TransportMode) *m.Client {
	baseTransport := &http.Transport{
		DialContext: (&net.Dialer{
			Timeout:   mc.Config.MCP.DialTimeout,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		TLSHandshakeTimeout:   mc.Config.MCP.TlsHandshakeTimeout,
		ResponseHeaderTimeout: mc.Config.MCP.ResponseHeaderTimeout,
		ExpectContinueTimeout: mc.Config.MCP.ExpectContinueTimeout,
	}

	fallbackURL := mc.BuildSSEFallbackURL(serverURL)

	httpClient := &http.Client{
		Timeout: mc.Config.MCP.ClientTimeout,
		Transport: &customRoundTripper{
			base:        baseTransport,
			mode:        mode,
			fallbackURL: fallbackURL,
		},
	}

	var acceptHeader string
	switch mode {
	case TransportModeStreamableHTTP:
		acceptHeader = "application/json, text/event-stream"
	case TransportModeSSE:
		acceptHeader = "text/event-stream"
	default:
		acceptHeader = "application/json, text/event-stream"
	}

	httpTransport := transport.NewHTTPClientTransport(serverURL).WithHeader(
		"Accept", acceptHeader).WithClient(httpClient)

	return m.NewClient(httpTransport)
}

// BuildSSEFallbackURL creates an SSE fallback URL from the main server URL
func (mc *MCPClient) BuildSSEFallbackURL(serverURL string) string {
	if strings.HasSuffix(serverURL, "/mcp") {
		return strings.TrimSuffix(serverURL, "/mcp") + "/sse"
	}
	if strings.HasSuffix(serverURL, "/") {
		return serverURL + "sse"
	}
	return serverURL + "/sse"
}
