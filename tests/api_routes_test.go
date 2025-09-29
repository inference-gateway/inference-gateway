package tests

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	gin "github.com/gin-gonic/gin"
	api "github.com/inference-gateway/inference-gateway/api"
	config "github.com/inference-gateway/inference-gateway/config"
	logger "github.com/inference-gateway/inference-gateway/logger"
	providers "github.com/inference-gateway/inference-gateway/providers"
	providersmocks "github.com/inference-gateway/inference-gateway/tests/mocks/providers"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
	gomock "go.uber.org/mock/gomock"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func TestListModelsHandler_AllowedModelsFiltering(t *testing.T) {
	tests := []struct {
		name                         string
		allowedModels                string
		mockModels                   []providers.Model
		expectedModelsSingleProvider []string
		expectedModelsAllProviders   []string
		description                  string
	}{
		{
			name:          "Empty ALLOWED_MODELS returns all models",
			allowedModels: "",
			mockModels: []providers.Model{
				{ID: "gpt-4", Object: "model", Created: 1677649963, OwnedBy: "openai", ServedBy: providers.OpenaiID},
				{ID: "gpt-3.5-turbo", Object: "model", Created: 1677610602, OwnedBy: "openai", ServedBy: providers.OpenaiID},
			},
			expectedModelsSingleProvider: []string{"openai/gpt-4", "openai/gpt-3.5-turbo"},
			expectedModelsAllProviders:   []string{"openai/gpt-4", "openai/gpt-3.5-turbo", "anthropic/gpt-4", "anthropic/gpt-3.5-turbo"},
			description:                  "When MODELS_LIST is empty, all models should be returned",
		},
		{
			name:          "Filter by exact model ID",
			allowedModels: "openai/gpt-4",
			mockModels: []providers.Model{
				{ID: "gpt-4", Object: "model", Created: 1677649963, OwnedBy: "openai", ServedBy: providers.OpenaiID},
				{ID: "gpt-3.5-turbo", Object: "model", Created: 1677610602, OwnedBy: "openai", ServedBy: providers.OpenaiID},
			},
			expectedModelsSingleProvider: []string{"openai/gpt-4"},
			expectedModelsAllProviders:   []string{"openai/gpt-4"},
			description:                  "Should return only the exact model ID match",
		},
		{
			name:          "Filter by model name without provider prefix",
			allowedModels: "gpt-4",
			mockModels: []providers.Model{
				{ID: "gpt-4", Object: "model", Created: 1677649963, OwnedBy: "openai", ServedBy: providers.OpenaiID},
				{ID: "gpt-3.5-turbo", Object: "model", Created: 1677610602, OwnedBy: "openai", ServedBy: providers.OpenaiID},
			},
			expectedModelsSingleProvider: []string{"openai/gpt-4"},
			expectedModelsAllProviders:   []string{"openai/gpt-4", "anthropic/gpt-4"},
			description:                  "Should match models by name without provider prefix",
		},
		{
			name:          "Case insensitive matching",
			allowedModels: "GPT-4",
			mockModels: []providers.Model{
				{ID: "gpt-4", Object: "model", Created: 1677649963, OwnedBy: "openai", ServedBy: providers.OpenaiID},
				{ID: "gpt-3.5-turbo", Object: "model", Created: 1677610602, OwnedBy: "openai", ServedBy: providers.OpenaiID},
			},
			expectedModelsSingleProvider: []string{"openai/gpt-4"},
			expectedModelsAllProviders:   []string{"openai/gpt-4", "anthropic/gpt-4"},
			description:                  "Should match models in a case-insensitive manner",
		},
		{
			name:          "Trim whitespace in ALLOWED_MODELS",
			allowedModels: " gpt-4 ",
			mockModels: []providers.Model{
				{ID: "gpt-4", Object: "model", Created: 1677649963, OwnedBy: "openai", ServedBy: providers.OpenaiID},
				{ID: "gpt-3.5-turbo", Object: "model", Created: 1677610602, OwnedBy: "openai", ServedBy: providers.OpenaiID},
			},
			expectedModelsSingleProvider: []string{"openai/gpt-4"},
			expectedModelsAllProviders:   []string{"openai/gpt-4", "anthropic/gpt-4"},
			description:                  "Should handle whitespace correctly in the models list",
		},
		{
			name:          "No matches returns empty list",
			allowedModels: "nonexistent-model",
			mockModels: []providers.Model{
				{ID: "gpt-4", Object: "model", Created: 1677649963, OwnedBy: "openai", ServedBy: providers.OpenaiID},
				{ID: "gpt-3.5-turbo", Object: "model", Created: 1677610602, OwnedBy: "openai", ServedBy: providers.OpenaiID},
			},
			expectedModelsSingleProvider: []string{},
			expectedModelsAllProviders:   []string{},
			description:                  "Should return empty list when no models match the filter",
		},
		{
			name:          "Mixed exact ID and name matching",
			allowedModels: "openai/gpt-4",
			mockModels: []providers.Model{
				{ID: "gpt-4", Object: "model", Created: 1677649963, OwnedBy: "openai", ServedBy: providers.OpenaiID},
				{ID: "gpt-3.5-turbo", Object: "model", Created: 1677610602, OwnedBy: "openai", ServedBy: providers.OpenaiID},
			},
			expectedModelsSingleProvider: []string{"openai/gpt-4"},
			expectedModelsAllProviders:   []string{"openai/gpt-4"},
			description:                  "Should handle mix of exact ID and name-only matching",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)

				response := providers.ListModelsResponse{
					Object: "list",
					Data:   tt.mockModels,
				}

				jsonResponse, err := json.Marshal(response)
				require.NoError(t, err)
				_, err = w.Write(jsonResponse)
				require.NoError(t, err)
			}))
			defer server.Close()

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			mockClient := providersmocks.NewMockClient(ctrl)

			mockClient.EXPECT().
				Do(gomock.Any()).
				DoAndReturn(func(req *http.Request) (*http.Response, error) {
					return http.DefaultClient.Get(server.URL + "/models")
				}).
				AnyTimes()

			log, err := logger.NewLogger("test")
			require.NoError(t, err)

			providerCfg := map[providers.Provider]*providers.Config{
				providers.OpenaiID: {
					ID:       providers.OpenaiID,
					Name:     providers.OpenaiDisplayName,
					URL:      server.URL,
					Token:    "test-token",
					AuthType: providers.AuthTypeBearer,
					Endpoints: providers.Endpoints{
						Models: providers.OpenaiModelsEndpoint,
					},
				},
				providers.AnthropicID: {
					ID:       providers.AnthropicID,
					Name:     providers.AnthropicDisplayName,
					URL:      server.URL,
					Token:    "test-token",
					AuthType: providers.AuthTypeXheader,
					Endpoints: providers.Endpoints{
						Models: providers.AnthropicModelsEndpoint,
					},
				},
			}

			registry := providers.NewProviderRegistry(providerCfg, log)

			cfg := config.Config{
				AllowedModels: tt.allowedModels,
				Server: &config.ServerConfig{
					ReadTimeout: time.Duration(5000) * time.Millisecond,
				},
				Providers: providerCfg,
			}

			router := api.NewRouter(cfg, log, registry, mockClient, nil)

			gin.SetMode(gin.TestMode)
			r := gin.New()
			r.GET("/v1/models", router.ListModelsHandler)

			t.Run("SingleProvider", func(t *testing.T) {
				w := httptest.NewRecorder()
				req, err := http.NewRequest("GET", "/v1/models?provider=openai", nil)
				require.NoError(t, err)

				r.ServeHTTP(w, req)

				assert.Equal(t, http.StatusOK, w.Code)

				var response providers.ListModelsResponse
				err = json.Unmarshal(w.Body.Bytes(), &response)
				require.NoError(t, err)

				assert.Equal(t, "list", response.Object)
				assert.Equal(t, len(tt.expectedModelsSingleProvider), len(response.Data))

				actualModelIDs := make([]string, len(response.Data))
				for i, model := range response.Data {
					actualModelIDs[i] = model.ID
				}

				for _, expectedID := range tt.expectedModelsSingleProvider {
					assert.Contains(t, actualModelIDs, expectedID, "Expected model %s not found in response", expectedID)
				}
			})

			t.Run("AllProviders", func(t *testing.T) {
				w := httptest.NewRecorder()
				req, err := http.NewRequest("GET", "/v1/models", nil)
				require.NoError(t, err)

				r.ServeHTTP(w, req)

				assert.Equal(t, http.StatusOK, w.Code)

				var response providers.ListModelsResponse
				err = json.Unmarshal(w.Body.Bytes(), &response)
				require.NoError(t, err)

				assert.Equal(t, "list", response.Object)
				assert.Equal(t, len(tt.expectedModelsAllProviders), len(response.Data))

				actualModelIDs := make([]string, len(response.Data))
				for i, model := range response.Data {
					actualModelIDs[i] = model.ID
				}

				for _, expectedID := range tt.expectedModelsAllProviders {
					assert.Contains(t, actualModelIDs, expectedID, "Expected model %s not found in response", expectedID)
				}
			})
		})
	}
}

