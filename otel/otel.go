//go:generate mockgen -source=otel.go -destination=../tests/mocks/otel.go -package=mocks
package otel

import (
	"cmp"
	"context"
	"errors"

	config "github.com/inference-gateway/inference-gateway/config"
	logger "github.com/inference-gateway/inference-gateway/logger"
	otel "go.opentelemetry.io/otel"
	attribute "go.opentelemetry.io/otel/attribute"
	otlptracehttp "go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	prometheus "go.opentelemetry.io/otel/exporters/prometheus"
	metric "go.opentelemetry.io/otel/metric"
	propagation "go.opentelemetry.io/otel/propagation"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	resource "go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.41.0"
	colmetricspb "go.opentelemetry.io/proto/otlp/collector/metrics/v1"
)

// SourceGateway is the source attribute value for metrics recorded by the
// gateway itself; pushed metrics must carry a different source.
const SourceGateway = "gateway"

// TeamUnknown is the team attribute value used when no organizational unit can
// be attributed to a measurement. Defaulting to it (instead of dropping the
// label) keeps the label present on every series so dashboards stay stable.
const TeamUnknown = "unknown"

// sourceKey labels every series with where the measurement came from,
// distinguishing gateway-observed traffic from subscription clients pushing
// via the OTLP endpoint.
const sourceKey = attribute.Key("source")

// teamKey attributes a measurement to an organizational unit (team or
// department) so usage and failures can be broken down per team.
const teamKey = attribute.Key("team")

// IngestResult summarizes an OTLP push ingestion.
type IngestResult struct {
	AcceptedDataPoints int64
	RejectedDataPoints int64
	ErrorMessage       string
}

// OpenTelemetry defines the operations for telemetry
type OpenTelemetry interface {
	Init(config config.Config, logger logger.Logger) error

	RecordTokenUsage(ctx context.Context, source, team, provider, model string, inputTokens, outputTokens int64)
	RecordRequestDuration(ctx context.Context, source, team, provider, model, errorType string, seconds float64)
	RecordToolCall(ctx context.Context, source, team, provider, model, toolType, toolName string)

	// IngestMetrics maps an OTLP push payload onto the gateway's instruments.
	IngestMetrics(ctx context.Context, req *colmetricspb.ExportMetricsServiceRequest) IngestResult

	ShutDown(ctx context.Context) error
}

type OpenTelemetryImpl struct {
	logger         logger.Logger
	meterProvider  *sdkmetric.MeterProvider
	tracerProvider *sdktrace.TracerProvider
	meter          metric.Meter

	// GenAI semantic-convention instruments
	tokenUsageHistogram     metric.Int64Histogram   // gen_ai.client.token.usage
	serverRequestDuration   metric.Float64Histogram // gen_ai.server.request.duration
	clientOperationDuration metric.Float64Histogram // gen_ai.client.operation.duration (push only)
	clientTimeToFirstChunk  metric.Float64Histogram // gen_ai.client.operation.time_to_first_chunk (push only)
	serverTimeToFirstToken  metric.Float64Histogram // gen_ai.server.time_to_first_token (push only)
	executeToolDuration     metric.Float64Histogram // gen_ai.execute_tool.duration (push only)
	toolCallCounter         metric.Int64Counter     // inference_gateway.tool_calls
}

// Semconv-recommended bucket boundaries: durations in seconds, token counts in powers of 4.
var (
	durationBoundaries = []float64{0.01, 0.02, 0.04, 0.08, 0.16, 0.32, 0.64, 1.28, 2.56, 5.12, 10.24, 20.48, 40.96, 81.92}
	tokenBoundaries    = []float64{1, 4, 16, 64, 256, 1024, 4096, 16384, 65536, 262144, 1048576, 4194304, 16777216, 67108864}
)

func (o *OpenTelemetryImpl) Init(cfg config.Config, log logger.Logger) error {
	o.logger = log

	o.logger.Info("initializing opentelemetry",
		"service_name", config.APPLICATION_NAME,
		"version", config.VERSION,
		"environment", cfg.Environment)

	exporter, err := prometheus.New()
	if err != nil {
		o.logger.Error("failed to create prometheus exporter", err)
		return err
	}

	res := resource.NewWithAttributes(
		semconv.SchemaURL,
		semconv.ServiceName(config.APPLICATION_NAME),
		semconv.ServiceVersion(config.VERSION),
		semconv.DeploymentEnvironmentNameKey.String(cfg.Environment),
	)

	o.meterProvider = sdkmetric.NewMeterProvider(
		sdkmetric.WithResource(res),
		sdkmetric.WithReader(exporter),
		sdkmetric.WithView(metricViews()...),
	)

	otel.SetMeterProvider(o.meterProvider)

	if err := o.initInstruments(o.meterProvider); err != nil {
		return err
	}

	if cfg.Telemetry.TracingEnable {
		traceExporter, err := otlptracehttp.New(context.Background(),
			otlptracehttp.WithEndpointURL(cfg.Telemetry.TracingOtlpEndpoint))
		if err != nil {
			o.logger.Error("failed to create otlp trace exporter", err)
			return err
		}

		o.tracerProvider = sdktrace.NewTracerProvider(
			sdktrace.WithResource(res),
			sdktrace.WithBatcher(traceExporter),
		)
		otel.SetTracerProvider(o.tracerProvider)
		otel.SetTextMapPropagator(propagation.TraceContext{})

		o.logger.Info("opentelemetry tracing enabled",
			"otlp_endpoint", cfg.Telemetry.TracingOtlpEndpoint)
	}

	o.logger.Info("opentelemetry initialization completed successfully",
		"prometheus_endpoint", "/metrics")

	return nil
}

