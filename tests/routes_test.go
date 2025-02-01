package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/inference-gateway/inference-gateway/api"
	"github.com/inference-gateway/inference-gateway/config"
	"github.com/inference-gateway/inference-gateway/providers"
	"github.com/inference-gateway/inference-gateway/tests/mocks"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

func setupTestRouter(t *testing.T) (*gin.Engine, *mocks.MockProviderRegistry, *mocks.MockClient, *mocks.MockLogger) {
	ctrl := gomock.NewController(t)
	mockRegistry := mocks.NewMockProviderRegistry(ctrl)
	mockClient := mocks.NewMockClient(ctrl)
	mockLogger := mocks.NewMockLogger(ctrl)

	cfg := config.Config{
		Server: &config.ServerConfig{
			ReadTimeout: 30000,
		},
	}

	router := api.NewRouter(cfg, mockLogger, mockRegistry, mockClient)

	// Setup Gin router
	r := gin.New()
	r.GET("/health", router.HealthcheckHandler)
	r.GET("/llms/:provider/models", router.ListModelsHandler)
	r.POST("/llms/:provider/generate", router.GenerateProvidersTokenHandler)

	return r, mockRegistry, mockClient, mockLogger
}

func TestRouterHandlers(t *testing.T) {
	tests := []struct {
		name         string
		method       string
		url          string
		body         interface{}
		setupMocks   func(*mocks.MockProviderRegistry, *mocks.MockClient, *mocks.MockLogger)
		expectedCode int
		expectedBody interface{}
	}{
		{
			name:   "healthcheck returns OK",
			method: "GET",
			url:    "/health",
			setupMocks: func(mr *mocks.MockProviderRegistry, mc *mocks.MockClient, ml *mocks.MockLogger) {
				ml.EXPECT().Debug("healthcheck")
			},
			expectedCode: http.StatusOK,
			expectedBody: gin.H{"message": "OK"},
		},
		{
			name:   "list models returns models from provider",
			method: "GET",
			url:    "/llms/test-provider/models",
			setupMocks: func(mr *mocks.MockProviderRegistry, mc *mocks.MockClient, ml *mocks.MockLogger) {
				mockProvider := mocks.NewMockProvider(gomock.NewController(t))
				mr.EXPECT().
					BuildProvider("test-provider", mc).
					Return(mockProvider, nil)

				mockProvider.EXPECT().
					ListModels(gomock.Any()).
					Return(providers.ListModelsResponse{
						Provider: "test-provider",
						Models: []providers.Model{
							{Name: "Test Model 1"},
						},
					}, nil)
			},
			expectedCode: http.StatusOK,
			expectedBody: providers.ListModelsResponse{
				Provider: "test-provider",
				Models: []providers.Model{
					{Name: "Test Model 1"},
				},
			},
		},
		{
			name:   "generate tokens returns response",
			method: "POST",
			url:    "/llms/test-provider/generate",
			body: providers.GenerateRequest{
				Model: "test-model",
				Messages: []providers.Message{
					{Role: "user", Content: "Hello"},
				},
			},
			setupMocks: func(mr *mocks.MockProviderRegistry, mc *mocks.MockClient, ml *mocks.MockLogger) {
				mockProvider := mocks.NewMockProvider(gomock.NewController(t))
				mr.EXPECT().
					BuildProvider("test-provider", mc).
					Return(mockProvider, nil)

				mockProvider.EXPECT().
					GenerateTokens(gomock.Any(), "test-model", gomock.Any()).
					Return(providers.GenerateResponse{
						Provider: "test-provider",
						Response: providers.ResponseTokens{
							Content: "Hello back!",
							Model:   "test-model",
							Role:    "assistant",
						},
					}, nil)
			},
			expectedCode: http.StatusOK,
			expectedBody: providers.GenerateResponse{
				Provider: "test-provider",
				Response: providers.ResponseTokens{
					Content: "Hello back!",
					Model:   "test-model",
					Role:    "assistant",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router, mockRegistry, mockClient, mockLogger := setupTestRouter(t)

			if tt.setupMocks != nil {
				tt.setupMocks(mockRegistry, mockClient, mockLogger)
			}

			var req *http.Request
			if tt.body != nil {
				jsonBody, _ := json.Marshal(tt.body)
				req = httptest.NewRequest(tt.method, tt.url, bytes.NewReader(jsonBody))
				req.Header.Set("Content-Type", "application/json")
			} else {
				req = httptest.NewRequest(tt.method, tt.url, nil)
			}

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedCode, w.Code)

			expectedJSON, err := json.Marshal(tt.expectedBody)
			assert.NoError(t, err)

			assert.Equal(t, string(expectedJSON), w.Body.String())
		})
	}
}

