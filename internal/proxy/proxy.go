package proxy

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"io"
	"net/http"

	"github.com/inference-gateway/inference-gateway/logger"
)

// ResponseModifier defines interface for modifying proxy responses
type ResponseModifier interface {
	Modify(resp *http.Response) error
}

// DevResponseModifier implements response modification for development
type DevResponseModifier struct {
	logger logger.Logger
}

// NewDevResponseModifier creates a new DevResponseModifier
func NewDevResponseModifier(l logger.Logger) ResponseModifier {
	return &DevResponseModifier{
		logger: l,
	}
}

func (m *DevResponseModifier) Modify(resp *http.Response) error {
	if resp == nil || resp.Body == nil {
		return nil
	}

	// Read original body for logging only
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		m.logger.Error("failed to read response body", err)
		return err
	}

	// Store original body to restore
	originalBody := bytes.NewBuffer(body)

	// Try to log prettified content if possible
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

	// Try to format JSON for logging only
	var prettyJSON bytes.Buffer
	if err := json.Indent(&prettyJSON, logBody, "", "  "); err == nil {
		logBody = prettyJSON.Bytes()
	}

	// Log the response without modifying it
	m.logger.Debug("proxy response",
		"status", resp.Status,
		"headers", resp.Header,
		"body", string(logBody),
	)

	// Restore original body exactly as received
	resp.Body = io.NopCloser(originalBody)
	resp.ContentLength = int64(originalBody.Len())

	return nil
}
