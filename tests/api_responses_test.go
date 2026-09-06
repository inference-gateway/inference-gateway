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

// responsesTestBodyLimit keeps the request-too-large case small while staying
// above every valid request body used in these tests.
const responsesTestBodyLimit = 512

func newResponsesTestRouter(t *testing.T, upstreamURL string) api.Router {
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

	responsesEndpoint := constants.OpenaiResponsesEndpoint
	providerCfg := map[types.Provider]*registry.ProviderConfig{
		constants.OpenaiID: {
			ID:        constants.OpenaiID,
			Name:      constants.OpenaiDisplayName,
			URL:       upstreamURL,
			Token:     "test-openai-key",
			AuthType:  constants.AuthTypeBearer,
			Endpoints: types.Endpoints{Responses: &responsesEndpoint},
		},
		constants.AnthropicID: {
			ID:       constants.AnthropicID,
			Name:     constants.AnthropicDisplayName,
			URL:      upstreamURL,
			Token:    "test-anthropic-key",
			AuthType: constants.AuthTypeXheader,
		},
	}

	cfg := config.Config{
		Server: &config.ServerConfig{
			ReadTimeout:        5 * time.Second,
			WriteTimeout:       5 * time.Second,
			MaxRequestBodySize: responsesTestBodyLimit,
		},
		Providers: providerCfg,
	}

	return api.NewRouter(cfg, log, registry.NewProviderRegistry(providerCfg, log), mockClient, nil, nil, nil)
}

func TestResponsesHandler_NonStreamingPassthrough(t *testing.T) {
	var upstreamBody map[string]any
	var upstreamHeaders http.Header
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, constants.OpenaiResponsesEndpoint, r.URL.Path)
		upstreamHeaders = r.Header.Clone()
		require.NoError(t, json.NewDecoder(r.Body).Decode(&upstreamBody))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"resp_1","object":"response","model":"gpt-4o","status":"completed","output":[{"type":"message","role":"assistant","content":[{"type":"output_text","text":"Hi"}]}]}`))
	}))
	defer server.Close()

	router := newResponsesTestRouter(t, server.URL)
	r := gin.New()
	r.POST("/v1/responses", router.ResponsesHandler)

	reqBody := `{"model":"openai/gpt-4o","input":"Hello","metadata":{"trace":"abc"}}`
	w := httptest.NewRecorder()
	req, err := http.NewRequest("POST", "/v1/responses", strings.NewReader(reqBody))
	require.NoError(t, err)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	assert.Equal(t, "gpt-4o", upstreamBody["model"], "provider prefix should be stripped")
	assert.Equal(t, "Bearer test-openai-key", upstreamHeaders.Get("Authorization"))
	assert.Equal(t, map[string]any{"trace": "abc"}, upstreamBody["metadata"], "other fields must pass through untouched")

	var response map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
	assert.Equal(t, "resp_1", response["id"])
}

func TestResponsesHandler_StreamingPassthrough(t *testing.T) {
	sse := "event: response.created\ndata: {\"type\":\"response.created\"}\n\nevent: response.completed\ndata: {\"type\":\"response.completed\"}\n\n"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte(sse))
	}))
	defer server.Close()

	router := newResponsesTestRouter(t, server.URL)
	r := gin.New()
	r.POST("/v1/responses", router.ResponsesHandler)

	gatewayServer := httptest.NewServer(r)
	defer gatewayServer.Close()

	resp, err := http.Post(gatewayServer.URL+"/v1/responses", "application/json", strings.NewReader(`{"model":"openai/gpt-4o","input":"Hello","stream":true}`))
	require.NoError(t, err)
	defer resp.Body.Close()

	require.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Contains(t, resp.Header.Get("Content-Type"), "text/event-stream")
	respBody, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.Equal(t, sse, string(respBody), "ResponseStreamEvent frames should be relayed verbatim")
}

func TestResponsesHandler_Errors(t *testing.T) {
	tests := []struct {
		name           string
		body           string
		expectedStatus int
		expectedMsg    string
	}{
		{
			name:           "Unsupported provider returns 400",
			body:           `{"model":"anthropic/claude-sonnet-4-5","input":"Hello"}`,
			expectedStatus: http.StatusBadRequest,
			expectedMsg:    "The Responses API is not supported by this provider yet.",
		},
		{
			name:           "Unknown provider prefix returns 400",
			body:           `{"model":"gpt-4o","input":"Hello"}`,
			expectedStatus: http.StatusBadRequest,
			expectedMsg:    "Unable to determine provider for model",
		},
		{
			name:           "Invalid JSON returns 400",
			body:           `{not json`,
			expectedStatus: http.StatusBadRequest,
			expectedMsg:    "Failed to decode request",
		},
		{
			name:           "Request body over the limit returns 413",
			body:           `{"model":"openai/gpt-4o","input":"` + strings.Repeat("a", responsesTestBodyLimit) + `"}`,
			expectedStatus: http.StatusRequestEntityTooLarge,
			expectedMsg:    "Request body too large",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := newResponsesTestRouter(t, "http://localhost:0")
			r := gin.New()
			r.POST("/v1/responses", router.ResponsesHandler)

			w := httptest.NewRecorder()
			req, err := http.NewRequest("POST", "/v1/responses", strings.NewReader(tt.body))
			require.NoError(t, err)
			r.ServeHTTP(w, req)

			require.Equal(t, tt.expectedStatus, w.Code)

			var response api.ErrorResponse
			require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
			assert.Contains(t, response.Error, tt.expectedMsg)
		})
	}
}