func TestListModelsHandler_ErrorCases(t *testing.T) {
	tests := []struct {
		name           string
		providerParam  string
		mockSetup      func(*providersmocks.MockClient)
		expectedStatus int
		expectedError  string
		description    string
	}{
		{
			name:           "Unknown provider",
			providerParam:  "unknown",
			mockSetup:      func(mockClient *providersmocks.MockClient) {},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Provider not found",
			description:    "Should return error for unknown provider",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			mockClient := providersmocks.NewMockClient(ctrl)
			tt.mockSetup(mockClient)

			log, err := logger.NewLogger("test")
			require.NoError(t, err)

			registry := providers.NewProviderRegistry(map[providers.Provider]*providers.Config{}, log)

			cfg := config.Config{
				Server: &config.ServerConfig{
					ReadTimeout: time.Duration(5000) * time.Millisecond,
				},
			}

			router := api.NewRouter(cfg, log, registry, mockClient, nil)

			gin.SetMode(gin.TestMode)
			r := gin.New()
			r.GET("/v1/models", router.ListModelsHandler)

			w := httptest.NewRecorder()
			req, err := http.NewRequest("GET", "/v1/models?provider="+tt.providerParam, nil)
			require.NoError(t, err)

			r.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)

			var response map[string]interface{}
			err = json.Unmarshal(w.Body.Bytes(), &response)
			require.NoError(t, err)

			errorMsg, exists := response["error"]
			assert.True(t, exists, "Response should contain error field")
			assert.Contains(t, errorMsg.(string), tt.expectedError, "Error message should contain expected text")
		})
	}
}

