package otel

import (
	config "github.com/inference-gateway/inference-gateway/config"
	otel "go.opentelemetry.io/otel"
	stdouttrace "go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	propagation "go.opentelemetry.io/otel/propagation"
	resource "go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.17.0"
	trace "go.opentelemetry.io/otel/trace"
)

type TracerProvider = *sdktrace.TracerProvider

type TraceSpan = trace.Span

//go:generate mockgen -source=otel.go -destination=../tests/mocks/otel.go -package=mocks
type OpenTelemetry interface {
	Init(config config.Config) (TracerProvider, error)
}

type OpenTelemetryImpl struct{}

func (o *OpenTelemetryImpl) Init(config config.Config) (TracerProvider, error) {
	var exporter sdktrace.SpanExporter
	var err error

	exporter, err = stdouttrace.New()
	if err != nil {
		return nil, err
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceNameKey.String(config.ApplicationName),
		)),
	)

	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.TraceContext{})

	return tp, nil
}
