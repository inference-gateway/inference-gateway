package tests

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
	gomock "go.uber.org/mock/gomock"

	providers "github.com/inference-gateway/inference-gateway/tests/mocks/providers"

	api "github.com/inference-gateway/inference-gateway/api"
	middlewares "github.com/inference-gateway/inference-gateway/api/middlewares"
	config "github.com/inference-gateway/inference-gateway/config"
	logger "github.com/inference-gateway/inference-gateway/internal/platform/logger"
	constants "github.com/inference-gateway/inference-gateway/providers/constants"
	registry "github.com/inference-gateway/inference-gateway/providers/registry"
	types "github.com/inference-gateway/inference-gateway/providers/types"
)

const (
	decisionsTestBodyLimit    = 512
	decisionsTestModel        = "gpt-6-luna"
	decisionsTestChatModel    = "gpt-6-sol"
	decisionsTestUnknownModel = "gpt-6-nonexistent"
	decisionsTestResponse     = `{"answers":[{"type":"predicate","name":"is_spam","probability":0.9}],"usage":{"input_tokens":12,"input_tokens_details":{"cached_tokens":0,"cache_write_tokens":0}}}`
)

func newDecisionsTestRouter(t *testing.T, upstreamURL string) *api.RouterImpl {
	t.Helper()
	return newDecisionsRouter(t, upstreamURL, true)
}

func newDecisionsRouter(t *testing.T, upstreamURL string, enabled bool) *api.RouterImpl {
	t.Helper()
	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	mockClient := providers.NewMockClient(ctrl)
	mockClient.EXPECT().
		Do(gomock.Any()).
		DoAndReturn(func(req *http.Request) (*http.Response, error) {
			return http.DefaultClient.Do(req)
		}).
		AnyTimes()

	log, err := logger.NewLogger("test")
	require.NoError(t, err)

	decisionsEndpoint := constants.OpenaiDecisionsEndpoint
	providerCfg := map[types.Provider]*registry.ProviderConfig{
		constants.OpenaiID: {
			ID:        constants.OpenaiID,
			Name:      constants.OpenaiDisplayName,
			URL:       upstreamURL,
			Token:     "test-openai-key",
			AuthType:  constants.AuthTypeBearer,
			Endpoints: types.Endpoints{Decisions: &decisionsEndpoint},
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
			MaxRequestBodySize: decisionsTestBodyLimit,
		},
		Providers:        providerCfg,
		DecisionsEnabled: enabled,
	}

	return api.NewRouter(cfg, log, registry.NewProviderRegistry(providerCfg, log), mockClient, nil, nil, nil, nil, nil)
}

func serveDecisions(t *testing.T, router *api.RouterImpl, target, body string) *httptest.ResponseRecorder {
	t.Helper()
	r := http.NewServeMux()
	r.HandleFunc("POST "+middlewares.DecisionsPath, router.DecisionsHandler)

	w := httptest.NewRecorder()
	req, err := http.NewRequest(http.MethodPost, target, strings.NewReader(body))
	require.NoError(t, err)
	r.ServeHTTP(w, req)
	return w
}

func newDecisionsUpstream(t *testing.T, upstreamBody *map[string]any, upstreamHeaders *http.Header) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, constants.OpenaiDecisionsEndpoint, r.URL.Path)
		*upstreamHeaders = r.Header.Clone()
		require.NoError(t, json.NewDecoder(r.Body).Decode(upstreamBody))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(decisionsTestResponse))
	}))
}

func TestDecisionsHandler_Passthrough(t *testing.T) {
	var upstreamBody map[string]any
	var upstreamHeaders http.Header
	server := newDecisionsUpstream(t, &upstreamBody, &upstreamHeaders)
	defer server.Close()

	reqBody := `{"model":"openai/` + decisionsTestModel + `","input":"Buy now!!!","questions":[{"type":"predicate","name":"is_spam","instructions":"Is this spam?"}],"prompt_cache_key":"k1"}`
	w := serveDecisions(t, newDecisionsTestRouter(t, server.URL), middlewares.DecisionsPath, reqBody)

	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, decisionsTestModel, upstreamBody["model"], "provider prefix should be stripped")
	assert.Equal(t, "Bearer test-openai-key", upstreamHeaders.Get("Authorization"))
	assert.Equal(t, "k1", upstreamBody["prompt_cache_key"], "other fields must pass through untouched")
	assert.Len(t, upstreamBody["questions"], 1)
	assert.JSONEq(t, decisionsTestResponse, w.Body.String())
}

