package tests

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"

	api "github.com/inference-gateway/inference-gateway/api"
	config "github.com/inference-gateway/inference-gateway/config"
	logger "github.com/inference-gateway/inference-gateway/internal/platform/logger"
	constants "github.com/inference-gateway/inference-gateway/providers/constants"
	registry "github.com/inference-gateway/inference-gateway/providers/registry"
	types "github.com/inference-gateway/inference-gateway/providers/types"
)

const (
	minimaxTestFlash = "MiniMax-M3.1-Flash"
	minimaxTestM3    = "MiniMax-M3"
	minimaxTestToken = "fixture-key"
)

type minimaxTestClient func(*http.Request) (*http.Response, error)

func (c minimaxTestClient) Do(req *http.Request) (*http.Response, error) {
	return c(req)
}

func newMinimaxTestGateway(t *testing.T, upstream, allowedModels string) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	gateway := httptest.NewServer(mux)
	t.Cleanup(gateway.Close)
	base, err := url.Parse(gateway.URL)
	require.NoError(t, err)
	client := minimaxTestClient(func(req *http.Request) (*http.Response, error) {
		forward := req.Clone(req.Context())
		if forward.URL.Host == "" {
			forward.URL.Scheme = base.Scheme
			forward.URL.Host = base.Host
		}
		return http.DefaultClient.Do(forward)
	})
	log, err := logger.NewLogger("test")
	require.NoError(t, err)
	providers := contextWindowProviderConfig(upstream, constants.MinimaxID)
	providers[constants.MinimaxID].Token = minimaxTestToken
	providers[constants.MinimaxID].AuthType = constants.AuthTypeBearer
	providers[constants.MinimaxID].Endpoints.Chat = "/chat/completions"
	cfg := config.Config{
		Server:        &config.ServerConfig{ReadTimeout: 5 * time.Second, WriteTimeout: 5 * time.Second},
		Providers:     providers,
		VisionEnabled: true,
		AllowedModels: allowedModels,
	}
	reg := registry.NewProviderRegistry(providers, log)
	router := api.NewRouter(cfg, log, reg, client, nil, nil, nil, nil, nil)
	mux.HandleFunc("GET /v1/models", router.ListModelsHandler)
	mux.HandleFunc("POST /v1/chat/completions", router.ChatCompletionsHandler)
	mux.HandleFunc("/proxy/{provider}/{path...}", router.ProxyHandler)
	return gateway
}

