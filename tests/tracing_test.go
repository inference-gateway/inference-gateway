package tests

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
	gomock "go.uber.org/mock/gomock"

	gin "github.com/gin-gonic/gin"
	otelgin "go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
	otelapi "go.opentelemetry.io/otel"
	attribute "go.opentelemetry.io/otel/attribute"
	otelcodes "go.opentelemetry.io/otel/codes"
	propagation "go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	tracetest "go.opentelemetry.io/otel/sdk/trace/tracetest"
	semconv "go.opentelemetry.io/otel/semconv/v1.41.0"

	api "github.com/inference-gateway/inference-gateway/api"
	middlewares "github.com/inference-gateway/inference-gateway/api/middlewares"
	config "github.com/inference-gateway/inference-gateway/config"
	mcp "github.com/inference-gateway/inference-gateway/internal/mcp"
	logger "github.com/inference-gateway/inference-gateway/logger"
	constants "github.com/inference-gateway/inference-gateway/providers/constants"
	registry "github.com/inference-gateway/inference-gateway/providers/registry"
	types "github.com/inference-gateway/inference-gateway/providers/types"
	mocks "github.com/inference-gateway/inference-gateway/tests/mocks"
	mcpmocks "github.com/inference-gateway/inference-gateway/tests/mocks/mcp"
	providersmocks "github.com/inference-gateway/inference-gateway/tests/mocks/providers"
)

// setupTracing installs an in-memory span recorder as the global tracer
// provider plus the W3C propagator, mirroring what otel.Init does when
// tracing is enabled.
func setupTracing(t *testing.T) *tracetest.SpanRecorder {
	t.Helper()
	sr := tracetest.NewSpanRecorder()
	otelapi.SetTracerProvider(sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(sr)))
	otelapi.SetTextMapPropagator(propagation.TraceContext{})
	return sr
}

func findAttr(attrs []attribute.KeyValue, key attribute.Key) (string, bool) {
	for _, a := range attrs {
		if a.Key == key {
			return a.Value.String(), true
		}
	}
	return "", false
}

func TestTracingMessagesHandler(t *testing.T) {
	sr := setupTracing(t)

	var upstreamTraceparent string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamTraceparent = r.Header.Get("traceparent")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"msg_1","type":"message","role":"assistant","content":[]}`))
	}))
	defer server.Close()

	router := newMessagesTestRouter(t, server.URL)
	r := gin.New()
	r.Use(otelgin.Middleware("inference-gateway"))
	r.POST("/v1/messages", router.MessagesHandler)

	w := httptest.NewRecorder()
	req, err := http.NewRequest("POST", "/v1/messages", strings.NewReader(`{"model":"anthropic/claude-sonnet-4-5","max_tokens":16,"messages":[{"role":"user","content":"Hello"}]}`))
	require.NoError(t, err)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	assert.NotEmpty(t, upstreamTraceparent, "upstream request must carry traceparent")

	spans := sr.Ended()
	require.Len(t, spans, 1)
	provider, ok := findAttr(spans[0].Attributes(), semconv.GenAIProviderNameKey)
	require.True(t, ok)
	assert.Equal(t, "anthropic", provider)
	model, ok := findAttr(spans[0].Attributes(), semconv.GenAIRequestModelKey)
	require.True(t, ok)
	assert.Equal(t, "anthropic/claude-sonnet-4-5", model)
}

func TestTracingTelemetryMiddlewareEnrichment(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		wantStatus otelcodes.Code
	}{
		{"success", http.StatusOK, otelcodes.Unset},
		{"upstream error", http.StatusInternalServerError, otelcodes.Error},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sr := setupTracing(t)
			ctrl := gomock.NewController(t)
			t.Cleanup(ctrl.Finish)

			mockOtel := mocks.NewMockOpenTelemetry(ctrl)
			mockOtel.EXPECT().RecordRequestDuration(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
			mockOtel.EXPECT().RecordTokenUsage(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
			mockOtel.EXPECT().RecordToolCall(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

			log, err := logger.NewLogger("test")
			require.NoError(t, err)
			telemetry, err := middlewares.NewTelemetryMiddleware(config.Config{}, mockOtel, log)
			require.NoError(t, err)

			r := gin.New()
			r.Use(otelgin.Middleware("inference-gateway"))
			r.Use(telemetry.Middleware())
			r.POST("/v1/chat/completions", func(c *gin.Context) {
				c.Data(tt.statusCode, "application/json", []byte(`{"choices":[],"usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}}`))
			})

			w := httptest.NewRecorder()
			req, err := http.NewRequest("POST", "/v1/chat/completions", strings.NewReader(`{"model":"openai/gpt-4o","messages":[{"role":"user","content":"hi"}]}`))
			require.NoError(t, err)
			r.ServeHTTP(w, req)

			spans := sr.Ended()
			require.Len(t, spans, 1)
			provider, ok := findAttr(spans[0].Attributes(), semconv.GenAIProviderNameKey)
			require.True(t, ok)
			assert.Equal(t, "openai", provider)
			model, ok := findAttr(spans[0].Attributes(), semconv.GenAIRequestModelKey)
			require.True(t, ok)
			assert.Equal(t, "openai/gpt-4o", model)
			assert.Equal(t, tt.wantStatus, spans[0].Status().Code)
		})
	}
}