func TestGenerateProvidersTokenHandler(t *testing.T) {
	tests := []struct {
		name         string
		url          string
		body         interface{}
		setupMocks   func(*mocks.MockProviderRegistry, *mocks.MockClient, *mocks.MockLogger)
		expectedCode int
		expectedBody interface{}
	}{
		{
			name: "invalid request body",
			url:  "/llms/test-provider/generate",
			body: "invalid json",
			setupMocks: func(mr *mocks.MockProviderRegistry, mc *mocks.MockClient, ml *mocks.MockLogger) {
			},
			expectedCode: http.StatusBadRequest,
			expectedBody: api.ErrorResponse{Error: "Failed to decode request"},
		},
		{
			name: "missing model",
			url:  "/llms/test-provider/generate",
			body: providers.GenerateRequest{
				Messages: []providers.Message{{Role: "user", Content: "Hello"}},
			},
			setupMocks: func(mr *mocks.MockProviderRegistry, mc *mocks.MockClient, ml *mocks.MockLogger) {
				ml.EXPECT().Error("model is required", nil)
			},
			expectedCode: http.StatusBadRequest,
			expectedBody: api.ErrorResponse{Error: "Model is required"},
		},
		{
			name: "provider not configured",
			url:  "/llms/test-provider/generate",
			body: providers.GenerateRequest{
				Model:    "test-model",
				Messages: []providers.Message{{Role: "user", Content: "Hello"}},
			},
			setupMocks: func(mr *mocks.MockProviderRegistry, mc *mocks.MockClient, ml *mocks.MockLogger) {
				mr.EXPECT().
					BuildProvider("test-provider", mc).
					Return(nil, errors.New("token not configured"))
				ml.EXPECT().
					Error("provider requires authentication but no API key was configured",
						gomock.Any(),
						"provider", "test-provider")
			},
			expectedCode: http.StatusBadRequest,
			expectedBody: api.ErrorResponse{Error: "Provider requires an API key. Please configure the provider's API key."},
		},
		{
			name: "successful non-streaming request",
			url:  "/llms/test-provider/generate",
			body: providers.GenerateRequest{
				Model:    "test-model",
				Messages: []providers.Message{{Role: "user", Content: "Hello"}},
			},
			setupMocks: func(mr *mocks.MockProviderRegistry, mc *mocks.MockClient, ml *mocks.MockLogger) {
				mockProvider := mocks.NewMockProvider(gomock.NewController(t))
				mr.EXPECT().
					BuildProvider("test-provider", mc).
					Return(mockProvider, nil)
				mockProvider.EXPECT().
					GenerateTokens(gomock.Any(), "test-model", gomock.Any()).
					Return(providers.GenerateResponse{
						Provider: "test-provider",
						Response: providers.ResponseTokens{
							Content: "Hello back!",
							Model:   "test-model",
							Role:    "assistant",
						},
					}, nil)
			},
			expectedCode: http.StatusOK,
			expectedBody: providers.GenerateResponse{
				Provider: "test-provider",
				Response: providers.ResponseTokens{
					Content: "Hello back!",
					Model:   "test-model",
					Role:    "assistant",
				},
			},
		},
		{
			name: "generation timeout",
			url:  "/llms/test-provider/generate",
			body: providers.GenerateRequest{
				Model:    "test-model",
				Messages: []providers.Message{{Role: "user", Content: "Hello"}},
			},
			setupMocks: func(mr *mocks.MockProviderRegistry, mc *mocks.MockClient, ml *mocks.MockLogger) {
				mockProvider := mocks.NewMockProvider(gomock.NewController(t))
				mr.EXPECT().
					BuildProvider("test-provider", mc).
					Return(mockProvider, nil)
				mockProvider.EXPECT().
					GenerateTokens(gomock.Any(), "test-model", gomock.Any()).
					Return(providers.GenerateResponse{}, context.DeadlineExceeded)
				ml.EXPECT().
					Error("request timed out", gomock.Any(), "provider", "test-provider")
			},
			expectedCode: http.StatusGatewayTimeout,
			expectedBody: api.ErrorResponse{Error: "Request timed out"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router, mockRegistry, mockClient, mockLogger := setupTestRouter(t)

			if tt.setupMocks != nil {
				tt.setupMocks(mockRegistry, mockClient, mockLogger)
			}

			var req *http.Request
			if tt.body != nil {
				var jsonBody []byte
				if s, ok := tt.body.(string); ok {
					jsonBody = []byte(s)
				} else {
					jsonBody, _ = json.Marshal(tt.body)
				}
				req = httptest.NewRequest(http.MethodPost, tt.url, bytes.NewReader(jsonBody))
				req.Header.Set("Content-Type", "application/json")
			} else {
				req = httptest.NewRequest(http.MethodPost, tt.url, nil)
			}

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedCode, w.Code)

			expectedJSON, err := json.Marshal(tt.expectedBody)
			assert.NoError(t, err)
			assert.Equal(t, string(expectedJSON), strings.TrimSpace(w.Body.String()))
		})
	}
}