func TestMinimaxCatalogAndChatHTTP(t *testing.T) {
	captured := make(chan map[string]any, 1)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "Bearer "+minimaxTestToken, r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodGet {
			assert.Equal(t, "/v1/models", r.URL.Path)
			_, _ = io.WriteString(w, `{"object":"list","data":[{"id":"MiniMax-M2.7"},{"id":"speech-2.8-hd"},{"id":"MiniMax-M3"},{"id":"custom-model"},{"id":"MiniMax-M3.1-Flash"},{"id":"MiniMax-M2.1-highspeed"}]}`)
			return
		}
		assert.Equal(t, "/v1/chat/completions", r.URL.Path)
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); !assert.NoError(t, err) {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		captured <- body
		if thinking, ok := body["thinking"].(map[string]any); ok && body["model"] == minimaxTestFlash && thinking["type"] == "disabled" {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = io.WriteString(w, `{"error":{"message":"Flash requires adaptive thinking","type":"invalid_request_error"}}`)
			return
		}
		if body["stream"] == true {
			w.Header().Set("Content-Type", "text/event-stream")
			_, _ = io.WriteString(w, "data: {\"choices\":[{\"delta\":{\"content\":\"ok\"}}]}\n\ndata: [DONE]\n\n")
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"id": "fixture", "model": body["model"], "choices": []any{}})
	}))
	defer upstream.Close()
	gateway := newMinimaxTestGateway(t, upstream.URL+"/v1", "")

	for _, query := range []string{"", "?provider=minimax&include=context_window,pricing,modalities"} {
		response, err := http.Get(gateway.URL + "/v1/models" + query)
		require.NoError(t, err)
		var catalog types.ListModelsResponse
		require.NoError(t, json.NewDecoder(response.Body).Decode(&catalog))
		require.NoError(t, response.Body.Close())
		require.Equal(t, http.StatusOK, response.StatusCode)
		ids := make([]string, 0, len(catalog.Data))
		for _, model := range catalog.Data {
			ids = append(ids, model.ID)
		}
		require.Equal(t, []string{"minimax/" + minimaxTestFlash, "minimax/" + minimaxTestM3, "minimax/speech-2.8-hd", "minimax/custom-model"}, ids)
		if query != "" {
			require.NotNil(t, catalog.Data[0].ContextWindow)
			assert.Equal(t, 1000000, catalog.Data[0].ContextWindow.Tokens)
			require.NotNil(t, catalog.Data[0].Modalities)
			assert.Contains(t, catalog.Data[0].Modalities.Input, types.ModalityImage)
		}
	}

	explicitGateway := newMinimaxTestGateway(t, upstream.URL+"/v1", "minimax/MiniMax-M2.7")
	for _, query := range []string{"", "?provider=minimax"} {
		response, err := http.Get(explicitGateway.URL + "/v1/models" + query)
		require.NoError(t, err)
		var catalog types.ListModelsResponse
		require.NoError(t, json.NewDecoder(response.Body).Decode(&catalog))
		require.NoError(t, response.Body.Close())
		require.Equal(t, http.StatusOK, response.StatusCode)
		require.Len(t, catalog.Data, 1)
		assert.Equal(t, "minimax/MiniMax-M2.7", catalog.Data[0].ID)
	}

	rawCatalog, err := http.Get(gateway.URL + "/proxy/minimax/models")
	require.NoError(t, err)
	var original types.ListModelsResponse
	require.NoError(t, json.NewDecoder(rawCatalog.Body).Decode(&original))
	require.NoError(t, rawCatalog.Body.Close())
	require.Equal(t, http.StatusOK, rawCatalog.StatusCode)
	require.Len(t, original.Data, 6)
	assert.Equal(t, "MiniMax-M2.7", original.Data[0].ID)

	for _, model := range []string{minimaxTestFlash, minimaxTestM3, "MiniMax-M2.7", "custom-model"} {
		for _, stream := range []bool{false, true} {
			t.Run(model+"/"+map[bool]string{false: "complete", true: "stream"}[stream], func(t *testing.T) {
				body := map[string]any{
					"model": "minimax/" + model,
					"messages": []any{
						map[string]any{"role": "assistant", "content": "", "reasoning_content": "Inspect the scene", "tool_calls": []any{map[string]any{"id": "call_1", "type": "function", "function": map[string]any{"name": "inspect", "arguments": "{}"}}}},
						map[string]any{"role": "tool", "tool_call_id": "call_1", "content": "A scene"},
						map[string]any{"role": "user", "content": []any{
							map[string]any{"type": "text", "text": "Describe the scene"},
							map[string]any{"type": "image_url", "image_url": map[string]any{"url": "https://example.com/image.png"}},
							map[string]any{"type": "video_url", "video_url": map[string]any{"url": "mm_file://fixture"}},
						}},
					},
					"stream": stream,
				}
				if model != minimaxTestFlash && model != minimaxTestM3 {
					body["messages"] = []any{map[string]any{"role": "user", "content": "Saved selection"}}
				}
				raw, err := json.Marshal(body)
				require.NoError(t, err)
				response, err := http.Post(gateway.URL+"/v1/chat/completions", "application/json", bytes.NewReader(raw))
				require.NoError(t, err)
				_, err = io.Copy(io.Discard, response.Body)
				require.NoError(t, err)
				require.NoError(t, response.Body.Close())
				require.Equal(t, http.StatusOK, response.StatusCode)
				outgoing := <-captured
				assert.Equal(t, model, outgoing["model"])
				assert.Equal(t, body["messages"], outgoing["messages"])
				assert.NotContains(t, outgoing, "thinking")
				assert.NotContains(t, outgoing, "reasoning_effort")
				assert.NotContains(t, outgoing, "service_tier")
				assert.NotContains(t, outgoing, "reasoning_split")
			})
		}
	}

	for _, effort := range []string{"low", "medium", "high", "xhigh", "max"} {
		body := map[string]any{"model": "minimax/" + minimaxTestFlash, "reasoning_effort": effort, "messages": []any{map[string]any{"role": "user", "content": "Hi"}}}
		raw, err := json.Marshal(body)
		require.NoError(t, err)
		response, err := http.Post(gateway.URL+"/v1/chat/completions", "application/json", bytes.NewReader(raw))
		require.NoError(t, err)
		require.NoError(t, response.Body.Close())
		require.Equal(t, http.StatusOK, response.StatusCode)
		assert.Equal(t, effort, (<-captured)["reasoning_effort"])
	}

	// Provider-specific controls use the existing raw proxy API: the generated
	// Chat Completions schema does not expose the MiniMax thinking field.
	for _, control := range []struct {
		model    string
		thinking string
		status   int
	}{
		{minimaxTestFlash, "adaptive", http.StatusOK},
		{minimaxTestM3, "adaptive", http.StatusOK},
		{minimaxTestM3, "disabled", http.StatusOK},
		{minimaxTestFlash, "disabled", http.StatusBadRequest},
	} {
		body := map[string]any{"model": control.model, "thinking": map[string]any{"type": control.thinking}, "messages": []any{map[string]any{"role": "user", "content": "Hi"}}}
		if control.model == minimaxTestFlash && control.thinking == "adaptive" {
			body["reasoning_effort"] = "max"
		}
		raw, err := json.Marshal(body)
		require.NoError(t, err)
		response, err := http.Post(gateway.URL+"/proxy/minimax/chat/completions", "application/json", bytes.NewReader(raw))
		require.NoError(t, err)
		require.NoError(t, response.Body.Close())
		require.Equal(t, control.status, response.StatusCode)
		assert.Equal(t, body, <-captured)
	}
}
