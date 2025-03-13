package tests

import (
	"bytes"
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
	r.GET("/v1/models", router.ListModelsOpenAICompatibleHandler)
	r.POST("/v1/chat/completions", router.ChatCompletionsOpenAICompatibleHandler)
	r.GET("/health", router.HealthcheckHandler)

	return r, mockRegistry, mockClient, mockLogger
}

func TestHealthcheckHandler(t *testing.T) {
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

func TestListModelsHandler(t *testing.T) {
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
			name:   "list models returns models from provider",
			method: "GET",
			url:    "/v1/models?provider=test-provider",
			setupMocks: func(mr *mocks.MockProviderRegistry, mc *mocks.MockClient, ml *mocks.MockLogger) {
				mockProvider := mocks.NewMockProvider(gomock.NewController(t))
				mr.EXPECT().
					BuildProvider("test-provider", mc).
					Return(mockProvider, nil)

				mockProvider.EXPECT().
					ListModels(gomock.Any()).
					Return(providers.ListModelsResponse{
						Provider: "test-provider",
						Object:   "list",
						Data: []providers.Model{
							{
								ID:       "Test Model 1",
								Object:   "model",
								Created:  0,
								OwnedBy:  "test-provider",
								ServedBy: "test-provider",
							},
						},
					}, nil)
			},
			expectedCode: http.StatusOK,
			expectedBody: providers.ListModelsResponse{
				Object:   "list",
				Provider: "test-provider",
				Data: []providers.Model{
					{
						ID:       "Test Model 1",
						Object:   "model",
						Created:  0,
						OwnedBy:  "test-provider",
						ServedBy: "test-provider",
					},
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

func TestChatCompletionsHandler(t *testing.T) {
	tests := []struct {
		name         string
		body         any
		setupMocks   func(*mocks.MockProviderRegistry, *mocks.MockClient, *mocks.MockLogger)
		expectedCode int
		expectedResp func() string
		checkBody    func(t *testing.T, body string)
		provider     string
	}{
		{
			name: "invalid request body",
			body: "invalid json",
			setupMocks: func(mr *mocks.MockProviderRegistry, mc *mocks.MockClient, ml *mocks.MockLogger) {
				ml.EXPECT().Error("failed to decode request", gomock.Any())
			},
			expectedCode: http.StatusBadRequest,
			expectedResp: func() string {
				resp, _ := json.Marshal(api.ErrorResponse{Error: "Failed to decode request"})
				return string(resp)
			},
		},
		{
			name: "missing provider and model",
			body: providers.ChatCompletionsRequest{
				Model:    "test-model",
				Messages: []providers.Message{{Role: "user", Content: "Hello"}},
			},
			setupMocks: func(mr *mocks.MockProviderRegistry, mc *mocks.MockClient, ml *mocks.MockLogger) {
				ml.EXPECT().Error("unable to determine provider for model", nil, "model", "test-model") // Updated to match actual call
			},
			expectedCode: http.StatusBadRequest,
			expectedResp: func() string {
				resp, _ := json.Marshal(api.ErrorResponse{Error: "Unable to determine provider for model. Please specify a provider."})
				return string(resp)
			},
		},
		{
			name: "implicit provider by model",
			body: providers.ChatCompletionsRequest{
				Model:    "gpt-model",
				Messages: []providers.Message{{Role: "user", Content: "Hello"}},
			},
			setupMocks: func(mr *mocks.MockProviderRegistry, mc *mocks.MockClient, ml *mocks.MockLogger) {
				mr.EXPECT().
					BuildProvider("openai", mc).
					Return(nil, errors.New("token not configured"))
				ml.EXPECT().
					Error("provider requires authentication but no API key was configured",
						gomock.Any(),
						"provider", gomock.Any())
			},
			expectedCode: http.StatusBadRequest,
			expectedResp: func() string {
				resp, _ := json.Marshal(api.ErrorResponse{Error: "Provider requires an API key. Please configure the provider's API key."})
				return string(resp)
			},
		},
		{
			name: "provider not configured",
			body: providers.ChatCompletionsRequest{
				Model:    "test-model",
				Messages: []providers.Message{{Role: "user", Content: "Hello"}},
			},
			provider: "test-provider",
			setupMocks: func(mr *mocks.MockProviderRegistry, mc *mocks.MockClient, ml *mocks.MockLogger) {
				mr.EXPECT().
					BuildProvider(gomock.Any(), mc).
					Return(nil, errors.New("token not configured"))
				ml.EXPECT().
					Error("provider requires authentication but no API key was configured",
						gomock.Any(),
						"provider", gomock.Any())
			},
			expectedCode: http.StatusBadRequest,
			expectedResp: func() string {
				resp, _ := json.Marshal(api.ErrorResponse{Error: "Provider requires an API key. Please configure the provider's API key."})
				return string(resp)
			},
		},
		{
			name: "successful non-streaming request",
			body: providers.ChatCompletionsRequest{
				Model:    "test-model",
				Messages: []providers.Message{{Role: "user", Content: "Hello"}},
				Stream:   false,
			},
			provider: "test-provider",
			setupMocks: func(mr *mocks.MockProviderRegistry, mc *mocks.MockClient, ml *mocks.MockLogger) {
				mockProvider := mocks.NewMockProvider(gomock.NewController(t))
				mr.EXPECT().
					BuildProvider(gomock.Any(), mc).
					Return(mockProvider, nil)
				mockProvider.EXPECT().
					GenerateTokens(gomock.Any(), "test-model", gomock.Any(), gomock.Any(), gomock.Any()).
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
			checkBody: func(t *testing.T, body string) {
				var resp providers.CompletionResponse
				err := json.Unmarshal([]byte(body), &resp)
				assert.NoError(t, err)
				assert.Equal(t, "chat.completion", resp.Object)
				assert.Equal(t, "test-model", resp.Model)
				assert.Equal(t, 1, len(resp.Choices))
				assert.Equal(t, "Hello back!", resp.Choices[0].Message.Content)
				assert.Equal(t, "assistant", resp.Choices[0].Message.Role)
			},
		},
		{
			name: "streaming request",
			body: providers.ChatCompletionsRequest{
				Model:    "test-model",
				Messages: []providers.Message{{Role: "user", Content: "Hello"}},
				Stream:   true,
			},
			provider: "test-provider",
			setupMocks: func(mr *mocks.MockProviderRegistry, mc *mocks.MockClient, ml *mocks.MockLogger) {
				mockProvider := mocks.NewMockProvider(gomock.NewController(t))
				mr.EXPECT().
					BuildProvider(gomock.Any(), mc).
					Return(mockProvider, nil)

				streamCh := make(chan providers.GenerateResponse)
				mockProvider.EXPECT().
					StreamTokens(gomock.Any(), "test-model", gomock.Any()).
					Return(streamCh, nil)

				go func() {
					streamCh <- providers.GenerateResponse{
						Provider: "test-provider",
						Response: providers.ResponseTokens{
							Content: "Hello",
							Model:   "test-model",
							Role:    "assistant",
						},
						EventType: providers.EventContentDelta,
					}
					streamCh <- providers.GenerateResponse{
						Provider: "test-provider",
						Response: providers.ResponseTokens{
							Content: " back!",
							Model:   "test-model",
							Role:    "assistant",
						},
						EventType: providers.EventContentDelta,
					}
					close(streamCh)
				}()
			},
			expectedCode: http.StatusOK,
			checkBody: func(t *testing.T, body string) {
				lines := strings.Split(strings.TrimSpace(body), "\n\n")
				assert.GreaterOrEqual(t, len(lines), 2, "Expected at least 2 SSE messages")

				for _, line := range lines {
					if !strings.HasPrefix(line, "data: ") {
						continue
					}

					data := strings.TrimPrefix(line, "data: ")
					if data == "[DONE]" {
						continue
					}

					var chunk providers.ChunkResponse
					err := json.Unmarshal([]byte(data), &chunk)
					assert.NoError(t, err)
					assert.Equal(t, "chat.completion.chunk", chunk.Object)
				}
			},
		},
		{
			name: "generation error",
			body: providers.ChatCompletionsRequest{
				Model:    "test-model",
				Messages: []providers.Message{{Role: "user", Content: "Hello"}},
			},
			provider: "test-provider",
			setupMocks: func(mr *mocks.MockProviderRegistry, mc *mocks.MockClient, ml *mocks.MockLogger) {
				mockProvider := mocks.NewMockProvider(gomock.NewController(t))
				mr.EXPECT().
					BuildProvider(gomock.Any(), mc).
					Return(mockProvider, nil)
				mockProvider.EXPECT().
					GenerateTokens(gomock.Any(), "test-model", gomock.Any(), gomock.Any(), gomock.Any()).
					Return(providers.GenerateResponse{}, errors.New("generation failed"))
				ml.EXPECT().
					Error("failed to generate tokens", gomock.Any(), "provider", gomock.Any())
			},
			expectedCode: http.StatusBadRequest,
			expectedResp: func() string {
				resp, _ := json.Marshal(api.ErrorResponse{Error: "Failed to generate tokens"})
				return string(resp)
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
				var jsonBody []byte
				if s, ok := tt.body.(string); ok {
					jsonBody = []byte(s)
				} else {
					jsonBody, _ = json.Marshal(tt.body)
				}

				url := "/v1/chat/completions"
				if tt.provider != "" {
					url += "?provider=" + tt.provider
				}

				req = httptest.NewRequest(http.MethodPost, url, bytes.NewReader(jsonBody))
				req.Header.Set("Content-Type", "application/json")
			} else {
				req = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
			}

			var w *httptest.ResponseRecorder
			if tt.name == "streaming request" {
				cnr := NewCloseNotifierResponseRecorder()
				router.ServeHTTP(cnr, req)
				w = cnr.ResponseRecorder
			} else {
				w = httptest.NewRecorder()
				router.ServeHTTP(w, req)
			}

			assert.Equal(t, tt.expectedCode, w.Code)

			if tt.expectedResp != nil {
				assert.Equal(t, tt.expectedResp(), strings.TrimSpace(w.Body.String()))
			}

			if tt.checkBody != nil {
				tt.checkBody(t, w.Body.String())
			}
		})
	}
}

type CloseNotifierResponseRecorder struct {
	*httptest.ResponseRecorder
	closed chan bool
}

func NewCloseNotifierResponseRecorder() *CloseNotifierResponseRecorder {
	return &CloseNotifierResponseRecorder{
		ResponseRecorder: httptest.NewRecorder(),
		closed:           make(chan bool, 1),
	}
}

func (r *CloseNotifierResponseRecorder) CloseNotify() <-chan bool {
	return r.closed
}
