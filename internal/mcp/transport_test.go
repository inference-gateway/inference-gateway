package mcp

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
	otelapi "go.opentelemetry.io/otel"
	propagation "go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

func TestCustomRoundTripperInjectsTraceContext(t *testing.T) {
	otelapi.SetTracerProvider(sdktrace.NewTracerProvider())
	otelapi.SetTextMapPropagator(propagation.TraceContext{})

	var traceparent string
	rt := &customRoundTripper{
		base: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			traceparent = req.Header.Get("traceparent")
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": {"application/json"}},
				Body:       io.NopCloser(strings.NewReader(`{}`)),
			}, nil
		}),
	}

	ctx, span := otelapi.Tracer("test").Start(context.Background(), "root")
	defer span.End()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://mcp.local/mcp", nil)
	require.NoError(t, err)
	resp, err := rt.RoundTrip(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.NotEmpty(t, traceparent, "mcp request must carry traceparent")
}
