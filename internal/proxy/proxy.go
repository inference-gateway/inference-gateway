package proxy

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"io"
	"net/http"

	"github.com/inference-gateway/inference-gateway/logger"
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
}

// DevResponseModifier implements response modification for development
type DevResponseModifier struct {
	logger logger.Logger
}

// NewDevRequestModifier creates a new DevRequestModifier
func NewDevRequestModifier(l logger.Logger) RequestModifier {
	return &DevRequestModifier{
		logger: l,
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

	bodyPreview := string(body)
	if len(bodyPreview) > 1024 {
		bodyPreview = bodyPreview[:1024] + "... (truncated)"
	}

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
