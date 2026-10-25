package tests

import (
	"net/http"
	"net/http/httptest"
	"testing"

	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"

	constants "github.com/inference-gateway/inference-gateway/providers/constants"
)

// TestListModelsHandler_CloudflareNativeEnvelope pins Cloudflare's model
// listing. GET .../ai/finetunes/public answers Cloudflare's v4 envelope
// {"result":[...],"success":true} - not the OpenAI {"object":"list","data":[...]}
// envelope the generated transformer assumes - and `name` is the public
// fine-tune's id (https://developers.cloudflare.com/api/resources/ai/subresources/finetunes/subresources/public/methods/list/).
// The listing must decode that envelope instead of coming back empty.
func TestListModelsHandler_CloudflareNativeEnvelope(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/proxy/cloudflare/finetunes/public", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success":true,"errors":[],"messages":[],"result":[{"id":"11111111-1111-1111-1111-111111111111","created_at":"2025-02-19T00:00:00Z","model":"@cf/mistral/mistral-7b-instruct-v0.1","modified_at":"2025-02-19T00:00:00Z","name":"cf-public-magicoder","public":true},{"id":"22222222-2222-2222-2222-222222222222","created_at":"2025-02-19T00:00:00Z","model":"@cf/mistral/mistral-7b-instruct-v0.1","modified_at":"2025-02-19T00:00:00Z","name":"cf-public-cnn-summarization","public":true}]}`))
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	providerCfg := contextWindowProviderConfig(server.URL, constants.CloudflareID)
	providerCfg[constants.CloudflareID].Endpoints.Models = constants.CloudflareModelsEndpoint
	r := newContextWindowRouter(t, server, providerCfg)

	w := httptest.NewRecorder()
	req, err := http.NewRequest("GET", "/v1/models?provider=cloudflare", nil)
	require.NoError(t, err)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	models := modelsByID(t, w.Body.Bytes())
	require.Len(t, models, 2, "Cloudflare's models listing must not come back empty")
	for _, id := range []string{"cloudflare/cf-public-magicoder", "cloudflare/cf-public-cnn-summarization"} {
		model, ok := models[id]
		require.True(t, ok, "expected %s in the listing", id)
		assert.Equal(t, "model", model["object"])
		assert.Equal(t, "cloudflare", model["owned_by"])
		assert.Equal(t, "cloudflare", model["served_by"])
		assert.Equal(t, float64(1739923200), model["created"])
	}
}
