package otel

import (
	"context"

	config "github.com/inference-gateway/inference-gateway/config"
	otel "go.opentelemetry.io/otel"
	attribute "go.opentelemetry.io/otel/attribute"
	stdout "go.opentelemetry.io/otel/exporters/stdout/stdoutmetric"
	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	resource "go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.17.0"
)

type MeterProvider = sdkmetric.MeterProvider

//go:generate mockgen -source=otel.go -destination=../tests/mocks/otel.go -package=mocks
type OpenTelemetry interface {
	Init(config config.Config) error
	RecordTokenUsage(ctx context.Context, provider string, model string, promptTokens, completionTokens, totalTokens int64)
	RecordTokenUsageWithTime(ctx context.Context, provider string, model string,
		promptTokens, completionTokens, totalTokens int64,
		queueTime, promptTime, completionTime, totalTime float64)
}

type OpenTelemetryImpl struct {
	meterProvider *MeterProvider
	// Token counters
	promptCounter metric.Int64Counter
	compCounter   metric.Int64Counter
	totalCounter  metric.Int64Counter
	// Time histograms
	queueTimeHistogram  metric.Float64Histogram
	promptTimeHistogram metric.Float64Histogram
	compTimeHistogram   metric.Float64Histogram
	totalTimeHistogram  metric.Float64Histogram
}

func (o *OpenTelemetryImpl) Init(config config.Config) error {
	// Initialize metrics - using stdout exporter for simplicity
	metricExporter, err := stdout.New()
	if err != nil {
		return err
	}

	// Create meter provider with simple configuration
	mp := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(metricExporter)),
		sdkmetric.WithResource(resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceNameKey.String(config.ApplicationName),
		)),
	)

	// Set as global provider and store locally
	otel.SetMeterProvider(mp)
	o.meterProvider = mp

	// Initialize token counters
	meter := mp.Meter("inference-gateway")

	// Create token counters
	var errs []error
	o.promptCounter, err = meter.Int64Counter(
		"llm.usage.prompt_tokens",
		metric.WithDescription("Number of prompt tokens used"),
	)
	errs = append(errs, err)

	o.compCounter, err = meter.Int64Counter(
		"llm.usage.completion_tokens",
		metric.WithDescription("Number of completion tokens used"),
	)
	errs = append(errs, err)

	o.totalCounter, err = meter.Int64Counter(
		"llm.usage.total_tokens",
		metric.WithDescription("Total number of tokens used"),
	)
	errs = append(errs, err)

	// Create time histograms with appropriate buckets for LLM processing times
	// Times are recorded in miliseconds
	timeUnit := "ms"

	o.queueTimeHistogram, err = meter.Float64Histogram(
		"llm.latency.queue_time",
		metric.WithDescription("Time spent in queue before processing"),
		metric.WithUnit(timeUnit),
	)
	errs = append(errs, err)

	o.promptTimeHistogram, err = meter.Float64Histogram(
		"llm.latency.prompt_time",
		metric.WithDescription("Time spent processing the prompt"),
		metric.WithUnit(timeUnit),
	)
	errs = append(errs, err)

	o.compTimeHistogram, err = meter.Float64Histogram(
		"llm.latency.completion_time",
		metric.WithDescription("Time spent generating the completion"),
		metric.WithUnit(timeUnit),
	)
	errs = append(errs, err)

	o.totalTimeHistogram, err = meter.Float64Histogram(
		"llm.latency.total_time",
		metric.WithDescription("Total time from request to response"),
		metric.WithUnit(timeUnit),
	)
	errs = append(errs, err)

	// Handle all potential errors at once
	for _, e := range errs {
		if e != nil {
			return e
		}
	}

	return nil
}

func (o *OpenTelemetryImpl) GetMeter(name string) metric.Meter {
	if o.meterProvider == nil {
		return nil
	}
	return o.meterProvider.Meter(name)
}

func (o *OpenTelemetryImpl) RecordTokenUsage(ctx context.Context, provider string, model string, promptTokens, completionTokens, totalTokens int64) {
	if o.promptCounter == nil || o.compCounter == nil || o.totalCounter == nil {
		return // Not initialized
	}

	attrs := []attribute.KeyValue{
		attribute.String("provider", provider),
		attribute.String("model", model),
	}

	o.promptCounter.Add(ctx, promptTokens, metric.WithAttributes(attrs...))
	o.compCounter.Add(ctx, completionTokens, metric.WithAttributes(attrs...))
	o.totalCounter.Add(ctx, totalTokens, metric.WithAttributes(attrs...))
}

// RecordTokenUsageWithTime records both token counts and processing times
func (o *OpenTelemetryImpl) RecordTokenUsageWithTime(ctx context.Context, provider string, model string,
	promptTokens, completionTokens, totalTokens int64,
	queueTime, promptTime, completionTime, totalTime float64) {

	// Record token usage
	o.RecordTokenUsage(ctx, provider, model, promptTokens, completionTokens, totalTokens)

	// If time histograms aren't initialized, return early
	if o.queueTimeHistogram == nil || o.promptTimeHistogram == nil ||
		o.compTimeHistogram == nil || o.totalTimeHistogram == nil {
		return
	}

	// Prepare attributes
	attrs := []attribute.KeyValue{
		attribute.String("provider", provider),
		attribute.String("model", model),
	}

	// Record time metrics
	if queueTime > 0 {
		o.queueTimeHistogram.Record(ctx, queueTime, metric.WithAttributes(attrs...))
	}
	if promptTime > 0 {
		o.promptTimeHistogram.Record(ctx, promptTime, metric.WithAttributes(attrs...))
	}
	if completionTime > 0 {
		o.compTimeHistogram.Record(ctx, completionTime, metric.WithAttributes(attrs...))
	}
	if totalTime > 0 {
		o.totalTimeHistogram.Record(ctx, totalTime, metric.WithAttributes(attrs...))
	}
}
