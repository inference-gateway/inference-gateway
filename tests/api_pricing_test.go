package tests

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"

	constants "github.com/inference-gateway/inference-gateway/providers/constants"
)

func TestListModelsHandler_PricingResolution(t *testing.T) {
	mux := http.NewServeMux()
	writeJSON := func(w http.ResponseWriter, payload string) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(payload))
	}

	mux.HandleFunc("/proxy/deepseek/models", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, `{"object":"list","data":[{"id":"deepseek-chat","object":"model","created":1750000000,"owned_by":"deepseek","pricing":{"prompt":"0.00000027","completion":"0.00000110","input_cache_read":"0.00000007","input_cache_write":"0.00000027"}}]}`)
	})

	mux.HandleFunc("/proxy/groq/models", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, `{"object":"list","data":[{"id":"llama-3.3-70b","object":"model","created":1750000000,"owned_by":"meta","pricing":{"prompt":0.00000059,"completion":"0.00000079","input_cache_read":"0","input_cache_write":0}}]}`)
	})

	mux.HandleFunc("/proxy/openai/models", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, `{"object":"list","data":[{"id":"gpt-4","object":"model","created":1750000000,"owned_by":"openai"},{"id":"gpt-nonexistent","object":"model","created":1750000000,"owned_by":"openai"}]}`)
	})

	mux.HandleFunc("/proxy/anthropic/models", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, `{"data":[{"id":"claude-sonnet-4-5-20250929","object":"model","created":1750000000,"owned_by":"anthropic"}]}`)
	})

	mux.HandleFunc("/proxy/nvidia/models", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, `{"object":"list","data":[{"id":"meta/llama-3.1-8b-instruct","object":"model","created":1750000000,"owned_by":"meta"}]}`)
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	providerCfg := contextWindowProviderConfig(server.URL,
		constants.DeepseekID, constants.GroqID, constants.OpenaiID, constants.AnthropicID, constants.NvidiaID)
	r := newContextWindowRouter(t, server, providerCfg)

	t.Run("include resolves provider, community, and null pricing", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, err := http.NewRequest("GET", "/v1/models?include=pricing", nil)
		require.NoError(t, err)
		r.ServeHTTP(w, req)
		require.Equal(t, http.StatusOK, w.Code)

		models := modelsByID(t, w.Body.Bytes())
		require.Len(t, models, 6)

		full, exists := models["deepseek/deepseek-chat"]["pricing"]
		require.True(t, exists)
		assert.Equal(t, map[string]any{
			"currency":              "USD",
			"input_per_token":       "0.00000027",
			"output_per_token":      "0.00000110",
			"cache_read_per_token":  "0.00000007",
			"cache_write_per_token": "0.00000027",
			"source":                "provider",
		}, full, "provider-published pricing must win over the community table")

		partial, ok := models["groq/llama-3.3-70b"]["pricing"].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "provider", partial["source"], "partial provider pricing must not fall back to community rates")
		assert.Equal(t, "0.00000059", partial["input_per_token"], "numeric rates become decimal strings")
		assert.Equal(t, "0.00000079", partial["output_per_token"])
		assert.NotContains(t, partial, "cache_read_per_token", "zero cache rates must be omitted, not rendered")
		assert.NotContains(t, partial, "cache_write_per_token")

		community, ok := models["openai/gpt-4"]["pricing"].(map[string]any)
		require.True(t, ok, "models without provider pricing must fall back to the community table")
		assert.Equal(t, "community", community["source"])
		assert.Equal(t, "USD", community["currency"])
		assert.NotEmpty(t, community["input_per_token"])
		assert.NotEmpty(t, community["output_per_token"])

		unpriced, exists := models["openai/gpt-nonexistent"]["pricing"]
		require.True(t, exists, "requested pricing must be present as an explicit key")
		assert.Nil(t, unpriced, "models absent from the community table must carry a null pricing")

		dated, ok := models["anthropic/claude-sonnet-4-5-20250929"]["pricing"].(map[string]any)
		require.True(t, ok, "date-pinned model IDs must resolve in the community table")
		assert.Equal(t, "community", dated["source"])

		free, ok := models["nvidia/meta/llama-3.1-8b-instruct"]["pricing"].(map[string]any)
		require.True(t, ok, "free-tier models must resolve in the community table")
		assert.Equal(t, "community", free["source"])
		assert.Equal(t, "0", free["input_per_token"], `free-tier models carry explicit "0" rates, not null pricing`)
		assert.Equal(t, "0", free["output_per_token"])
	})

	t.Run("anthropic listing defaults object to list", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, err := http.NewRequest("GET", "/v1/models?provider=anthropic", nil)
		require.NoError(t, err)
		r.ServeHTTP(w, req)
		require.Equal(t, http.StatusOK, w.Code)

		var response map[string]any
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
		assert.Equal(t, "list", response["object"], "missing upstream object must default to list")
	})

	t.Run("without include no pricing key appears", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, err := http.NewRequest("GET", "/v1/models", nil)
		require.NoError(t, err)
		r.ServeHTTP(w, req)
		require.Equal(t, http.StatusOK, w.Code)

		for id, model := range modelsByID(t, w.Body.Bytes()) {
			_, exists := model["pricing"]
			assert.False(t, exists, "model %q should not carry pricing without include", id)
		}
	})
}
