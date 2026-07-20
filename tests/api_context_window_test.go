package tests

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
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

// newContextWindowRouter builds a models router whose mock client forwards
// every request to the given test server: self-proxy calls (relative URLs) by
// path, runtime lookups (absolute URLs pointing at the server) as-is.
func newContextWindowRouter(t testing.TB, server *httptest.Server, providerCfg map[types.Provider]*registry.ProviderConfig) *gin.Engine {
	t.Helper()

	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)
	mockClient := providersmocks.NewMockClient(ctrl)
	mockClient.EXPECT().
		Do(gomock.Any()).
		DoAndReturn(func(req *http.Request) (*http.Response, error) {
			if req.URL.Host != "" {
				return http.DefaultClient.Do(req)
			}
			fwd, err := http.NewRequest(req.Method, server.URL+req.URL.Path, req.Body)
			if err != nil {
				return nil, err
			}
			return http.DefaultClient.Do(fwd)
		}).
		AnyTimes()

	log, err := logger.NewLogger("test")
	require.NoError(t, err)

	reg := registry.NewProviderRegistry(providerCfg, log)
	cfg := config.Config{
		Server: &config.ServerConfig{
			ReadTimeout: 5 * time.Second,
		},
		Providers: providerCfg,
	}
	router := api.NewRouter(cfg, log, reg, mockClient, nil, nil)

	r := gin.New()
	r.GET("/v1/models", router.ListModelsHandler)
	return r
}

func contextWindowProviderConfig(serverURL string, providers ...types.Provider) map[types.Provider]*registry.ProviderConfig {
	cfg := make(map[types.Provider]*registry.ProviderConfig, len(providers))
	for _, id := range providers {
		authType := constants.AuthTypeBearer
		token := "test-token"
		if id == constants.OllamaID {
			authType = constants.AuthTypeNone
			token = ""
		}
		cfg[id] = &registry.ProviderConfig{
			ID:       id,
			Name:     string(id),
			URL:      serverURL,
			Token:    token,
			AuthType: authType,
			Endpoints: types.Endpoints{
				Models: "/models",
			},
		}
	}
	return cfg
}

func modelsByID(t *testing.T, body []byte) map[string]map[string]any {
	t.Helper()

	var response map[string]any
	require.NoError(t, json.Unmarshal(body, &response))
	data, ok := response["data"].([]any)
	require.True(t, ok, "response must contain a data array")

	models := make(map[string]map[string]any, len(data))
	for _, item := range data {
		model, ok := item.(map[string]any)
		require.True(t, ok)
		models[model["id"].(string)] = model
	}
	return models
}

func TestListModelsHandler_ContextWindowResolution(t *testing.T) {
	mux := http.NewServeMux()
	writeJSON := func(w http.ResponseWriter, payload string) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(payload))
	}

	mux.HandleFunc("/proxy/llamacpp/models", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, `{"object":"list","data":[{"id":"qwen3-coder","object":"model","created":1750000000,"owned_by":"qwen","max_context_length":131072}]}`)
	})
	mux.HandleFunc("/props", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, `{"default_generation_settings":{"n_ctx":32768}}`)
	})

	mux.HandleFunc("/proxy/ollama/models", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, `{"object":"list","data":[{"id":"llama3","object":"model","created":1750000000,"owned_by":"meta"}]}`)
	})
	mux.HandleFunc("/api/show", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, `{"parameters":"num_ctx 8192\nstop \"<|end|>\"","model_info":{"llama.context_length":131072}}`)
	})

	mux.HandleFunc("/proxy/mistral/models", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, `{"object":"list","data":[{"id":"mistral-large","object":"model","created":1750000000,"owned_by":"mistralai","max_context_length":32768}]}`)
	})

	mux.HandleFunc("/proxy/openai/models", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, `{"object":"list","data":[{"id":"gpt-4","object":"model","created":1750000000,"owned_by":"openai"},{"id":"gpt-nonexistent","object":"model","created":1750000000,"owned_by":"openai"}]}`)
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	providerCfg := contextWindowProviderConfig(server.URL,
		constants.LlamacppID, constants.OllamaID, constants.MistralID, constants.OpenaiID)
	r := newContextWindowRouter(t, server, providerCfg)

	t.Run("include resolves runtime, provider, community, and null windows", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, err := http.NewRequest("GET", "/v1/models?include=context_window", nil)
		require.NoError(t, err)
		r.ServeHTTP(w, req)
		require.Equal(t, http.StatusOK, w.Code)

		models := modelsByID(t, w.Body.Bytes())
		require.Len(t, models, 5)

		expected := map[string]map[string]any{
			"llamacpp/qwen3-coder":   {"tokens": float64(32768), "source": "runtime"},
			"ollama/llama3":          {"tokens": float64(8192), "source": "runtime"},
			"mistral/mistral-large":  {"tokens": float64(32768), "source": "provider"},
			"openai/gpt-4":           {"tokens": float64(8192), "source": "community"},
			"openai/gpt-nonexistent": nil,
		}
		for id, want := range expected {
			model, ok := models[id]
			require.True(t, ok, "model %q missing from response", id)
			got, exists := model["context_window"]
			require.True(t, exists, "model %q should carry a context_window key", id)
			if want == nil {
				assert.Nil(t, got, "model %q should have a null context_window", id)
				continue
			}
			window, ok := got.(map[string]any)
			require.True(t, ok, "model %q context_window should be an object", id)
			assert.Equal(t, want["tokens"], window["tokens"], "model %q tokens", id)
			assert.Equal(t, want["source"], window["source"], "model %q source", id)
		}
	})

	t.Run("single provider request resolves the runtime window", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, err := http.NewRequest("GET", "/v1/models?provider=llamacpp&include=context_window", nil)
		require.NoError(t, err)
		r.ServeHTTP(w, req)
		require.Equal(t, http.StatusOK, w.Code)

		models := modelsByID(t, w.Body.Bytes())
		window, ok := models["llamacpp/qwen3-coder"]["context_window"].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, float64(32768), window["tokens"])
		assert.Equal(t, "runtime", window["source"])
	})

	t.Run("without include no context_window key appears", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, err := http.NewRequest("GET", "/v1/models", nil)
		require.NoError(t, err)
		r.ServeHTTP(w, req)
		require.Equal(t, http.StatusOK, w.Code)

		for id, model := range modelsByID(t, w.Body.Bytes()) {
			_, exists := model["context_window"]
			assert.False(t, exists, "model %q should not carry context_window without include", id)
		}
	})
}

func TestListModelsHandler_ContextWindowLookupFailure(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/proxy/llamacpp/models", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"object":"list","data":[{"id":"qwen3-coder","object":"model","created":1750000000,"owned_by":"qwen"}]}`))
	})
	mux.HandleFunc("/props", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	providerCfg := contextWindowProviderConfig(server.URL, constants.LlamacppID)
	r := newContextWindowRouter(t, server, providerCfg)

	w := httptest.NewRecorder()
	req, err := http.NewRequest("GET", "/v1/models?include=context_window", nil)
	require.NoError(t, err)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code, "a failing runtime lookup must not fail the request")

	models := modelsByID(t, w.Body.Bytes())
	model, ok := models["llamacpp/qwen3-coder"]
	require.True(t, ok)
	val, exists := model["context_window"]
	require.True(t, exists, "context_window key should be present")
	assert.Nil(t, val, "unresolved window should be an explicit null")
}