func TestDecisionsHandler_ProviderQueryOverride(t *testing.T) {
	var upstreamBody map[string]any
	var upstreamHeaders http.Header
	server := newDecisionsUpstream(t, &upstreamBody, &upstreamHeaders)
	defer server.Close()

	reqBody := `{"model":"` + decisionsTestModel + `","input":"x","questions":[]}`
	w := serveDecisions(t, newDecisionsTestRouter(t, server.URL), middlewares.DecisionsPath+"?provider="+string(constants.OpenaiID), reqBody)

	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, decisionsTestModel, upstreamBody["model"])
}

func TestDecisionsHandler_Errors(t *testing.T) {
	tests := []struct {
		name           string
		body           string
		expectedStatus int
		expectedMsg    string
	}{
		{
			name:           "Unsupported provider returns 400",
			body:           `{"model":"anthropic/claude-sonnet-4-5","input":"x","questions":[]}`,
			expectedStatus: http.StatusBadRequest,
			expectedMsg:    "The Decisions API is not supported by this provider yet.",
		},
		{
			name:           "Unknown provider prefix returns 400",
			body:           `{"model":"` + decisionsTestModel + `","input":"x","questions":[]}`,
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
			body:           `{"model":"openai/` + decisionsTestModel + `","input":"` + strings.Repeat("a", decisionsTestBodyLimit) + `"}`,
			expectedStatus: http.StatusRequestEntityTooLarge,
			expectedMsg:    "Request body too large",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := serveDecisions(t, newDecisionsTestRouter(t, "http://localhost:0"), middlewares.DecisionsPath, tt.body)

			require.Equal(t, tt.expectedStatus, w.Code)
			var response api.ErrorResponse
			require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
			assert.Contains(t, response.Error, tt.expectedMsg)
		})
	}
}

func TestDecisionsHandler_OutputModalityGate(t *testing.T) {
	tests := []struct {
		name           string
		model          string
		expectedStatus int
		expectedMsg    string
	}{
		{
			name:           "Decision model is forwarded",
			model:          decisionsTestModel,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "Chat model with known modalities is rejected before any upstream call",
			model:          decisionsTestChatModel,
			expectedStatus: http.StatusBadRequest,
			expectedMsg:    "Model openai/" + decisionsTestChatModel + " is not supported by the Decisions API: its output modalities are [text], not decisions.",
		},
		{
			name:           "Model unknown to the table is forwarded so the upstream decides",
			model:          decisionsTestUnknownModel,
			expectedStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var upstreamBody map[string]any
			var upstreamHeaders http.Header
			server := newDecisionsUpstream(t, &upstreamBody, &upstreamHeaders)
			defer server.Close()

			reqBody := `{"model":"openai/` + tt.model + `","input":"x","questions":[]}`
			w := serveDecisions(t, newDecisionsTestRouter(t, server.URL), middlewares.DecisionsPath, reqBody)

			require.Equal(t, tt.expectedStatus, w.Code)
			if tt.expectedMsg == "" {
				assert.Equal(t, tt.model, upstreamBody["model"])
				return
			}
			assert.Nil(t, upstreamBody, "a rejected model must not reach the upstream")
			var response api.ErrorResponse
			require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
			assert.Equal(t, tt.expectedMsg, response.Error)
		})
	}
}

func TestDecisionsHandler_Disabled(t *testing.T) {
	reqBody := `{"model":"openai/` + decisionsTestModel + `","input":"x","questions":[]}`
	w := serveDecisions(t, newDecisionsRouter(t, "http://localhost:0", false), middlewares.DecisionsPath, reqBody)

	require.Equal(t, http.StatusNotFound, w.Code)
	var response api.ErrorResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
	assert.Equal(t, "The Decisions API is not enabled. Set DECISIONS_ENABLED=true to enable it.", response.Error)
}
