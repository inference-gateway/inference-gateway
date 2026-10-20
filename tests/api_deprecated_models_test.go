package tests

import (
	"net/http"
	"net/http/httptest"
	"testing"

	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"

	constants "github.com/inference-gateway/inference-gateway/providers/constants"
)

func TestListModelsHandler_DeprecatedModels(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/proxy/minimax/models", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"object":"list","data":[{"id":"MiniMax-M2","object":"model","created":1750000000,"owned_by":"minimax"},{"id":"MiniMax-M3","object":"model","created":1750000000,"owned_by":"minimax"}]}`))
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	providerCfg := contextWindowProviderConfig(server.URL, constants.MinimaxID)
	r := newContextWindowRouter(t, server, providerCfg)

	listModels := func(t *testing.T, path string) map[string]map[string]any {
		w := httptest.NewRecorder()
		req, err := http.NewRequest("GET", path, nil)
		require.NoError(t, err)
		r.ServeHTTP(w, req)
		require.Equal(t, http.StatusOK, w.Code)
		return modelsByID(t, w.Body.Bytes())
	}

	for name, query := range map[string]string{"all providers": "", "single provider": "provider=minimax&"} {
		t.Run(name+" hides deprecated by default", func(t *testing.T) {
			models := listModels(t, "/v1/models?"+query)
			assert.Contains(t, models, "minimax/MiniMax-M3")
			assert.NotContains(t, models, "minimax/MiniMax-M2")
		})

		t.Run(name+" include=deprecated lists them without adding a field", func(t *testing.T) {
			models := listModels(t, "/v1/models?"+query+"include=deprecated,pricing")
			require.Contains(t, models, "minimax/MiniMax-M2")
			_, hasKey := models["minimax/MiniMax-M2"]["deprecated"]
			assert.False(t, hasKey, "deprecated is a row selector, not a model field")
			_, hasPricing := models["minimax/MiniMax-M2"]["pricing"]
			assert.True(t, hasPricing, "other include keys keep injecting their field")
		})
	}
}
