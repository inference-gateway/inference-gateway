package proxy

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/inference-gateway/inference-gateway/config"
	"github.com/inference-gateway/inference-gateway/logger"
	"github.com/inference-gateway/inference-gateway/providers"
)

// RequestModifier defines interface for modifying proxy requests
type RequestModifier interface {
	Modify(req *http.Request) error
}

// ResponseModifier defines interface for modifying proxy responses
type ResponseModifier interface {
	Modify(resp *http.Response) error
}

// DevRequestModifier implements request modification for development
type DevRequestModifier struct {
	logger logger.Logger
	cfg    *config.Config
}

// DevResponseModifier implements response modification for development
type DevResponseModifier struct {
	logger logger.Logger
}

// NewDevRequestModifier creates a new DevRequestModifier
func NewDevRequestModifier(l logger.Logger, cfg *config.Config) RequestModifier {
	return &DevRequestModifier{
		logger: l,
		cfg:    cfg,
	}
}

// NewDevResponseModifier creates a new DevResponseModifier
func NewDevResponseModifier(l logger.Logger) ResponseModifier {
	return &DevResponseModifier{
		logger: l,
	}
}

func (m *DevRequestModifier) Modify(req *http.Request) error {
	if req == nil || req.Body == nil {
		return nil
	}

	body, err := io.ReadAll(req.Body)
	if err != nil {
		m.logger.Error("failed to read request body", err)
		return err
	}

	bodyBuffer := bytes.NewBuffer(body)

	// Try to parse as JSON and apply smart truncation for chat completion requests
	bodyPreview := m.createSmartBodyPreview(body)

	m.logger.Debug("proxy request",
		"method", req.Method,
		"url", req.URL.String(),
		"content_length", len(body),
		"body_preview", bodyPreview,
	)

	req.Body = io.NopCloser(bodyBuffer)
	req.ContentLength = int64(bodyBuffer.Len())

	return nil
}

// truncateWords truncates text to the specified number of words
func (m *DevRequestModifier) truncateWords(text string, maxWords int) string {
	if maxWords <= 0 {
		return ""
	}

	words := strings.Fields(text)
	if len(words) <= maxWords {
		return text
	}

	return strings.Join(words[:maxWords], " ") + "..."
}

// createSmartBodyPreview creates an intelligent preview of the request body
func (m *DevRequestModifier) createSmartBodyPreview(body []byte) string {
	// Try to parse as chat completion request
	var chatReq providers.CreateChatCompletionRequest
	if err := json.Unmarshal(body, &chatReq); err != nil {
		// Not a chat completion request, fall back to simple truncation
		bodyPreview := string(body)
		if len(bodyPreview) > 1024 {
			bodyPreview = bodyPreview[:1024] + "... (truncated)"
		}
		return bodyPreview
	}

	// Apply smart truncation to chat completion request
	return m.truncateChatCompletionRequest(chatReq)
}

// truncateChatCompletionRequest applies smart truncation to chat completion requests
func (m *DevRequestModifier) truncateChatCompletionRequest(req providers.CreateChatCompletionRequest) string {
	maxWords := m.cfg.DebugContentTruncateWords
	maxMessages := m.cfg.DebugMaxMessages

	// Create a copy to modify for display
	displayReq := req

	// Limit number of messages
	if len(displayReq.Messages) > maxMessages {
		displayReq.Messages = displayReq.Messages[:maxMessages]
	}

	// Truncate content in each message
	for i := range displayReq.Messages {
		if displayReq.Messages[i].Content != "" {
			displayReq.Messages[i].Content = m.truncateWords(displayReq.Messages[i].Content, maxWords)
		}
	}

	// Convert back to JSON for display
	truncatedBytes, err := json.Marshal(displayReq)
	if err != nil {
		// Fall back to original string representation if marshaling fails
		bodyPreview := fmt.Sprintf("%+v", req)
		if len(bodyPreview) > 1024 {
			bodyPreview = bodyPreview[:1024] + "... (truncated)"
		}
		return bodyPreview
	}

	preview := string(truncatedBytes)
	if len(displayReq.Messages) < len(req.Messages) {
		preview = strings.TrimSuffix(preview, "}") +
			fmt.Sprintf(",\"_truncated\":\"showing %d of %d messages\"}", len(displayReq.Messages), len(req.Messages))
	}

	return preview
}

func (m *DevResponseModifier) Modify(resp *http.Response) error {
	if resp == nil || resp.Body == nil {
		return nil
	}

	contentType := resp.Header.Get("Content-Type")
	transferEncoding := resp.Header.Get("Transfer-Encoding")

	isStreaming := contentType == "text/event-stream" ||
		(transferEncoding == "chunked" && contentType != "application/json") ||
		(resp.ContentLength == -1 && contentType != "application/json")

	if isStreaming {
		m.logger.Debug("proxy streaming response",
			"status", resp.Status,
			"content_type", contentType,
			"transfer_encoding", transferEncoding,
			"streaming", true,
		)
		return nil
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		m.logger.Error("failed to read response body", err)
		return err
	}

	originalBody := bytes.NewBuffer(body)

	if len(body) <= 4096 {
		var logBody []byte
		if resp.Header.Get("Content-Encoding") == "gzip" {
			reader, err := gzip.NewReader(bytes.NewReader(body))
			if err == nil {
				defer reader.Close()
				if uncompressed, err := io.ReadAll(reader); err == nil {
					logBody = uncompressed
				}
			}
		} else {
			logBody = body
		}

		var prettyJSON bytes.Buffer
		if len(logBody) <= 2048 && json.Valid(logBody) {
			if err := json.Indent(&prettyJSON, logBody, "", "  "); err == nil {
				logBody = prettyJSON.Bytes()
			}
		}

		m.logger.Debug("proxy response",
			"status", resp.Status,
			"content_length", len(body),
			"content_type", resp.Header.Get("Content-Type"),
			"body", string(logBody),
		)
	} else {
		m.logger.Debug("proxy response",
			"status", resp.Status,
			"content_length", len(body),
			"content_type", resp.Header.Get("Content-Type"),
			"body", "... (response too large for logging)",
		)
	}

	resp.Body = io.NopCloser(originalBody)
	resp.ContentLength = int64(originalBody.Len())

	return nil
}
