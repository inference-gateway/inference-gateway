package tests

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	gin "github.com/gin-gonic/gin"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
	gomock "go.uber.org/mock/gomock"

	api "github.com/inference-gateway/inference-gateway/api"
	config "github.com/inference-gateway/inference-gateway/config"
	logger "github.com/inference-gateway/inference-gateway/logger"
	constants "github.com/inference-gateway/inference-gateway/providers/constants"
	registry "github.com/inference-gateway/inference-gateway/providers/registry"
	types "github.com/inference-gateway/inference-gateway/providers/types"
	providersmocks "github.com/inference-gateway/inference-gateway/tests/mocks/providers"
)

const (
	sseContentType  = "text/event-stream"
	jsonContentType = "application/json"
)

// newProxyGateway mounts ProxyHandler in front of upstreamURL and returns the
// gateway test server. The provider client is a passthrough so the streaming
// path performs a real HTTP round trip against the upstream.
func newProxyGateway(t *testing.T, upstreamURL string, serverCfg *config.ServerConfig) *httptest.Server {
	t.Helper()

	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	mockClient := providersmocks.NewMockClient(ctrl)
	mockClient.EXPECT().
		Do(gomock.Any()).
		DoAndReturn(func(req *http.Request) (*http.Response, error) {
			return http.DefaultClient.Do(req)
		}).
		AnyTimes()

	log, err := logger.NewLogger("test")
	require.NoError(t, err)

	providerCfg := map[types.Provider]*registry.ProviderConfig{
		constants.OpenaiID: {
			ID:       constants.OpenaiID,
			Name:     constants.OpenaiDisplayName,
			URL:      upstreamURL,
			Token:    "test-key",
			AuthType: constants.AuthTypeBearer,
		},
	}
	cfg := config.Config{
		Server:    serverCfg,
		Providers: providerCfg,
	}
	router := api.NewRouter(cfg, log, registry.NewProviderRegistry(providerCfg, log), mockClient, nil, nil, nil)

	r := gin.New()
	r.Any("/proxy/:provider/*path", router.ProxyHandler)

	gateway := httptest.NewServer(r)
	t.Cleanup(gateway.Close)
	return gateway
}

// The hand-rolled streaming path of /proxy (selected by Accept:
// text/event-stream) must relay whatever the upstream answered: an error
// envelope keeps its status code and JSON content type instead of being
// wrapped in a 200 event stream, and a real event stream is passed through.
func TestProxyHandler_StreamingRelaysUpstreamResponse(t *testing.T) {
	tests := []struct {
		name                string
		upstreamStatus      int
		upstreamContentType string
		upstreamBody        string
	}{
		{
			name:                "upstream error envelope keeps its status and JSON content type",
			upstreamStatus:      http.StatusUnauthorized,
			upstreamContentType: jsonContentType,
			upstreamBody:        `{"error":{"message":"invalid api key"}}`,
		},
		{
			name:                "upstream rate limit keeps its status and JSON content type",
			upstreamStatus:      http.StatusTooManyRequests,
			upstreamContentType: jsonContentType,
			upstreamBody:        `{"error":{"message":"rate limit exceeded"}}`,
		},
		{
			name:                "upstream non-streaming success is relayed as JSON",
			upstreamStatus:      http.StatusOK,
			upstreamContentType: jsonContentType,
			upstreamBody:        `{"object":"list","data":[]}`,
		},
		{
			name:                "upstream event stream is streamed through",
			upstreamStatus:      http.StatusOK,
			upstreamContentType: sseContentType,
			upstreamBody:        "data: {\"id\":\"chunk-1\"}\n\ndata: [DONE]\n\n",
		},
		{
			name:                "final stream line without trailing newline is not dropped",
			upstreamStatus:      http.StatusOK,
			upstreamContentType: sseContentType,
			upstreamBody:        "data: {\"id\":\"chunk-1\"}\n\ndata: [DONE]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "Bearer test-key", r.Header.Get("Authorization"))
				w.Header().Set("Content-Type", tt.upstreamContentType)
				w.WriteHeader(tt.upstreamStatus)
				_, _ = w.Write([]byte(tt.upstreamBody))
			}))
			defer upstream.Close()

			gateway := newProxyGateway(t, upstream.URL, &config.ServerConfig{
				ReadTimeout:  5 * time.Second,
				WriteTimeout: 5 * time.Second,
			})

			req, err := http.NewRequest(http.MethodPost, gateway.URL+"/proxy/openai/chat/completions", strings.NewReader(`{"model":"gpt-4o","stream":true}`))
			require.NoError(t, err)
			req.Header.Set("Content-Type", jsonContentType)
			req.Header.Set("Accept", sseContentType)

			resp, err := http.DefaultClient.Do(req)
			require.NoError(t, err)
			defer resp.Body.Close()

			body, err := io.ReadAll(resp.Body)
			require.NoError(t, err)

			assert.Equal(t, tt.upstreamStatus, resp.StatusCode)
			assert.Contains(t, resp.Header.Get("Content-Type"), tt.upstreamContentType)
			assert.Equal(t, tt.upstreamBody, string(body))
		})
	}
}

// Errors the gateway raises before contacting the upstream (here: an
// oversized body) must be plain JSON errors, not responses carrying the SSE
// headers of a stream that never started.
func TestProxyHandler_StreamingRejectsOversizedBodyAsJSON(t *testing.T) {
	var upstreamCalls atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamCalls.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()

	const maxBodySize = 16
	gateway := newProxyGateway(t, upstream.URL, &config.ServerConfig{
		ReadTimeout:        5 * time.Second,
		WriteTimeout:       5 * time.Second,
		MaxRequestBodySize: maxBodySize,
	})

	req, err := http.NewRequest(http.MethodPost, gateway.URL+"/proxy/openai/chat/completions", strings.NewReader(strings.Repeat("x", maxBodySize+1)))
	require.NoError(t, err)
	req.Header.Set("Content-Type", jsonContentType)
	req.Header.Set("Accept", sseContentType)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusRequestEntityTooLarge, resp.StatusCode)
	assert.Contains(t, resp.Header.Get("Content-Type"), jsonContentType)
	assert.Empty(t, resp.Header.Get("Cache-Control"), "SSE headers must not leak onto a JSON error")
	assert.Equal(t, int32(0), upstreamCalls.Load(), "upstream must not be contacted")
}
