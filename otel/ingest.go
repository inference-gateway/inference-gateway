package otel

import (
	"cmp"
	"context"
	"fmt"
	"strings"

	attribute "go.opentelemetry.io/otel/attribute"
	metric "go.opentelemetry.io/otel/metric"
	colmetricspb "go.opentelemetry.io/proto/otlp/collector/metrics/v1"
	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
	metricspb "go.opentelemetry.io/proto/otlp/metrics/v1"
)

// maxReplayObservations bounds the number of synthetic observations replayed
// from a single pushed histogram data point.
const maxReplayObservations = 10000

// Only these data-point attributes are copied onto internal instruments, to
// bound label cardinality from untrusted pushers.
var allowedAttributes = map[string]bool{
	"gen_ai.provider.name":  true,
	"gen_ai.system":         true, // legacy alias for gen_ai.provider.name
	"gen_ai.request.model":  true,
	"gen_ai.response.model": true,
	"gen_ai.operation.name": true,
	"gen_ai.token.type":     true,
	"gen_ai.tool.name":      true,
	"gen_ai.tool.type":      true,
	"error.type":            true,
}

// IngestMetrics maps a pushed OTLP payload onto the gateway's instruments.
// Only allowlisted gen_ai.* metrics with delta temporality are accepted;
// everything else is counted as rejected and reported via the result.
func (o *OpenTelemetryImpl) IngestMetrics(ctx context.Context, req *colmetricspb.ExportMetricsServiceRequest) IngestResult {
	var result IngestResult
	rejections := map[string]bool{}

	reject := func(points int, reason string) {
		result.RejectedDataPoints += int64(points)
		rejections[reason] = true
	}

	for _, rm := range req.GetResourceMetrics() {
		serviceName := resourceServiceName(rm)

		for _, sm := range rm.GetScopeMetrics() {
			for _, m := range sm.GetMetrics() {
				switch m.GetName() {
				case "gen_ai.client.token.usage":
					o.ingestTokenUsage(ctx, m, serviceName, reject, &result)
				case "gen_ai.client.operation.duration":
					o.ingestDurationHistogram(ctx, m, o.clientOperationDuration, serviceName, reject, &result)
				case "gen_ai.server.request.duration":
					o.ingestDurationHistogram(ctx, m, o.serverRequestDuration, serviceName, reject, &result)
				case "gen_ai.client.operation.time_to_first_chunk":
					o.ingestDurationHistogram(ctx, m, o.clientTimeToFirstChunk, serviceName, reject, &result)
				case "gen_ai.server.time_to_first_token":
					o.ingestDurationHistogram(ctx, m, o.serverTimeToFirstToken, serviceName, reject, &result)
				case "gen_ai.execute_tool.duration":
					o.ingestDurationHistogram(ctx, m, o.executeToolDuration, serviceName, reject, &result)
				case "inference_gateway.tool_calls":
					o.ingestToolCalls(ctx, m, serviceName, reject, &result)
				default:
					reject(countDataPoints(m), fmt.Sprintf("unsupported metric %q", m.GetName()))
				}
			}
		}
	}

	if len(rejections) > 0 {
		reasons := make([]string, 0, len(rejections))
		for r := range rejections {
			reasons = append(reasons, r)
		}
		result.ErrorMessage = strings.Join(reasons, "; ")
	}

	return result
}

// ingestTokenUsage accepts either a delta sum (recorded as one observation,
// matching semconv's one-observation-per-operation usage) or a delta histogram.
func (o *OpenTelemetryImpl) ingestTokenUsage(ctx context.Context, m *metricspb.Metric, serviceName string, reject func(int, string), result *IngestResult) {
	switch data := m.GetData().(type) {
	case *metricspb.Metric_Sum:
		if data.Sum.GetAggregationTemporality() != metricspb.AggregationTemporality_AGGREGATION_TEMPORALITY_DELTA {
			reject(len(data.Sum.GetDataPoints()), fmt.Sprintf("metric %q: only delta temporality is supported", m.GetName()))
			return
		}
		for _, dp := range data.Sum.GetDataPoints() {
			attrs := o.pushAttributes(dp.GetAttributes(), serviceName)
			o.tokenUsageHistogram.Record(ctx, numberValueInt(dp), metric.WithAttributes(attrs...))
			result.AcceptedDataPoints++
		}
	case *metricspb.Metric_Histogram:
		o.replayHistogram(ctx, m.GetName(), data.Histogram, serviceName, reject, result, func(value float64, opts metric.MeasurementOption) {
			o.tokenUsageHistogram.Record(ctx, int64(value), opts)
		})
	default:
		reject(countDataPoints(m), fmt.Sprintf("metric %q: unsupported data type", m.GetName()))
	}
}

func (o *OpenTelemetryImpl) ingestDurationHistogram(ctx context.Context, m *metricspb.Metric, target metric.Float64Histogram, serviceName string, reject func(int, string), result *IngestResult) {
	data, ok := m.GetData().(*metricspb.Metric_Histogram)
	if !ok {
		reject(countDataPoints(m), fmt.Sprintf("metric %q: only histogram data is supported", m.GetName()))
		return
	}
	o.replayHistogram(ctx, m.GetName(), data.Histogram, serviceName, reject, result, func(value float64, opts metric.MeasurementOption) {
		target.Record(ctx, value, opts)
	})
}

