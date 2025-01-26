package proxy

import (
	"bytes"
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

// Modify opens the response and logs it in development mode
func (m *DevResponseModifier) Modify(resp *http.Response) error {
	if resp == nil || resp.Body == nil {
		return nil
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		m.logger.Error("failed to read response body", err)
		return err
	}
	resp.Body.Close()

	m.logger.Debug("proxy response",
		"status", resp.Status,
		"headers", resp.Header,
		"body", string(body),
	)

	resp.Body = io.NopCloser(bytes.NewBuffer(body))

	resp.ContentLength = int64(len(body))

	return nil
}
