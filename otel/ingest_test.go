package otel

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	colmetricspb "go.opentelemetry.io/proto/otlp/collector/metrics/v1"
	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
	metricspb "go.opentelemetry.io/proto/otlp/metrics/v1"
	resourcepb "go.opentelemetry.io/proto/otlp/resource/v1"
)

func newTestTelemetry(t *testing.T) (*OpenTelemetryImpl, *sdkmetric.ManualReader) {
	t.Helper()
	reader := sdkmetric.NewManualReader()
	provider := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(reader),
		sdkmetric.WithView(metricViews()...),
	)
	o := &OpenTelemetryImpl{meterProvider: provider}
	require.NoError(t, o.initInstruments(provider))
	return o, reader
}

func collect(t *testing.T, reader *sdkmetric.ManualReader) metricdata.ResourceMetrics {
	t.Helper()
	var rm metricdata.ResourceMetrics
	require.NoError(t, reader.Collect(context.Background(), &rm))
	return rm
}

func findMetric(rm metricdata.ResourceMetrics, name string) (metricdata.Metrics, bool) {
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			if m.Name == name {
				return m, true
			}
		}
	}
	return metricdata.Metrics{}, false
}

func strAttr(key, value string) *commonpb.KeyValue {
	return &commonpb.KeyValue{
		Key:   key,
		Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: value}},
	}
}

func requestWith(serviceName string, metrics ...*metricspb.Metric) *colmetricspb.ExportMetricsServiceRequest {
	rm := &metricspb.ResourceMetrics{
		ScopeMetrics: []*metricspb.ScopeMetrics{{Metrics: metrics}},
	}
	if serviceName != "" {
		rm.Resource = &resourcepb.Resource{Attributes: []*commonpb.KeyValue{strAttr("service.name", serviceName)}}
	}
	return &colmetricspb.ExportMetricsServiceRequest{ResourceMetrics: []*metricspb.ResourceMetrics{rm}}
}

func deltaSum(name string, value int64, monotonic bool, attrs ...*commonpb.KeyValue) *metricspb.Metric {
	return &metricspb.Metric{
		Name: name,
		Data: &metricspb.Metric_Sum{Sum: &metricspb.Sum{
			AggregationTemporality: metricspb.AggregationTemporality_AGGREGATION_TEMPORALITY_DELTA,
			IsMonotonic:            monotonic,
			DataPoints: []*metricspb.NumberDataPoint{{
				Attributes: attrs,
				Value:      &metricspb.NumberDataPoint_AsInt{AsInt: value},
			}},
		}},
	}
}