func (o *OpenTelemetryImpl) ingestToolCalls(ctx context.Context, m *metricspb.Metric, serviceName string, reject func(int, string), result *IngestResult) {
	data, ok := m.GetData().(*metricspb.Metric_Sum)
	if !ok {
		reject(countDataPoints(m), fmt.Sprintf("metric %q: only sum data is supported", m.GetName()))
		return
	}
	if data.Sum.GetAggregationTemporality() != metricspb.AggregationTemporality_AGGREGATION_TEMPORALITY_DELTA || !data.Sum.GetIsMonotonic() {
		reject(len(data.Sum.GetDataPoints()), fmt.Sprintf("metric %q: only delta monotonic sums are supported", m.GetName()))
		return
	}

	for _, dp := range data.Sum.GetDataPoints() {
		attrs := o.pushAttributes(dp.GetAttributes(), serviceName)
		o.toolCallCounter.Add(ctx, numberValueInt(dp), metric.WithAttributes(attrs...))
		result.AcceptedDataPoints++
	}
}

// replayHistogram approximates a pushed histogram by re-recording bucket
// midpoints (first bucket at its upper bound, overflow bucket at its lower
// bound). This preserves _count exactly and _sum approximately; percentile
// distortion is a documented v1 limitation.
func (o *OpenTelemetryImpl) replayHistogram(ctx context.Context, name string, h *metricspb.Histogram, serviceName string, reject func(int, string), result *IngestResult, record func(float64, metric.MeasurementOption)) {
	if h.GetAggregationTemporality() != metricspb.AggregationTemporality_AGGREGATION_TEMPORALITY_DELTA {
		reject(len(h.GetDataPoints()), fmt.Sprintf("metric %q: only delta temporality is supported", name))
		return
	}

	for _, dp := range h.GetDataPoints() {
		opts := metric.WithAttributes(o.pushAttributes(dp.GetAttributes(), serviceName)...)
		bounds := dp.GetExplicitBounds()
		counts := dp.GetBucketCounts()

		replayed := 0
		if len(bounds) > 0 && len(counts) == len(bounds)+1 {
			for i, count := range counts {
				value := bucketValue(bounds, i)
				for range count {
					if replayed >= maxReplayObservations {
						break
					}
					record(value, opts)
					replayed++
				}
			}
		} else if dp.GetCount() > 0 {
			mean := dp.GetSum() / float64(dp.GetCount())
			replays := min(int(dp.GetCount()), maxReplayObservations)
			for range replays {
				record(mean, opts)
			}
		}
		result.AcceptedDataPoints++
	}
}

func bucketValue(bounds []float64, bucket int) float64 {
	switch {
	case bucket == 0:
		return bounds[0]
	case bucket >= len(bounds):
		return bounds[len(bounds)-1]
	default:
		return (bounds[bucket-1] + bounds[bucket]) / 2
	}
}

// pushAttributes filters pushed attributes down to the allowlist and derives
// the source and team labels. Source: an explicit source attribute wins (unless
// it impersonates the gateway), then the resource's service.name, then
// "unknown". Team: an explicit team attribute is carried through, defaulting to
// TeamUnknown so the label stays present on every series.
func (o *OpenTelemetryImpl) pushAttributes(kvs []*commonpb.KeyValue, serviceName string) []attribute.KeyValue {
	source := ""
	team := ""
	attrs := make([]attribute.KeyValue, 0, len(kvs)+2)

	for _, kv := range kvs {
		value := kv.GetValue().GetStringValue()
		switch kv.GetKey() {
		case "source":
			source = value
			continue
		case "team":
			team = value
			continue
		}
		if allowedAttributes[kv.GetKey()] && value != "" {
			attrs = append(attrs, attribute.String(kv.GetKey(), value))
		}
	}

	if source == "" || source == SourceGateway {
		source = serviceName
	}
	if source == "" || source == SourceGateway {
		source = "unknown"
	}

	return append(attrs, sourceKey.String(source), teamKey.String(cmp.Or(team, TeamUnknown)))
}

func resourceServiceName(rm *metricspb.ResourceMetrics) string {
	for _, kv := range rm.GetResource().GetAttributes() {
		if kv.GetKey() == "service.name" {
			return kv.GetValue().GetStringValue()
		}
	}
	return ""
}

func numberValueInt(dp *metricspb.NumberDataPoint) int64 {
	if v, ok := dp.GetValue().(*metricspb.NumberDataPoint_AsDouble); ok {
		return int64(v.AsDouble)
	}
	return dp.GetAsInt()
}

func countDataPoints(m *metricspb.Metric) int {
	switch data := m.GetData().(type) {
	case *metricspb.Metric_Sum:
		return len(data.Sum.GetDataPoints())
	case *metricspb.Metric_Gauge:
		return len(data.Gauge.GetDataPoints())
	case *metricspb.Metric_Histogram:
		return len(data.Histogram.GetDataPoints())
	case *metricspb.Metric_ExponentialHistogram:
		return len(data.ExponentialHistogram.GetDataPoints())
	case *metricspb.Metric_Summary:
		return len(data.Summary.GetDataPoints())
	default:
		return 0
	}
}