func metricViews() []sdkmetric.View {
	durationView := sdkmetric.NewView(
		sdkmetric.Instrument{Kind: sdkmetric.InstrumentKindHistogram, Unit: "s"},
		sdkmetric.Stream{Aggregation: sdkmetric.AggregationExplicitBucketHistogram{
			Boundaries: durationBoundaries,
		}},
	)
	tokenView := sdkmetric.NewView(
		sdkmetric.Instrument{Name: "gen_ai.client.token.usage"},
		sdkmetric.Stream{Aggregation: sdkmetric.AggregationExplicitBucketHistogram{
			Boundaries: tokenBoundaries,
		}},
	)
	return []sdkmetric.View{durationView, tokenView}
}

// initInstruments creates the instruments on the given provider. Split from
// Init so tests can use a manual reader instead of the Prometheus exporter.
func (o *OpenTelemetryImpl) initInstruments(provider *sdkmetric.MeterProvider) error {
	o.meter = provider.Meter(config.APPLICATION_NAME)

	var errs [7]error

	o.tokenUsageHistogram, errs[0] = o.meter.Int64Histogram("gen_ai.client.token.usage",
		metric.WithDescription("Number of input and output tokens used per operation"),
		metric.WithUnit("{token}"))

	o.serverRequestDuration, errs[1] = o.meter.Float64Histogram("gen_ai.server.request.duration",
		metric.WithDescription("Generative AI server request duration"),
		metric.WithUnit("s"))

	o.clientOperationDuration, errs[2] = o.meter.Float64Histogram("gen_ai.client.operation.duration",
		metric.WithDescription("GenAI operation duration as observed by the client"),
		metric.WithUnit("s"))

	o.clientTimeToFirstChunk, errs[3] = o.meter.Float64Histogram("gen_ai.client.operation.time_to_first_chunk",
		metric.WithDescription("Time to receive the first chunk of a streaming response"),
		metric.WithUnit("s"))

	o.serverTimeToFirstToken, errs[4] = o.meter.Float64Histogram("gen_ai.server.time_to_first_token",
		metric.WithDescription("Time to generate the first token of a response"),
		metric.WithUnit("s"))

	o.executeToolDuration, errs[5] = o.meter.Float64Histogram("gen_ai.execute_tool.duration",
		metric.WithDescription("GenAI tool execution duration"),
		metric.WithUnit("s"))

	o.toolCallCounter, errs[6] = o.meter.Int64Counter("inference_gateway.tool_calls",
		metric.WithDescription("Number of tool calls observed in model responses"),
		metric.WithUnit("{call}"))

	for _, err := range errs {
		if err != nil {
			if o.logger != nil {
				o.logger.Error("failed to create metric", err)
			}
			return err
		}
	}

	return nil
}

func (o *OpenTelemetryImpl) RecordTokenUsage(ctx context.Context, source, team, provider, model string, inputTokens, outputTokens int64) {
	base := []attribute.KeyValue{
		sourceKey.String(source),
		teamKey.String(cmp.Or(team, TeamUnknown)),
		semconv.GenAIOperationNameChat,
		semconv.GenAIProviderNameKey.String(provider),
		semconv.GenAIRequestModel(model),
	}

	o.tokenUsageHistogram.Record(ctx, inputTokens,
		metric.WithAttributes(append(base, semconv.GenAITokenTypeInput)...))
	o.tokenUsageHistogram.Record(ctx, outputTokens,
		metric.WithAttributes(append(base, semconv.GenAITokenTypeOutput)...))
}

func (o *OpenTelemetryImpl) RecordRequestDuration(ctx context.Context, source, team, provider, model, errorType string, seconds float64) {
	attributes := []attribute.KeyValue{
		sourceKey.String(source),
		teamKey.String(cmp.Or(team, TeamUnknown)),
		semconv.GenAIOperationNameChat,
		semconv.GenAIProviderNameKey.String(provider),
		semconv.GenAIRequestModel(model),
	}
	if errorType != "" {
		attributes = append(attributes, semconv.ErrorTypeKey.String(errorType))
	}

	o.serverRequestDuration.Record(ctx, seconds, metric.WithAttributes(attributes...))
}

func (o *OpenTelemetryImpl) RecordToolCall(ctx context.Context, source, team, provider, model, toolType, toolName string) {
	attributes := []attribute.KeyValue{
		sourceKey.String(source),
		teamKey.String(cmp.Or(team, TeamUnknown)),
		semconv.GenAIProviderNameKey.String(provider),
		semconv.GenAIRequestModel(model),
		semconv.GenAIToolType(toolType),
		semconv.GenAIToolName(toolName),
	}

	o.toolCallCounter.Add(ctx, 1, metric.WithAttributes(attributes...))
}

func (o *OpenTelemetryImpl) ShutDown(ctx context.Context) error {
	err := o.meterProvider.Shutdown(ctx)
	if o.tracerProvider != nil {
		err = errors.Join(err, o.tracerProvider.Shutdown(ctx))
	}
	return err
}
