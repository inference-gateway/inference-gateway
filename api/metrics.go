package api

import (
	"compress/gzip"
	"io"
	"net/http"

	gin "github.com/gin-gonic/gin"
	colmetricspb "go.opentelemetry.io/proto/otlp/collector/metrics/v1"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

// maxMetricsBodyBytes caps the decoded OTLP push payload size.
const maxMetricsBodyBytes = 4 << 20

const (
	contentTypeProtobuf = "application/x-protobuf"
	contentTypeJSON     = "application/json"
)

// MetricsIngestionHandler is the OTLP/HTTP metrics receiver (POST /v1/metrics).
// It lets clients that bypass the gateway's inference path (e.g. subscription
// clients driving Claude Code directly) push their usage metrics.
func (router *RouterImpl) MetricsIngestionHandler(c *gin.Context) {
	if !router.cfg.Telemetry.Enable || !router.cfg.Telemetry.MetricsPushEnable {
		c.JSON(http.StatusForbidden, ErrorResponse{Error: "Metrics push is not enabled"})
		return
	}

	contentType := c.ContentType()
	if contentType != contentTypeProtobuf && contentType != contentTypeJSON {
		c.JSON(http.StatusUnsupportedMediaType, ErrorResponse{Error: "Content-Type must be application/x-protobuf or application/json"})
		return
	}

	var reader io.Reader = c.Request.Body
	if c.GetHeader("Content-Encoding") == "gzip" {
		gz, err := gzip.NewReader(reader)
		if err != nil {
			c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Invalid gzip payload"})
			return
		}
		defer gz.Close()
		reader = gz
	}

	body, err := io.ReadAll(io.LimitReader(reader, maxMetricsBodyBytes+1))
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Failed to read request body"})
		return
	}
	if len(body) > maxMetricsBodyBytes {
		c.JSON(http.StatusRequestEntityTooLarge, ErrorResponse{Error: "Payload exceeds 4 MiB limit"})
		return
	}

	req := &colmetricspb.ExportMetricsServiceRequest{}
	if contentType == contentTypeProtobuf {
		err = proto.Unmarshal(body, req)
	} else {
		err = protojson.Unmarshal(body, req)
	}
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Failed to decode OTLP payload"})
		return
	}

	result := router.telemetry.IngestMetrics(c.Request.Context(), req)

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
			c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "Failed to encode response"})
			return
		}
		c.Data(http.StatusOK, contentTypeProtobuf, payload)
		return
	}

	payload, err := protojson.Marshal(resp)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "Failed to encode response"})
		return
	}
	c.Data(http.StatusOK, contentTypeJSON, payload)
}