func TestIngestMetrics(t *testing.T) {
	ctx := context.Background()

	t.Run("delta sum token usage is recorded as one observation", func(t *testing.T) {
		o, reader := newTestTelemetry(t)

		result := o.IngestMetrics(ctx, requestWith("infer-cli",
			deltaSum("gen_ai.client.token.usage", 1500, false,
				strAttr("gen_ai.provider.name", "anthropic"),
				strAttr("gen_ai.token.type", "input"),
				strAttr("source", "claude-code-subscription"),
			),
		))

		assert.Equal(t, int64(1), result.AcceptedDataPoints)
		assert.Equal(t, int64(0), result.RejectedDataPoints)

		m, ok := findMetric(collect(t, reader), "gen_ai.client.token.usage")
		require.True(t, ok)
		hist := m.Data.(metricdata.Histogram[int64])
		require.Len(t, hist.DataPoints, 1)
		dp := hist.DataPoints[0]
		assert.Equal(t, uint64(1), dp.Count)
		assert.Equal(t, int64(1500), dp.Sum)

		source, ok := dp.Attributes.Value("source")
		require.True(t, ok)
		assert.Equal(t, "claude-code-subscription", source.AsString())
		provider, ok := dp.Attributes.Value("gen_ai.provider.name")
		require.True(t, ok)
		assert.Equal(t, "anthropic", provider.AsString())
	})

	t.Run("tool calls delta monotonic sum is added to the counter", func(t *testing.T) {
		o, reader := newTestTelemetry(t)

		result := o.IngestMetrics(ctx, requestWith("infer-cli",
			deltaSum("inference_gateway.tool_calls", 3, true,
				strAttr("gen_ai.tool.name", "mcp_read_file")),
		))

		assert.Equal(t, int64(1), result.AcceptedDataPoints)

		m, ok := findMetric(collect(t, reader), "inference_gateway.tool_calls")
		require.True(t, ok)
		sum := m.Data.(metricdata.Sum[int64])
		require.Len(t, sum.DataPoints, 1)
		assert.Equal(t, int64(3), sum.DataPoints[0].Value)
	})

	t.Run("histogram replay preserves count", func(t *testing.T) {
		o, reader := newTestTelemetry(t)

		result := o.IngestMetrics(ctx, requestWith("infer-cli", &metricspb.Metric{
			Name: "gen_ai.execute_tool.duration",
			Data: &metricspb.Metric_Histogram{Histogram: &metricspb.Histogram{
				AggregationTemporality: metricspb.AggregationTemporality_AGGREGATION_TEMPORALITY_DELTA,
				DataPoints: []*metricspb.HistogramDataPoint{{
					Count:          5,
					Sum:            floatPtr(2.5),
					ExplicitBounds: []float64{0.1, 1.0},
					BucketCounts:   []uint64{2, 2, 1},
				}},
			}},
		}))

		assert.Equal(t, int64(1), result.AcceptedDataPoints)

		m, ok := findMetric(collect(t, reader), "gen_ai.execute_tool.duration")
		require.True(t, ok)
		hist := m.Data.(metricdata.Histogram[float64])
		require.Len(t, hist.DataPoints, 1)
		assert.Equal(t, uint64(5), hist.DataPoints[0].Count)
	})

	t.Run("cumulative temporality is rejected", func(t *testing.T) {
		o, _ := newTestTelemetry(t)

		result := o.IngestMetrics(ctx, requestWith("infer-cli", &metricspb.Metric{
			Name: "gen_ai.client.token.usage",
			Data: &metricspb.Metric_Sum{Sum: &metricspb.Sum{
				AggregationTemporality: metricspb.AggregationTemporality_AGGREGATION_TEMPORALITY_CUMULATIVE,
				DataPoints:             []*metricspb.NumberDataPoint{{Value: &metricspb.NumberDataPoint_AsInt{AsInt: 10}}},
			}},
		}))

		assert.Equal(t, int64(0), result.AcceptedDataPoints)
		assert.Equal(t, int64(1), result.RejectedDataPoints)
		assert.Contains(t, result.ErrorMessage, "delta temporality")
	})

	t.Run("unknown metric names are rejected", func(t *testing.T) {
		o, _ := newTestTelemetry(t)

		result := o.IngestMetrics(ctx, requestWith("infer-cli",
			deltaSum("my.custom.metric", 1, true)))

		assert.Equal(t, int64(0), result.AcceptedDataPoints)
		assert.Equal(t, int64(1), result.RejectedDataPoints)
		assert.Contains(t, result.ErrorMessage, `unsupported metric "my.custom.metric"`)
	})

	t.Run("source falls back to service.name and gateway is rewritten", func(t *testing.T) {
		o, reader := newTestTelemetry(t)

		o.IngestMetrics(ctx, requestWith("infer-cli",
			deltaSum("gen_ai.client.token.usage", 10, false, strAttr("gen_ai.token.type", "input")),
			deltaSum("gen_ai.client.token.usage", 20, false,
				strAttr("gen_ai.token.type", "output"),
				strAttr("source", SourceGateway)),
		))

		m, ok := findMetric(collect(t, reader), "gen_ai.client.token.usage")
		require.True(t, ok)
		hist := m.Data.(metricdata.Histogram[int64])
		for _, dp := range hist.DataPoints {
			source, ok := dp.Attributes.Value("source")
			require.True(t, ok)
			assert.Equal(t, "infer-cli", source.AsString(), "pushed source must never be %q", SourceGateway)
		}
	})

	t.Run("disallowed attributes are dropped", func(t *testing.T) {
		o, reader := newTestTelemetry(t)

		o.IngestMetrics(ctx, requestWith("infer-cli",
			deltaSum("gen_ai.client.token.usage", 10, false,
				strAttr("gen_ai.token.type", "input"),
				strAttr("user.id", "someone"),
				strAttr("session.id", "abc123"))))

		m, ok := findMetric(collect(t, reader), "gen_ai.client.token.usage")
		require.True(t, ok)
		hist := m.Data.(metricdata.Histogram[int64])
		require.Len(t, hist.DataPoints, 1)
		_, hasUser := hist.DataPoints[0].Attributes.Value("user.id")
		assert.False(t, hasUser)
		_, hasSession := hist.DataPoints[0].Attributes.Value("session.id")
		assert.False(t, hasSession)
	})

	t.Run("replay is capped per data point", func(t *testing.T) {
		o, reader := newTestTelemetry(t)

		result := o.IngestMetrics(ctx, requestWith("infer-cli", &metricspb.Metric{
			Name: "gen_ai.server.request.duration",
			Data: &metricspb.Metric_Histogram{Histogram: &metricspb.Histogram{
				AggregationTemporality: metricspb.AggregationTemporality_AGGREGATION_TEMPORALITY_DELTA,
				DataPoints: []*metricspb.HistogramDataPoint{{
					Count:          1000000,
					ExplicitBounds: []float64{1},
					BucketCounts:   []uint64{1000000, 0},
				}},
			}},
		}))

		assert.Equal(t, int64(1), result.AcceptedDataPoints)

		m, ok := findMetric(collect(t, reader), "gen_ai.server.request.duration")
		require.True(t, ok)
		hist := m.Data.(metricdata.Histogram[float64])
		require.Len(t, hist.DataPoints, 1)
		assert.Equal(t, uint64(maxReplayObservations), hist.DataPoints[0].Count)
	})

	t.Run("error message aggregates distinct reasons", func(t *testing.T) {
		o, _ := newTestTelemetry(t)

		result := o.IngestMetrics(ctx, requestWith("infer-cli",
			deltaSum("bogus.one", 1, true),
			deltaSum("bogus.two", 1, true)))

		assert.Equal(t, int64(2), result.RejectedDataPoints)
		assert.Equal(t, 2, len(strings.Split(result.ErrorMessage, "; ")))
	})
}

func floatPtr(f float64) *float64 { return &f }
