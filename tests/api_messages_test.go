package tests

import (
	"encoding/json"
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

	providersmocks "github.com/inference-gateway/inference-gateway/tests/mocks/providers"

	api "github.com/inference-gateway/inference-gateway/api"
	config "github.com/inference-gateway/inference-gateway/config"
	logger "github.com/inference-gateway/inference-gateway/logger"
	constants "github.com/inference-gateway/inference-gateway/providers/constants"
	registry "github.com/inference-gateway/inference-gateway/providers/registry"
	types "github.com/inference-gateway/inference-gateway/providers/types"
)

func newMessagesTestRouter(t *testing.T, upstreamURL string) api.Router {
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
		constants.AnthropicID: {
			ID:       constants.AnthropicID,
			Name:     constants.AnthropicDisplayName,
			URL:      upstreamURL,
			Token:    "test-anthropic-key",
			AuthType: constants.AuthTypeXheader,
			ExtraHeaders: map[string][]string{
				"anthropic-version": {"2023-06-01"},
			},
		},
		constants.OpenaiID: {
			ID:       constants.OpenaiID,
			Name:     constants.OpenaiDisplayName,
			URL:      upstreamURL,
			Token:    "test-openai-key",
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

	return api.NewRouter(cfg, log, registry.NewProviderRegistry(providerCfg, log), mockClient, nil, nil)
}

func TestMessagesHandler_NonStreamingPassthrough(t *testing.T) {
	var upstreamBody map[string]any
	var upstreamHeaders http.Header
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/messages", r.URL.Path)
		upstreamHeaders = r.Header.Clone()
		require.NoError(t, json.NewDecoder(r.Body).Decode(&upstreamBody))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"msg_1","type":"message","role":"assistant","model":"claude-sonnet-4-5","content":[{"type":"text","text":"Hi"}],"stop_reason":"end_turn","usage":{"input_tokens":10,"output_tokens":2,"cache_creation_input_tokens":5,"cache_read_input_tokens":3}}`))
	}))
	defer server.Close()

	router := newMessagesTestRouter(t, server.URL)
	r := gin.New()
	r.POST("/v1/messages", router.MessagesHandler)

	reqBody := `{"model":"anthropic/claude-sonnet-4-5","max_tokens":16,"system":[{"type":"text","text":"be brief","cache_control":{"type":"ephemeral"}}],"messages":[{"role":"user","content":"Hello"}]}`
	w := httptest.NewRecorder()
	req, err := http.NewRequest("POST", "/v1/messages", strings.NewReader(reqBody))
	require.NoError(t, err)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	assert.Equal(t, "claude-sonnet-4-5", upstreamBody["model"], "provider prefix should be stripped")
	assert.Equal(t, "test-anthropic-key", upstreamHeaders.Get("x-api-key"))
	assert.Equal(t, "2023-06-01", upstreamHeaders.Get("anthropic-version"))
	system := upstreamBody["system"].([]any)[0].(map[string]any)
	assert.Equal(t, map[string]any{"type": "ephemeral"}, system["cache_control"], "cache_control must pass through untouched")

	var response map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
	usage := response["usage"].(map[string]any)
	assert.Equal(t, float64(5), usage["cache_creation_input_tokens"])
	assert.Equal(t, float64(3), usage["cache_read_input_tokens"])
}

func TestMessagesHandler_StreamingPassthrough(t *testing.T) {
	sse := "event: message_start\ndata: {\"type\":\"message_start\"}\n\nevent: message_stop\ndata: {\"type\":\"message_stop\"}\n\n"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte(sse))
	}))
	defer server.Close()

	router := newMessagesTestRouter(t, server.URL)
	r := gin.New()
	r.POST("/v1/messages", router.MessagesHandler)

	gatewayServer := httptest.NewServer(r)
	defer gatewayServer.Close()

	resp, err := http.Post(gatewayServer.URL+"/v1/messages", "application/json", strings.NewReader(`{"model":"anthropic/claude-sonnet-4-5","max_tokens":16,"stream":true,"messages":[{"role":"user","content":"Hello"}]}`))
	require.NoError(t, err)
	defer resp.Body.Close()

	require.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Contains(t, resp.Header.Get("Content-Type"), "text/event-stream")
	respBody, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.Equal(t, sse, string(respBody), "Anthropic SSE events should be relayed verbatim")
}

func TestMessagesHandler_Errors(t *testing.T) {
	tests := []struct {
		name           string
		body           string
		expectedStatus int
		expectedType   string
		expectedMsg    string
	}{
		{
			name:           "Unsupported provider returns 400 in Anthropic error envelope",
			body:           `{"model":"openai/gpt-4o","max_tokens":16,"messages":[]}`,
			expectedStatus: http.StatusBadRequest,
			expectedType:   "not_supported_error",
			expectedMsg:    "The Messages API is not supported by this provider yet.",
		},
		{
			name:           "Unknown provider prefix returns 400",
			body:           `{"model":"claude-sonnet-4-5","max_tokens":16,"messages":[]}`,
			expectedStatus: http.StatusBadRequest,
			expectedType:   "invalid_request_error",
			expectedMsg:    "Unable to determine provider for model",
		},
		{
			name:           "Invalid JSON returns 400",
			body:           `{not json`,
			expectedStatus: http.StatusBadRequest,
			expectedType:   "invalid_request_error",
			expectedMsg:    "Failed to decode request",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := newMessagesTestRouter(t, "http://localhost:0")
			r := gin.New()
			r.POST("/v1/messages", router.MessagesHandler)

			w := httptest.NewRecorder()
			req, err := http.NewRequest("POST", "/v1/messages", strings.NewReader(tt.body))
			require.NoError(t, err)
			r.ServeHTTP(w, req)

			require.Equal(t, tt.expectedStatus, w.Code)

			var response types.MessagesError
			require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
			assert.Equal(t, types.MessagesErrorTypeError, response.Type)
			assert.Equal(t, tt.expectedType, response.Error.Type)
			assert.Contains(t, response.Error.Message, tt.expectedMsg)
		})
	}
}
