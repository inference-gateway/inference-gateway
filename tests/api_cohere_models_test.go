package tests

import (
	"net/http"
	"net/http/httptest"
	"testing"

	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"

	constants "github.com/inference-gateway/inference-gateway/providers/constants"
)

// TestListModelsHandler_CohereNativeEnvelope pins Cohere's model listing.
// GET https://api.cohere.ai/v1/models answers {"models":[{"name":...}]} - not
// the OpenAI {"object":"list","data":[...]} envelope the generated transformer
// assumes - and `name` is the id a chat request must send
// (https://docs.cohere.com/reference/list-models). The transformer must convert
// that envelope and keep each entry's context_length reachable through
// include=context_window.
func TestListModelsHandler_CohereNativeEnvelope(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/proxy/cohere/models", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"models":[{"name":"command-r-plus","is_deprecated":false,"endpoints":["chat"],"context_length":128000},{"name":"command-r","is_deprecated":false,"endpoints":["chat"],"context_length":128000}],"next_page_token":""}`))
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	providerCfg := contextWindowProviderConfig(server.URL, constants.CohereID)
	r := newContextWindowRouter(t, server, providerCfg)

	w := httptest.NewRecorder()
	req, err := http.NewRequest("GET", "/v1/models?provider=cohere&include=context_window", nil)
	require.NoError(t, err)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	models := modelsByID(t, w.Body.Bytes())
	require.Len(t, models, 2, "Cohere's models listing must not come back empty")
	for _, id := range []string{"cohere/command-r-plus", "cohere/command-r"} {
		model, ok := models[id]
		require.True(t, ok, "expected %s in the listing", id)
		assert.Equal(t, "model", model["object"])
		assert.Equal(t, "cohere", model["owned_by"])
		assert.Equal(t, "cohere", model["served_by"])
		assert.Equal(t, map[string]any{"tokens": float64(128000), "source": "provider"}, model["context_window"])
	}
}
