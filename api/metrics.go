package api

import (
	"compress/gzip"
	"io"
	"net/http"
	"strings"

	colmetricspb "go.opentelemetry.io/proto/otlp/collector/metrics/v1"
	protojson "google.golang.org/protobuf/encoding/protojson"
	proto "google.golang.org/protobuf/proto"

	middlewares "github.com/inference-gateway/inference-gateway/api/middlewares"
)

// maxMetricsBodyBytes caps the decoded OTLP push payload size.
const maxMetricsBodyBytes = 4 << 20

const (
	contentTypeProtobuf    = "application/x-protobuf"
	contentTypeJSON        = "application/json"
	contentTypeEventStream = "text/event-stream"
)

// contentTypeOf returns the bare media type of a Content-Type header value,
// matching gin's Context.ContentType: everything before the first space or
// semicolon.
func contentTypeOf(header http.Header) string {
	value := header.Get("Content-Type")
	if idx := strings.IndexAny(value, " ;"); idx != -1 {
		return value[:idx]
	}
	return value
}

// MetricsIngestionHandler is the OTLP/HTTP metrics receiver (POST /v1/metrics).
// It lets clients that bypass the gateway's inference path (e.g. subscription
// clients driving Claude Code directly) push their usage metrics.
func (router *RouterImpl) MetricsIngestionHandler(w http.ResponseWriter, r *http.Request) {
	if !router.cfg.Telemetry.Enabled || !router.cfg.Telemetry.MetricsPushEnabled {
		middlewares.WriteJSON(w, http.StatusForbidden, ErrorResponse{Error: "Metrics push is not enabled"})
		return
	}

	contentType := contentTypeOf(r.Header)
	if contentType != contentTypeProtobuf && contentType != contentTypeJSON {
		middlewares.WriteJSON(w, http.StatusUnsupportedMediaType, ErrorResponse{Error: "Content-Type must be application/x-protobuf or application/json"})
		return
	}

	var reader io.Reader = r.Body
	if r.Header.Get("Content-Encoding") == "gzip" {
		gz, err := gzip.NewReader(reader)
		if err != nil {
			middlewares.WriteJSON(w, http.StatusBadRequest, ErrorResponse{Error: "Invalid gzip payload"})
			return
		}
		defer gz.Close()
		reader = gz
	}

	body, err := io.ReadAll(io.LimitReader(reader, maxMetricsBodyBytes+1))
	if err != nil {
		middlewares.WriteJSON(w, http.StatusBadRequest, ErrorResponse{Error: "Failed to read request body"})
		return
	}
	if len(body) > maxMetricsBodyBytes {
		middlewares.WriteJSON(w, http.StatusRequestEntityTooLarge, ErrorResponse{Error: "Payload exceeds 4 MiB limit"})
		return
	}

	req := &colmetricspb.ExportMetricsServiceRequest{}
	if contentType == contentTypeProtobuf {
		err = proto.Unmarshal(body, req)
	} else {
		err = protojson.Unmarshal(body, req)
	}
	if err != nil {
		middlewares.WriteJSON(w, http.StatusBadRequest, ErrorResponse{Error: "Failed to decode OTLP payload"})
		return
	}

	result := router.telemetry.IngestMetrics(r.Context(), req)

	resp := &colmetricspb.ExportMetricsServiceResponse{}
	if result.RejectedDataPoints > 0 {
		resp.PartialSuccess = &colmetricspb.ExportMetricsPartialSuccess{
			RejectedDataPoints: result.RejectedDataPoints,
			ErrorMessage:       result.ErrorMessage,
		}
	}

	router.logger.Debug("otlp metrics push ingested",
		"accepted_data_points", result.AcceptedDataPoints,
		"rejected_data_points", result.RejectedDataPoints)

	if contentType == contentTypeProtobuf {
		payload, err := proto.Marshal(resp)
		if err != nil {
			middlewares.WriteJSON(w, http.StatusInternalServerError, ErrorResponse{Error: "Failed to encode response"})
			return
		}
		writeData(w, http.StatusOK, contentTypeProtobuf, payload)
		return
	}

	payload, err := protojson.Marshal(resp)
	if err != nil {
		middlewares.WriteJSON(w, http.StatusInternalServerError, ErrorResponse{Error: "Failed to encode response"})
		return
	}
	writeData(w, http.StatusOK, contentTypeJSON, payload)
}
