package tests

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
	gomock "go.uber.org/mock/gomock"

	gin "github.com/gin-gonic/gin"

	mocks "github.com/inference-gateway/inference-gateway/tests/mocks"

	middlewares "github.com/inference-gateway/inference-gateway/api/middlewares"
	config "github.com/inference-gateway/inference-gateway/config"
	logger "github.com/inference-gateway/inference-gateway/logger"
	registry "github.com/inference-gateway/inference-gateway/providers/registry"
	routing "github.com/inference-gateway/inference-gateway/providers/routing"
	transformers "github.com/inference-gateway/inference-gateway/providers/transformers"
)

// TestProviderWiringDrift fails when a provider exists in the generated
// registry but is not wired into routing, the list-models transformer
// factory, or telemetry detection. Adding a provider to openapi.yaml and
// running `task generate` must be sufficient to make this pass.
func TestProviderWiringDrift(t *testing.T) {
	for id := range registry.Registry {
		t.Run(string(id), func(t *testing.T) {
			t.Run("routing resolves prefix", func(t *testing.T) {
				provider, model := routing.DetermineProviderAndModelName(string(id) + "/some-model")
				require.NotNil(t, provider)
				assert.Equal(t, id, *provider)
				assert.Equal(t, "some-model", model)
			})

			t.Run("list models transformer stamps the provider", func(t *testing.T) {
				transformer := transformers.NewListModelsTransformer(id)
				require.NoError(t, json.Unmarshal([]byte(`{"object":"list","data":[{"id":"m1"}]}`), transformer))

				result := transformer.Transform()
				require.NotNil(t, result.Provider)
				assert.Equal(t, id, *result.Provider)
				require.Len(t, result.Data, 1)
				assert.Equal(t, string(id)+"/m1", result.Data[0].ID)
				assert.Equal(t, id, result.Data[0].ServedBy)
			})

			t.Run("telemetry detects provider from model prefix", func(t *testing.T) {
				assertTelemetryDetects(t, string(id), fmt.Sprintf(`{"model":%q}`, string(id)+"/some-model"), "/v1/chat/completions")
			})

			t.Run("telemetry detects provider from query parameter", func(t *testing.T) {
				assertTelemetryDetects(t, string(id), `{"model":"some-model"}`, "/v1/chat/completions?provider="+string(id))
			})
		})
	}
}

func assertTelemetryDetects(t *testing.T, expectedProvider, requestBody, url string) {
	t.Helper()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockOtel := mocks.NewMockOpenTelemetry(ctrl)
	mockOtel.EXPECT().
		RecordRequestDuration(gomock.Any(), gomock.Any(), gomock.Any(), expectedProvider, gomock.Any(), gomock.Any(), gomock.Any()).
		Times(1)
	mockOtel.EXPECT().
		RecordTokenUsage(gomock.Any(), gomock.Any(), gomock.Any(), expectedProvider, gomock.Any(), gomock.Any(), gomock.Any()).
		AnyTimes()
	mockOtel.EXPECT().
		RecordToolCall(gomock.Any(), gomock.Any(), gomock.Any(), expectedProvider, gomock.Any(), gomock.Any(), gomock.Any()).
		AnyTimes()

	telemetry, err := middlewares.NewTelemetryMiddleware(config.Config{}, mockOtel, logger.NewNoopLogger())
	require.NoError(t, err)

	router := gin.New()
	router.Use(telemetry.Middleware())
	router.POST("/v1/chat/completions", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{})
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", url, bytes.NewReader([]byte(requestBody)))
	req.Header.Set("Content-Type", "application/json")

	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}
