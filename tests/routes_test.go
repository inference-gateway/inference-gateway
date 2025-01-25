package tests

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/inference-gateway/inference-gateway/api"
	"github.com/inference-gateway/inference-gateway/config"
	"github.com/inference-gateway/inference-gateway/logger"
	"github.com/inference-gateway/inference-gateway/providers"
	"github.com/inference-gateway/inference-gateway/tests/mocks"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

func setupTestRouter(t *testing.T) (*gin.Engine, *mocks.MockLogger) {
	gin.SetMode(gin.TestMode)
	ctrl := gomock.NewController(t)
	mockLogger := mocks.NewMockLogger(ctrl)

	cfg := config.Config{
		ApplicationName: "inference-gateway-test",
		Environment:     "test",
	}

	// Create HTTP client with reasonable timeout
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	// Pass mockLogger as logger.Logger interface
	var l logger.Logger = mockLogger
	router := api.NewRouter(cfg, &l, client)
	r := gin.New()
	r.GET("/health", router.HealthcheckHandler)
	r.GET("/llms", router.FetchAllModelsHandler)
	r.POST("/llms/:provider/generate", router.GenerateProvidersTokenHandler)

	return r, mockLogger
}

func TestHealthcheckHandler(t *testing.T) {
	r, mockLogger := setupTestRouter(t)
	mockLogger.EXPECT().Debug("healthcheck")

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var response map[string]string
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, "OK", response["message"])
}

func TestFetchAllModelsHandler(t *testing.T) {
	r, mockLogger := setupTestRouter(t)
	mockLogger.EXPECT().Debug(gomock.Any()).AnyTimes()

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/llms", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var response []providers.ModelsResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
}

func TestGenerateProvidersTokenHandler(t *testing.T) {
	tests := []struct {
		name           string
		provider       string
		requestBody    map[string]interface{}
		expectedStatus int
		setupMocks     func(*mocks.MockLogger)
	}{
		{
			name:     "Invalid Provider",
			provider: "invalid",
			requestBody: map[string]interface{}{
				"model": "test-model",
				"messages": []map[string]string{
					{"role": "user", "content": "test"},
				},
			},
			expectedStatus: http.StatusBadRequest,
			setupMocks: func(ml *mocks.MockLogger) {
				ml.EXPECT().Error(gomock.Any(), gomock.Any(), gomock.Any()).Times(1)
			},
		},
		{
			name:     "Missing Model",
			provider: "groq",
			requestBody: map[string]interface{}{
				"messages": []map[string]string{
					{"role": "user", "content": "test"},
				},
			},
			expectedStatus: http.StatusBadRequest,
			setupMocks: func(ml *mocks.MockLogger) {
				ml.EXPECT().Error(gomock.Any(), gomock.Any(), gomock.Any()).Times(1)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, mockLogger := setupTestRouter(t)
			tt.setupMocks(mockLogger)

			body, err := json.Marshal(tt.requestBody)
			assert.NoError(t, err)

			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "/llms/"+tt.provider+"/generate", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			r.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
		})
	}
}

func TestProxyHandler_UnreachableHost(t *testing.T) {
	// Setup
	r, mockLogger := setupTestRouter(t)
	mockLogger.EXPECT().Error("proxy request failed", gomock.Any())

	// Configure test router with unreachable host
	cfg := config.Config{
		ApplicationName: "inference-gateway-test",
		Environment:     "test",
		Providers: map[string]*config.BaseProviderConfig{
			providers.OllamaID: {
				ID:       providers.OllamaID,
				Name:     "Ollama",
				URL:      "http://ollama:8080",
				Token:    "",
				AuthType: "none",
				Endpoints: struct {
					List     string
					Generate string
				}{
					List:     "/v1/models",
					Generate: "/v1/generate",
				},
			},
		},
	}

	var l logger.Logger = mockLogger
	router := api.NewRouter(cfg, &l, &http.Client{
		Timeout: 1 * time.Second,
	})

	r.Any("/proxy/:provider/*proxyPath", router.ProxyHandler)

	// Execute
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/proxy/ollama/v1/models", nil)
	r.ServeHTTP(w, req)

	// Assert
	assert.Equal(t, http.StatusBadGateway, w.Code)

	var response api.ErrorResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Contains(t, response.Error, "Failed to reach upstream server")
}