func TestTracingExecuteToolsSpans(t *testing.T) {
	sr := setupTracing(t)
	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	log, err := logger.NewLogger("test")
	require.NoError(t, err)

	mockMCP := mcpmocks.NewMockMCPClientInterface(ctrl)
	mockMCP.EXPECT().GetServerForTool("search").Return("http://mcp.local", nil)
	mockMCP.EXPECT().ExecuteTool(gomock.Any(), gomock.Any(), "http://mcp.local").Return(&mcp.CallToolResult{}, nil)
	mockMCP.EXPECT().GetServerForTool("missing").Return("", assert.AnError)

	agent := mcp.NewAgent(log, mockMCP)
	results, err := agent.ExecuteTools(context.Background(), []types.ChatCompletionMessageToolCall{
		{ID: "1", Function: types.ChatCompletionMessageToolCallFunction{Name: "mcp_search", Arguments: "{}"}},
		{ID: "2", Function: types.ChatCompletionMessageToolCallFunction{Name: "mcp_missing", Arguments: "{}"}},
	})
	require.NoError(t, err)
	require.Len(t, results, 2)

	spans := sr.Ended()
	require.Len(t, spans, 2)

	assert.Equal(t, "execute_tool search", spans[0].Name())
	toolName, ok := findAttr(spans[0].Attributes(), semconv.GenAIToolNameKey)
	require.True(t, ok)
	assert.Equal(t, "search", toolName)
	serverURL, ok := findAttr(spans[0].Attributes(), attribute.Key("mcp.server.url"))
	require.True(t, ok)
	assert.Equal(t, "http://mcp.local", serverURL)
	assert.Equal(t, otelcodes.Unset, spans[0].Status().Code)

	assert.Equal(t, "execute_tool missing", spans[1].Name())
	assert.Equal(t, otelcodes.Error, spans[1].Status().Code)
}

func TestTracingProviderCorePropagation(t *testing.T) {
	setupTracing(t)
	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	log, err := logger.NewLogger("test")
	require.NoError(t, err)

	var capturedHeaders []http.Header
	mockClient := providersmocks.NewMockClient(ctrl)
	mockClient.EXPECT().
		Do(gomock.Any()).
		DoAndReturn(func(req *http.Request) (*http.Response, error) {
			capturedHeaders = append(capturedHeaders, req.Header.Clone())
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(`{}`)),
				Header:     http.Header{"Content-Type": {"application/json"}},
			}, nil
		}).
		Times(2)

	providerCfg := map[types.Provider]*registry.ProviderConfig{
		constants.OpenaiID: {
			ID:       constants.OpenaiID,
			Name:     constants.OpenaiDisplayName,
			URL:      "http://upstream.local/v1",
			Token:    "test-key",
			AuthType: constants.AuthTypeBearer,
		},
	}
	provider, err := registry.NewProviderRegistry(providerCfg, log).BuildProvider(constants.OpenaiID, mockClient)
	require.NoError(t, err)

	ctx, span := otelapi.Tracer("test").Start(context.Background(), "root")
	defer span.End()

	_, _ = provider.ChatCompletions(ctx, types.CreateChatCompletionRequest{Model: "gpt-4o"})
	_, _ = provider.ListModels(ctx)

	require.Len(t, capturedHeaders, 2)
	assert.NotEmpty(t, capturedHeaders[0].Get("traceparent"), "chat completions request must carry traceparent")
	assert.NotEmpty(t, capturedHeaders[1].Get("traceparent"), "list models request must carry traceparent")
}

func TestTracingProxyPropagation(t *testing.T) {
	setupTracing(t)
	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	var upstreamTraceparent string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamTraceparent = r.Header.Get("traceparent")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{}`))
	}))
	defer server.Close()

	log, err := logger.NewLogger("test")
	require.NoError(t, err)

	providerCfg := map[types.Provider]*registry.ProviderConfig{
		constants.OpenaiID: {
			ID:       constants.OpenaiID,
			Name:     constants.OpenaiDisplayName,
			URL:      server.URL,
			Token:    "test-key",
			AuthType: constants.AuthTypeBearer,
		},
	}
	cfg := config.Config{
		Server: &config.ServerConfig{
			ReadTimeout:  5 * time.Second,
			WriteTimeout: 5 * time.Second,
		},
		Providers: providerCfg,
	}
	router := api.NewRouter(cfg, log, registry.NewProviderRegistry(providerCfg, log), providersmocks.NewMockClient(ctrl), nil, nil)

	r := gin.New()
	r.Use(otelgin.Middleware("inference-gateway"))
	r.Any("/proxy/:provider/*path", router.ProxyHandler)

	gatewayServer := httptest.NewServer(r)
	defer gatewayServer.Close()

	resp, err := http.Get(gatewayServer.URL + "/proxy/openai/models")
	require.NoError(t, err)
	defer resp.Body.Close()

	require.Equal(t, http.StatusOK, resp.StatusCode)
	assert.NotEmpty(t, upstreamTraceparent, "proxied request must carry traceparent")
}
