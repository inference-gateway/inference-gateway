package tests

import (
	"net/http"
	"net/http/httptest"
	"testing"

	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"

	constants "github.com/inference-gateway/inference-gateway/providers/constants"
)

func TestListModelsHandler_ModalitiesResolution(t *testing.T) {
	mux := http.NewServeMux()
	writeJSON := func(w http.ResponseWriter, payload string) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(payload))
	}

	mux.HandleFunc("/proxy/openai/models", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, `{"object":"list","data":[{"id":"tts-1","object":"model","created":1750000000,"owned_by":"openai"},{"id":"whisper-1","object":"model","created":1750000000,"owned_by":"openai"},{"id":"gpt-5-2025-08-07","object":"model","created":1750000000,"owned_by":"openai"},{"id":"gpt-nonexistent","object":"model","created":1750000000,"owned_by":"openai"}]}`)
	})

	mux.HandleFunc("/proxy/ollama_cloud/models", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, `{"object":"list","data":[{"id":"deepseek-v4-pro:0813","object":"model","created":1750000000,"owned_by":"ollama"}]}`)
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	providerCfg := contextWindowProviderConfig(server.URL, constants.OpenaiID, constants.OllamaCloudID)
	r := newContextWindowRouter(t, server, providerCfg)

	t.Run("include resolves overlay, normalized, and null modalities", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, err := http.NewRequest("GET", "/v1/models?include=modalities", nil)
		require.NoError(t, err)
		r.ServeHTTP(w, req)
		require.Equal(t, http.StatusOK, w.Code)

		models := modelsByID(t, w.Body.Bytes())
		require.Len(t, models, 5)

		tts, ok := models["openai/tts-1"]["modalities"].(map[string]any)
		require.True(t, ok, "overlay-covered speech models must resolve modalities")
		assert.Equal(t, []any{"text"}, tts["input"])
		assert.Equal(t, []any{"audio"}, tts["output"], "text-to-speech models must be audio-out")

		stt, ok := models["openai/whisper-1"]["modalities"].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, []any{"audio"}, stt["input"], "speech-to-text models must be audio-in")
		assert.Equal(t, []any{"text"}, stt["output"])

		dated, ok := models["openai/gpt-5-2025-08-07"]["modalities"].(map[string]any)
		require.True(t, ok, "dashed date-pinned IDs must resolve to their base table entry")
		assert.Contains(t, dated["input"], "text")

		tagged, ok := models["ollama_cloud/deepseek-v4-pro:0813"]["modalities"].(map[string]any)
		require.True(t, ok, "ollama tag-suffixed IDs must resolve to their base table entry")
		assert.Contains(t, tagged["input"], "text")

		unknown, exists := models["openai/gpt-nonexistent"]["modalities"]
		require.True(t, exists, "requested modalities must be present as an explicit key")
		assert.Nil(t, unknown, "models absent from the community table must carry null modalities")
	})

	t.Run("without include no modalities key appears", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, err := http.NewRequest("GET", "/v1/models", nil)
		require.NoError(t, err)
		r.ServeHTTP(w, req)
		require.Equal(t, http.StatusOK, w.Code)

		for id, model := range modelsByID(t, w.Body.Bytes()) {
			_, exists := model["modalities"]
			assert.False(t, exists, "model %q should not carry modalities without include", id)
		}
	})
}
