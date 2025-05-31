package otel

import (
	"context"

	"github.com/inference-gateway/inference-gateway/config"
	"github.com/inference-gateway/inference-gateway/logger"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.27.0"
)

// OpenTelemetry defines the operations for telemetry
type OpenTelemetry interface {
	Init(config config.Config, logger logger.Logger) error
	// Application level metrics
	RecordTokenUsage(ctx context.Context, provider, model string, promptTokens, completionTokens, totalTokens int64)

	// TODO - implement the usage of this metric probably possible to get a similar metrics from the general infrastructure request response metrics
	RecordLatency(ctx context.Context, provider, model string, queueTime, promptTime, completionTime, totalTime float64)
	RecordRequestCount(ctx context.Context, provider, requestType string)
	RecordResponseStatus(ctx context.Context, provider, requestType, requestPath string, statusCode int)
	RecordRequestDuration(ctx context.Context, provider, requestType, requestPath string, durationMs float64)
	ShutDown(ctx context.Context) error
}

type OpenTelemetryImpl struct {
	logger        logger.Logger
	meterProvider *sdkmetric.MeterProvider
	meter         metric.Meter

	// Metrics
	promptTokensCounter     metric.Int64Counter
	completionTokensCounter metric.Int64Counter
	totalTokensCounter      metric.Int64Counter

	// TODO - Implement them
	queueTimeHistogram       metric.Float64Histogram
	promptTimeHistogram      metric.Float64Histogram
	completionTimeHistogram  metric.Float64Histogram
	totalTimeHistogram       metric.Float64Histogram
	requestCounter           metric.Int64Counter
	responseStatusCounter    metric.Int64Counter
	requestDurationHistogram metric.Float64Histogram
}

func (o *OpenTelemetryImpl) Init(cfg config.Config, log logger.Logger) error {
	o.logger = log

	o.logger.Info("initializing opentelemetry",
		"service_name", config.APPLICATION_NAME,
		"version", config.VERSION,
		"environment", cfg.Environment)

	// Create a Prometheus exporter
	exporter, err := prometheus.New()
	if err != nil {
		o.logger.Error("failed to create prometheus exporter", err)
		return err
	}

	o.logger.Debug("prometheus exporter created successfully")

	// Create resource with service information
	res := resource.NewWithAttributes(
		semconv.SchemaURL,
		semconv.ServiceName(config.APPLICATION_NAME),
		semconv.ServiceVersion(config.VERSION),
		semconv.DeploymentEnvironmentName(cfg.Environment),
	)

	o.logger.Debug("opentelemetry resource created",
		"service_name", config.APPLICATION_NAME,
		"service_version", config.VERSION)

	// Define histogram boundaries for metrics
	histogramBoundaries := []float64{1, 5, 10, 25, 50, 75, 100, 250, 500, 750, 1000, 2500, 5000, 7500, 10000}

	// Create a view to customize histogram boundaries
	latencyView := sdkmetric.NewView(
		sdkmetric.Instrument{
			Kind: sdkmetric.InstrumentKindHistogram,
		},
		sdkmetric.Stream{
			Aggregation: sdkmetric.AggregationExplicitBucketHistogram{
				Boundaries: histogramBoundaries,
			},
		},
	)

	o.logger.Debug("histogram boundaries configured", "boundaries", histogramBoundaries)

	// Create meter provider with the Prometheus exporter
	o.meterProvider = sdkmetric.NewMeterProvider(
		sdkmetric.WithResource(res),
		sdkmetric.WithReader(exporter),
		sdkmetric.WithView(latencyView),
	)

	// Set global meter provider
	otel.SetMeterProvider(o.meterProvider)

	o.logger.Debug("meter provider created and set globally")

	// Create meter
	o.meter = o.meterProvider.Meter(config.APPLICATION_NAME)

	o.logger.Debug("meter created", "name", config.APPLICATION_NAME)

	// Initialize metrics
	o.logger.Debug("initializing opentelemetry metrics")
	var err1, err2, err3, err4, err5, err6, err7, err8, err9, err10 error

	o.promptTokensCounter, err1 = o.meter.Int64Counter("llm_usage_prompt_tokens",
		metric.WithDescription("Number of prompt tokens used"))

	o.completionTokensCounter, err2 = o.meter.Int64Counter("llm_usage_completion_tokens",
		metric.WithDescription("Number of completion tokens used"))

	o.totalTokensCounter, err3 = o.meter.Int64Counter("llm_usage_total_tokens",
		metric.WithDescription("Total number of tokens used"))

	// o.queueTimeHistogram, err4 = o.meter.Float64Histogram("llm_latency_queue_time",
	// 	metric.WithDescription("Time spent in queue before processing"),
	// 	metric.WithUnit("ms"))

	// o.promptTimeHistogram, err5 = o.meter.Float64Histogram("llm_latency_prompt_time",
	// 	metric.WithDescription("Time spent processing the prompt"),
	// 	metric.WithUnit("ms"))

	// o.completionTimeHistogram, err6 = o.meter.Float64Histogram("llm_latency_completion_time",
	// 	metric.WithDescription("Time spent generating the completion"),
	// 	metric.WithUnit("ms"))

	// o.totalTimeHistogram, err7 = o.meter.Float64Histogram("llm_latency_total_time",
	// 	metric.WithDescription("Total time from request to response"),
	// 	metric.WithUnit("ms"))

	o.requestCounter, err8 = o.meter.Int64Counter("llm_requests_total",
		metric.WithDescription("Total number of requests processed"))

	o.responseStatusCounter, err9 = o.meter.Int64Counter("llm_responses_total",
		metric.WithDescription("Total number of responses by status code"))

	o.requestDurationHistogram, err10 = o.meter.Float64Histogram("llm_request_duration",
		metric.WithDescription("End-to-end request duration"),
		metric.WithUnit("ms"))

	// Check for errors
	for i, err := range []error{err1, err2, err3, err4, err5, err6, err7, err8, err9, err10} {
		if err != nil {
			metricNames := []string{
				"llm_usage_prompt_tokens", "llm_usage_completion_tokens", "llm_usage_total_tokens",
				"llm_latency_queue_time", "llm_latency_prompt_time", "llm_latency_completion_time",
				"llm_latency_total_time", "llm_requests_total", "llm_responses_total", "llm_request_duration",
			}
			o.logger.Error("failed to create metric", err, "metric_name", metricNames[i])
			return err
		}
	}

	o.logger.Info("opentelemetry initialization completed successfully",
		"metrics_initialized", 7, // Only counting non-commented metrics
		"prometheus_endpoint", "/metrics")

	return nil
}

