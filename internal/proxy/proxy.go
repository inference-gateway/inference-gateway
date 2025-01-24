package proxy

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"io"
	"net/http"
	"strings"

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

func NewDevResponseModifier(l logger.Logger) ResponseModifier {
	return &DevResponseModifier{logger: l}
}

func (m *DevResponseModifier) Modify(resp *http.Response) error {
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		m.logger.Error("Failed to read response from proxy", err)
		return err
	}

	// Always restore the body
	defer func() {
		resp.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
	}()

	if !strings.Contains(resp.Header.Get("Content-Type"), "application/json") {
		return nil
	}

	contentBody := m.handleGzippedContent(resp, bodyBytes)
	m.logJSONResponse(resp, contentBody)
	return nil
}

func (m *DevResponseModifier) handleGzippedContent(resp *http.Response, bodyBytes []byte) []byte {
	if resp.Header.Get("Content-Encoding") != "gzip" || len(bodyBytes) == 0 {
		return bodyBytes
	}

	reader, err := gzip.NewReader(bytes.NewReader(bodyBytes))
	if err != nil {
		m.logger.Error("Invalid gzip content", err)
		return bodyBytes
	}
	defer reader.Close()

	decompressed, err := io.ReadAll(reader)
	if err != nil {
		m.logger.Error("Failed to read gzipped content", err)
		return bodyBytes
	}

	return decompressed
}

func (m *DevResponseModifier) logJSONResponse(resp *http.Response, body []byte) {
	var data interface{}
	if err := json.Unmarshal(body, &data); err != nil {
		m.logger.Error("Failed to unmarshal JSON response", err)
		return
	}

	m.logger.Debug("Proxy response", "status", resp.StatusCode, "body", data)
}