func TestChatCompletionsHandler_ModelValidation(t *testing.T) {
	tests := []struct {
		name           string
		allowedModels  string
		requestModel   string
		expectedStatus int
		expectedError  string
		description    string
	}{
		{
			name:           "Allowed model passes validation",
			allowedModels:  "gpt-4,claude-3",
			requestModel:   "openai/gpt-4",
			expectedStatus: http.StatusOK,
			expectedError:  "",
			description:    "Should allow requests with models in the allowed list",
		},
		{
			name:           "Disallowed model fails validation",
			allowedModels:  "gpt-4,claude-3",
			requestModel:   "openai/gpt-3.5-turbo",
			expectedStatus: http.StatusForbidden,
			expectedError:  "Model not allowed. Please check the list of allowed models.",
			description:    "Should reject requests with models not in the allowed list",
		},
		{
			name:           "Empty allowed models allows all",
			allowedModels:  "",
			requestModel:   "openai/gpt-3.5-turbo",
			expectedStatus: http.StatusOK,
			expectedError:  "",
			description:    "Should allow all models when ALLOWED_MODELS is empty",
		},
		{
			name:           "Case insensitive model validation",
			allowedModels:  "GPT-4,CLAUDE-3",
			requestModel:   "openai/gpt-4",
			expectedStatus: http.StatusOK,
			expectedError:  "",
			description:    "Should validate models in a case-insensitive manner",
		},
		{
			name:           "Exact model ID matching",
			allowedModels:  "openai/gpt-4",
			requestModel:   "openai/gpt-4",
			expectedStatus: http.StatusOK,
			expectedError:  "",
			description:    "Should allow exact model ID matches",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)

				response := map[string]interface{}{
					"id":      "chatcmpl-test",
					"object":  "chat.completion",
					"created": 1677649963,
					"model":   tt.requestModel,
					"choices": []map[string]interface{}{
						{
							"index": 0,
							"message": map[string]interface{}{
								"role":    "assistant",
								"content": "Test response",
							},
							"finish_reason": "stop",
						},
					},
				}

				jsonResponse, _ := json.Marshal(response)
				_, _ = w.Write(jsonResponse)
			}))
			defer server.Close()

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			mockClient := providersmocks.NewMockClient(ctrl)

			mockClient.EXPECT().
				Do(gomock.Any()).
				DoAndReturn(func(req *http.Request) (*http.Response, error) {
					if tt.expectedStatus == http.StatusOK {
						testReq, err := http.NewRequest(req.Method, server.URL+"/chat/completions", req.Body)
						if err != nil {
							return nil, err
						}

						for key, values := range req.Header {
							for _, value := range values {
								testReq.Header.Add(key, value)
							}
						}
						return http.DefaultClient.Do(testReq)
					}
					return nil, nil
				}).
				AnyTimes()

			log, err := logger.NewLogger("test")
			require.NoError(t, err)

			providerCfg := map[providers.Provider]*providers.Config{
				providers.OpenaiID: {
					ID:       providers.OpenaiID,
					Name:     providers.OpenaiDisplayName,
					URL:      server.URL,
					Token:    "test-token",
					AuthType: providers.AuthTypeBearer,
					Endpoints: providers.Endpoints{
						Chat: providers.OpenaiChatEndpoint,
					},
				},
			}

			registry := providers.NewProviderRegistry(providerCfg, log)

			cfg := config.Config{
				AllowedModels: tt.allowedModels,
				Server: &config.ServerConfig{
					ReadTimeout: time.Duration(5000) * time.Millisecond,
				},
				Providers: providerCfg,
			}

			router := api.NewRouter(cfg, log, registry, mockClient, nil)

			gin.SetMode(gin.TestMode)
			r := gin.New()
			r.POST("/v1/chat/completions", router.ChatCompletionsHandler)

			requestBody := map[string]interface{}{
				"model": tt.requestModel,
				"messages": []map[string]string{
					{
						"role":    "user",
						"content": "Hello, world!",
					},
				},
			}

			jsonBody, err := json.Marshal(requestBody)
			require.NoError(t, err)

			w := httptest.NewRecorder()
			req, err := http.NewRequest("POST", "/v1/chat/completions", strings.NewReader(string(jsonBody)))
			require.NoError(t, err)
			req.Header.Set("Content-Type", "application/json")

			r.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)

			var response map[string]interface{}
			err = json.Unmarshal(w.Body.Bytes(), &response)
			require.NoError(t, err)

			if tt.expectedError != "" {
				errorMsg, exists := response["error"]
				assert.True(t, exists, "Response should contain error field")
				assert.Contains(t, errorMsg.(string), tt.expectedError, "Error message should contain expected text")
			} else {
				assert.Equal(t, "chat.completion", response["object"])
				assert.NotEmpty(t, response["model"])
			}
		})
	}
}
