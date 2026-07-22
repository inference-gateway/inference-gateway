package tests

import (
	"bytes"
	"compress/gzip"
	"net/http"
	"net/http/httptest"
	"testing"

	gin "github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	colmetricspb "go.opentelemetry.io/proto/otlp/collector/metrics/v1"
	"go.uber.org/mock/gomock"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	"github.com/inference-gateway/inference-gateway/api"
	"github.com/inference-gateway/inference-gateway/config"
	"github.com/inference-gateway/inference-gateway/logger"
	"github.com/inference-gateway/inference-gateway/otel"
	mocks "github.com/inference-gateway/inference-gateway/tests/mocks"
)

func newMetricsTestRouter(t *testing.T, telemetryEnabled, pushEnabled bool, telemetry otel.OpenTelemetry) *gin.Engine {
	t.Helper()

	log, err := logger.NewLogger("test")
	require.NoError(t, err)

	cfg := config.Config{
		Telemetry: &config.TelemetryConfig{
			Enable:            telemetryEnabled,
			MetricsPushEnable: pushEnabled,
		},
	}

	router := api.NewRouter(cfg, log, nil, nil, nil, telemetry, nil)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/v1/metrics", router.MetricsIngestionHandler)
	return r
}

func TestMetricsIngestionHandler(t *testing.T) {
	t.Run("returns 403 when push is disabled", func(t *testing.T) {
		for _, flags := range [][2]bool{{false, false}, {true, false}, {false, true}} {
			r := newMetricsTestRouter(t, flags[0], flags[1], nil)

			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "/v1/metrics", bytes.NewBufferString("{}"))
			req.Header.Set("Content-Type", "application/json")
			r.ServeHTTP(w, req)

			assert.Equal(t, http.StatusForbidden, w.Code)
		}
	})

	t.Run("returns 415 for unsupported content type", func(t *testing.T) {
		r := newMetricsTestRouter(t, true, true, nil)

		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/v1/metrics", bytes.NewBufferString("hello"))
		req.Header.Set("Content-Type", "text/plain")
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnsupportedMediaType, w.Code)
	})

	t.Run("returns 400 for malformed payloads", func(t *testing.T) {
		r := newMetricsTestRouter(t, true, true, nil)

		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/v1/metrics", bytes.NewBufferString("not-json"))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("accepts a JSON OTLP payload", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		mockTelemetry := mocks.NewMockOpenTelemetry(ctrl)
		mockTelemetry.EXPECT().IngestMetrics(gomock.Any(), gomock.Any()).Return(otel.IngestResult{AcceptedDataPoints: 2})

		r := newMetricsTestRouter(t, true, true, mockTelemetry)

		body, err := protojson.Marshal(&colmetricspb.ExportMetricsServiceRequest{})
		require.NoError(t, err)

		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/v1/metrics", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var resp colmetricspb.ExportMetricsServiceResponse
		require.NoError(t, protojson.Unmarshal(w.Body.Bytes(), &resp))
		assert.Nil(t, resp.GetPartialSuccess())
	})

	t.Run("accepts a protobuf OTLP payload and reports partial success", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		mockTelemetry := mocks.NewMockOpenTelemetry(ctrl)
		mockTelemetry.EXPECT().IngestMetrics(gomock.Any(), gomock.Any()).Return(otel.IngestResult{
			AcceptedDataPoints: 1,
			RejectedDataPoints: 2,
			ErrorMessage:       `unsupported metric "bogus"`,
		})

		r := newMetricsTestRouter(t, true, true, mockTelemetry)

		body, err := proto.Marshal(&colmetricspb.ExportMetricsServiceRequest{})
		require.NoError(t, err)

		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/v1/metrics", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/x-protobuf")
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var resp colmetricspb.ExportMetricsServiceResponse
		require.NoError(t, proto.Unmarshal(w.Body.Bytes(), &resp))
		require.NotNil(t, resp.GetPartialSuccess())
		assert.Equal(t, int64(2), resp.GetPartialSuccess().GetRejectedDataPoints())
		assert.Contains(t, resp.GetPartialSuccess().GetErrorMessage(), "bogus")
	})

	t.Run("accepts gzip-compressed payloads", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		mockTelemetry := mocks.NewMockOpenTelemetry(ctrl)
		mockTelemetry.EXPECT().IngestMetrics(gomock.Any(), gomock.Any()).Return(otel.IngestResult{})

		r := newMetricsTestRouter(t, true, true, mockTelemetry)

		body, err := protojson.Marshal(&colmetricspb.ExportMetricsServiceRequest{})
		require.NoError(t, err)
		var compressed bytes.Buffer
		gz := gzip.NewWriter(&compressed)
		_, err = gz.Write(body)
		require.NoError(t, err)
		require.NoError(t, gz.Close())

		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/v1/metrics", &compressed)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Content-Encoding", "gzip")
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("returns 413 for oversized payloads", func(t *testing.T) {
		r := newMetricsTestRouter(t, true, true, nil)

		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/v1/metrics", bytes.NewBuffer(make([]byte, 5<<20)))
		req.Header.Set("Content-Type", "application/x-protobuf")
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusRequestEntityTooLarge, w.Code)
	})
}