func (o *OpenTelemetryImpl) RecordTokenUsage(ctx context.Context, provider, model string, promptTokens, completionTokens, totalTokens int64) {
	attributes := []attribute.KeyValue{
		attribute.String("provider", provider),
		attribute.String("model", model),
	}

	o.promptTokensCounter.Add(ctx, promptTokens, metric.WithAttributes(attributes...))
	o.completionTokensCounter.Add(ctx, completionTokens, metric.WithAttributes(attributes...))
	o.totalTokensCounter.Add(ctx, totalTokens, metric.WithAttributes(attributes...))
}

func (o *OpenTelemetryImpl) RecordLatency(ctx context.Context, provider, model string, queueTime, promptTime, completionTime, totalTime float64) {
	attributes := []attribute.KeyValue{
		attribute.String("provider", provider),
		attribute.String("model", model),
	}

	o.queueTimeHistogram.Record(ctx, queueTime, metric.WithAttributes(attributes...))
	o.promptTimeHistogram.Record(ctx, promptTime, metric.WithAttributes(attributes...))
	o.completionTimeHistogram.Record(ctx, completionTime, metric.WithAttributes(attributes...))
	o.totalTimeHistogram.Record(ctx, totalTime, metric.WithAttributes(attributes...))
}

func (o *OpenTelemetryImpl) RecordRequestCount(ctx context.Context, provider, requestType string) {
	attributes := []attribute.KeyValue{
		attribute.String("provider", provider),
		attribute.String("request_type", requestType),
	}

	o.requestCounter.Add(ctx, 1, metric.WithAttributes(attributes...))
}

func (o *OpenTelemetryImpl) RecordResponseStatus(ctx context.Context, provider, requestType, requestPath string, statusCode int) {
	attributes := []attribute.KeyValue{
		attribute.String("provider", provider),
		attribute.String("request_method", requestType),
		attribute.String("request_path", requestPath),
		attribute.Int("status_code", statusCode),
	}

	o.responseStatusCounter.Add(ctx, 1, metric.WithAttributes(attributes...))
}

func (o *OpenTelemetryImpl) RecordRequestDuration(ctx context.Context, provider, requestType, requestPath string, durationMs float64) {
	attributes := []attribute.KeyValue{
		attribute.String("provider", provider),
		attribute.String("request_method", requestType),
		attribute.String("request_path", requestPath),
	}

	o.requestDurationHistogram.Record(ctx, durationMs, metric.WithAttributes(attributes...))
}

func (o *OpenTelemetryImpl) ShutDown(ctx context.Context) error {
	return o.meterProvider.Shutdown(ctx)
}
