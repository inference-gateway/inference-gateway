package tests

import (
	"bytes"
	"encoding/json"
	"fmt"
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
	timeout := 1 * time.Second
	transport := providers.NewTransport(timeout)
	client := providers.NewClient("http", "localhost", "8080", timeout, transport)

	// Pass mockLogger as logger.Logger interface
	var l logger.Logger = mockLogger
	router := api.NewRouter(cfg, &l, client)
	r := gin.New()
	r.GET("/health", router.HealthcheckHandler)
	r.GET("/llms", router.ListAllModelsHandler)
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
	var response []providers.ListModelsResponse
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

	// Create mock controller
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Create mock client
	mockClient := mocks.NewMockClient(ctrl)
	mockClient.EXPECT().
		Get(gomock.Any()).
		Return(nil, fmt.Errorf("connection refused")).
		AnyTimes()

	// Setup logger expectation
	mockLogger.EXPECT().Error("proxy request failed", gomock.Any()).Times(1)

	// Configure test router with mock client
	cfg := config.Config{
		ApplicationName: "inference-gateway-test",
		Environment:     "test",
		Providers: map[string]*providers.Config{
			providers.OllamaID: {
				ID:       providers.OllamaID,
				Name:     "Ollama",
				URL:      "http://ollama:8080",
				Token:    "",
				AuthType: providers.AuthTypeNone,
				Endpoints: providers.Endpoints{
					List:     "/v1/models",
					Generate: "/v1/generate",
				},
			},
		},
	}

	var l logger.Logger = mockLogger
	router := api.NewRouter(cfg, &l, mockClient)

	r.Any("/proxy/:provider/*proxyPath", router.ProxyHandler)

	// Create custom response writer
	w := &customResponseWriter{
		ResponseRecorder: httptest.NewRecorder(),
	}

	// Execute request
	req := httptest.NewRequest(http.MethodGet, "/proxy/ollama/v1/models", nil)
	r.ServeHTTP(w, req)

	// Assert response
	assert.Equal(t, http.StatusBadGateway, w.Code)

	var response api.ErrorResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Contains(t, response.Error, "Failed to reach upstream server")
}

var providerFactory = providers.NewProvider

func TestProxyHandler_TokenValidation(t *testing.T) {
	tests := []struct {
		name           string
		providerID     string
		authType       string
		token          string
		expectedStatus int
		expectedError  string
		setupMocks     func(*mocks.MockLogger, *mocks.MockProvider)
	}{
		{
			name:           "Missing Required Token",
			providerID:     providers.GroqID,
			authType:       providers.AuthTypeBearer,
			token:          "",
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Provider requires an API key. Please configure the provider's API key.",
			setupMocks: func(ml *mocks.MockLogger, mp *mocks.MockProvider) {
				ml.EXPECT().Error("provider requires authentication but no API key was configured",
					gomock.Any(),
					"provider",
					providers.GroqID)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			r, mockLogger := setupTestRouter(t)
			mockProvider := mocks.NewMockProvider(ctrl)

			originalFactory := providerFactory
			providerFactory = func(cfg map[string]*providers.Config, id string, logger *logger.Logger, client *providers.Client) (providers.Provider, error) {
				return mockProvider, nil
			}
			defer func() { providerFactory = originalFactory }()

			tt.setupMocks(mockLogger, mockProvider)

			cfg := config.Config{
				ApplicationName: "inference-gateway-test",
				Environment:     "test",
				Providers: map[string]*providers.Config{
					tt.providerID: {
						ID: tt.providerID,
					},
				},
			}

			var l logger.Logger = mockLogger
			router := api.NewRouter(cfg, &l, nil)
			r.Any("/proxy/:provider/*proxyPath", router.ProxyHandler)

			w := &customResponseWriter{
				ResponseRecorder: httptest.NewRecorder(),
			}
			req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/proxy/%s/v1/models", tt.providerID), nil)
			r.ServeHTTP(w, req)
		})
	}
}

// Custom response writer that skips CloseNotifier
type customResponseWriter struct {
	*httptest.ResponseRecorder
}

func (w *customResponseWriter) CloseNotify() <-chan bool {
	return nil
}
