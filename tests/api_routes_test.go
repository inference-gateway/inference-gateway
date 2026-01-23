package tests

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	gin "github.com/gin-gonic/gin"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
	gomock "go.uber.org/mock/gomock"

	api "github.com/inference-gateway/inference-gateway/api"
	config "github.com/inference-gateway/inference-gateway/config"
	logger "github.com/inference-gateway/inference-gateway/logger"
	constants "github.com/inference-gateway/inference-gateway/providers/constants"
	core "github.com/inference-gateway/inference-gateway/providers/core"
	registry "github.com/inference-gateway/inference-gateway/providers/registry"
	types "github.com/inference-gateway/inference-gateway/providers/types"
	providersmocks "github.com/inference-gateway/inference-gateway/tests/mocks/providers"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func TestListModelsHandler_AllowedModelsFiltering(t *testing.T) {
	tests := []struct {
		name                         string
		allowedModels                string
		mockModels                   []types.Model
		expectedModelsSingleProvider []string
		expectedModelsAllProviders   []string
		description                  string
	}{
		{
			name:          "Empty ALLOWED_MODELS returns all models",
			allowedModels: "",
			mockModels: []types.Model{
				{ID: "gpt-4", Object: "model", Created: 1677649963, OwnedBy: "openai", ServedBy: constants.OpenaiID},
				{ID: "gpt-3.5-turbo", Object: "model", Created: 1677610602, OwnedBy: "openai", ServedBy: constants.OpenaiID},
			},
			expectedModelsSingleProvider: []string{"openai/gpt-4", "openai/gpt-3.5-turbo"},
			expectedModelsAllProviders:   []string{"openai/gpt-4", "openai/gpt-3.5-turbo", "anthropic/gpt-4", "anthropic/gpt-3.5-turbo"},
			description:                  "When MODELS_LIST is empty, all models should be returned",
		},
		{
			name:          "Filter by exact model ID",
			allowedModels: "openai/gpt-4",
			mockModels: []types.Model{
				{ID: "gpt-4", Object: "model", Created: 1677649963, OwnedBy: "openai", ServedBy: constants.OpenaiID},
				{ID: "gpt-3.5-turbo", Object: "model", Created: 1677610602, OwnedBy: "openai", ServedBy: constants.OpenaiID},
			},
			expectedModelsSingleProvider: []string{"openai/gpt-4"},
			expectedModelsAllProviders:   []string{"openai/gpt-4"},
			description:                  "Should return only the exact model ID match",
		},
		{
			name:          "Filter by model name without provider prefix",
			allowedModels: "gpt-4",
			mockModels: []types.Model{
				{ID: "gpt-4", Object: "model", Created: 1677649963, OwnedBy: "openai", ServedBy: constants.OpenaiID},
				{ID: "gpt-3.5-turbo", Object: "model", Created: 1677610602, OwnedBy: "openai", ServedBy: constants.OpenaiID},
			},
			expectedModelsSingleProvider: []string{"openai/gpt-4"},
			expectedModelsAllProviders:   []string{"openai/gpt-4", "anthropic/gpt-4"},
			description:                  "Should match models by name without provider prefix",
		},
		{
			name:          "Case insensitive matching",
			allowedModels: "GPT-4",
			mockModels: []types.Model{
				{ID: "gpt-4", Object: "model", Created: 1677649963, OwnedBy: "openai", ServedBy: constants.OpenaiID},
				{ID: "gpt-3.5-turbo", Object: "model", Created: 1677610602, OwnedBy: "openai", ServedBy: constants.OpenaiID},
			},
			expectedModelsSingleProvider: []string{"openai/gpt-4"},
			expectedModelsAllProviders:   []string{"openai/gpt-4", "anthropic/gpt-4"},
			description:                  "Should match models in a case-insensitive manner",
		},
		{
			name:          "Trim whitespace in ALLOWED_MODELS",
			allowedModels: " gpt-4 ",
			mockModels: []types.Model{
				{ID: "gpt-4", Object: "model", Created: 1677649963, OwnedBy: "openai", ServedBy: constants.OpenaiID},
				{ID: "gpt-3.5-turbo", Object: "model", Created: 1677610602, OwnedBy: "openai", ServedBy: constants.OpenaiID},
			},
			expectedModelsSingleProvider: []string{"openai/gpt-4"},
			expectedModelsAllProviders:   []string{"openai/gpt-4", "anthropic/gpt-4"},
			description:                  "Should handle whitespace correctly in the models list",
		},
		{
			name:          "No matches returns empty list",
			allowedModels: "nonexistent-model",
			mockModels: []types.Model{
				{ID: "gpt-4", Object: "model", Created: 1677649963, OwnedBy: "openai", ServedBy: constants.OpenaiID},
				{ID: "gpt-3.5-turbo", Object: "model", Created: 1677610602, OwnedBy: "openai", ServedBy: constants.OpenaiID},
			},
			expectedModelsSingleProvider: []string{},
			expectedModelsAllProviders:   []string{},
			description:                  "Should return empty list when no models match the filter",
		},
		{
			name:          "Mixed exact ID and name matching",
			allowedModels: "openai/gpt-4",
			mockModels: []types.Model{
				{ID: "gpt-4", Object: "model", Created: 1677649963, OwnedBy: "openai", ServedBy: constants.OpenaiID},
				{ID: "gpt-3.5-turbo", Object: "model", Created: 1677610602, OwnedBy: "openai", ServedBy: constants.OpenaiID},
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

				response := types.ListModelsResponse{
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

			providerCfg := map[types.Provider]*registry.ProviderConfig{
				constants.OpenaiID: {
					ID:       constants.OpenaiID,
					Name:     constants.OpenaiDisplayName,
					URL:      server.URL,
					Token:    "test-token",
					AuthType: constants.AuthTypeBearer,
					Endpoints: types.Endpoints{
						Models: constants.OpenaiModelsEndpoint,
					},
				},
				constants.AnthropicID: {
					ID:       constants.AnthropicID,
					Name:     constants.AnthropicDisplayName,
					URL:      server.URL,
					Token:    "test-token",
					AuthType: constants.AuthTypeXheader,
					Endpoints: types.Endpoints{
						Models: constants.AnthropicModelsEndpoint,
					},
				},
			}

			registry := registry.NewProviderRegistry(providerCfg, log)

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

				var response types.ListModelsResponse
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

				var response types.ListModelsResponse
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

			registry := registry.NewProviderRegistry(map[types.Provider]*registry.ProviderConfig{}, log)

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

			var response map[string]any
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

				response := map[string]any{
					"id":      "chatcmpl-test",
					"object":  "chat.completion",
					"created": 1677649963,
					"model":   tt.requestModel,
					"choices": []map[string]any{
						{
							"index": 0,
							"message": map[string]any{
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

			providerCfg := map[types.Provider]*registry.ProviderConfig{
				constants.OpenaiID: {
					ID:       constants.OpenaiID,
					Name:     constants.OpenaiDisplayName,
					URL:      server.URL,
					Token:    "test-token",
					AuthType: constants.AuthTypeBearer,
					Endpoints: types.Endpoints{
						Chat: constants.OpenaiChatEndpoint,
					},
				},
			}

			registry := registry.NewProviderRegistry(providerCfg, log)

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

			requestBody := map[string]any{
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

			var response map[string]any
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

func TestListModelsHandler_DisallowedModelsFiltering(t *testing.T) {
	tests := []struct {
		name                         string
		disallowedModels             string
		mockModels                   []types.Model
		expectedModelsSingleProvider []string
		expectedModelsAllProviders   []string
		description                  string
	}{
		{
			name:             "Empty DISALLOWED_MODELS returns all models",
			disallowedModels: "",
			mockModels: []types.Model{
				{ID: "gpt-4", Object: "model", Created: 1677649963, OwnedBy: "openai", ServedBy: constants.OpenaiID},
				{ID: "gpt-3.5-turbo", Object: "model", Created: 1677610602, OwnedBy: "openai", ServedBy: constants.OpenaiID},
			},
			expectedModelsSingleProvider: []string{"openai/gpt-4", "openai/gpt-3.5-turbo"},
			expectedModelsAllProviders:   []string{"openai/gpt-4", "openai/gpt-3.5-turbo", "anthropic/gpt-4", "anthropic/gpt-3.5-turbo"},
			description:                  "When DISALLOWED_MODELS is empty, all models should be returned",
		},
		{
			name:             "Disallow specific model by exact ID",
			disallowedModels: "openai/gpt-4",
			mockModels: []types.Model{
				{ID: "gpt-4", Object: "model", Created: 1677649963, OwnedBy: "openai", ServedBy: constants.OpenaiID},
				{ID: "gpt-3.5-turbo", Object: "model", Created: 1677610602, OwnedBy: "openai", ServedBy: constants.OpenaiID},
			},
			expectedModelsSingleProvider: []string{"openai/gpt-3.5-turbo"},
			expectedModelsAllProviders:   []string{"openai/gpt-3.5-turbo", "anthropic/gpt-4", "anthropic/gpt-3.5-turbo"},
			description:                  "Should block only the exact model ID match",
		},
		{
			name:             "Disallow by model name without provider prefix",
			disallowedModels: "gpt-4",
			mockModels: []types.Model{
				{ID: "gpt-4", Object: "model", Created: 1677649963, OwnedBy: "openai", ServedBy: constants.OpenaiID},
				{ID: "gpt-3.5-turbo", Object: "model", Created: 1677610602, OwnedBy: "openai", ServedBy: constants.OpenaiID},
			},
			expectedModelsSingleProvider: []string{"openai/gpt-3.5-turbo"},
			expectedModelsAllProviders:   []string{"openai/gpt-3.5-turbo", "anthropic/gpt-3.5-turbo"},
			description:                  "Should block models by name across all providers",
		},
		{
			name:             "Case insensitive disallowing",
			disallowedModels: "GPT-4",
			mockModels: []types.Model{
				{ID: "gpt-4", Object: "model", Created: 1677649963, OwnedBy: "openai", ServedBy: constants.OpenaiID},
				{ID: "gpt-3.5-turbo", Object: "model", Created: 1677610602, OwnedBy: "openai", ServedBy: constants.OpenaiID},
			},
			expectedModelsSingleProvider: []string{"openai/gpt-3.5-turbo"},
			expectedModelsAllProviders:   []string{"openai/gpt-3.5-turbo", "anthropic/gpt-3.5-turbo"},
			description:                  "Should match disallowed models in a case-insensitive manner",
		},
		{
			name:             "Disallow multiple models",
			disallowedModels: "gpt-4,gpt-3.5-turbo",
			mockModels: []types.Model{
				{ID: "gpt-4", Object: "model", Created: 1677649963, OwnedBy: "openai", ServedBy: constants.OpenaiID},
				{ID: "gpt-3.5-turbo", Object: "model", Created: 1677610602, OwnedBy: "openai", ServedBy: constants.OpenaiID},
			},
			expectedModelsSingleProvider: []string{},
			expectedModelsAllProviders:   []string{},
			description:                  "Should block multiple models specified in the disallowed list",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)

				response := types.ListModelsResponse{
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

			providerCfg := map[types.Provider]*registry.ProviderConfig{
				constants.OpenaiID: {
					ID:       constants.OpenaiID,
					Name:     constants.OpenaiDisplayName,
					URL:      server.URL,
					Token:    "test-token",
					AuthType: constants.AuthTypeBearer,
					Endpoints: types.Endpoints{
						Models: constants.OpenaiModelsEndpoint,
					},
				},
				constants.AnthropicID: {
					ID:       constants.AnthropicID,
					Name:     constants.AnthropicDisplayName,
					URL:      server.URL,
					Token:    "test-token",
					AuthType: constants.AuthTypeXheader,
					Endpoints: types.Endpoints{
						Models: constants.AnthropicModelsEndpoint,
					},
				},
			}

			registry := registry.NewProviderRegistry(providerCfg, log)

			cfg := config.Config{
				DisallowedModels: tt.disallowedModels,
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

				var response types.ListModelsResponse
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

				var response types.ListModelsResponse
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

func TestChatCompletionsHandler_DisallowedModelValidation(t *testing.T) {
	tests := []struct {
		name             string
		disallowedModels string
		requestModel     string
		expectedStatus   int
		expectedError    string
		description      string
	}{
		{
			name:             "Non-disallowed model passes validation",
			disallowedModels: "gpt-3.5-turbo,claude-2",
			requestModel:     "openai/gpt-4",
			expectedStatus:   http.StatusOK,
			expectedError:    "",
			description:      "Should allow requests with models not in the disallowed list",
		},
		{
			name:             "Disallowed model fails validation",
			disallowedModels: "gpt-4,claude-3",
			requestModel:     "openai/gpt-4",
			expectedStatus:   http.StatusForbidden,
			expectedError:    "Model is disallowed",
			description:      "Should reject requests with models in the disallowed list",
		},
		{
			name:             "Empty disallowed models allows all",
			disallowedModels: "",
			requestModel:     "openai/gpt-4",
			expectedStatus:   http.StatusOK,
			expectedError:    "",
			description:      "Should allow all models when DISALLOWED_MODELS is empty",
		},
		{
			name:             "Case insensitive disallowed model validation",
			disallowedModels: "GPT-4,CLAUDE-3",
			requestModel:     "openai/gpt-4",
			expectedStatus:   http.StatusForbidden,
			expectedError:    "Model is disallowed",
			description:      "Should validate disallowed models in a case-insensitive manner",
		},
		{
			name:             "Model without provider prefix blocked",
			disallowedModels: "gpt-4",
			requestModel:     "openai/gpt-4",
			expectedStatus:   http.StatusForbidden,
			expectedError:    "Model is disallowed",
			description:      "Should block models when specified by name without provider",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)

				response := types.CreateChatCompletionResponse{
					ID:      "chatcmpl-123",
					Object:  "chat.completion",
					Created: 1677649963,
					Model:   "gpt-4",
					Choices: []types.ChatCompletionChoice{
						{
							Index:        0,
							Message:      types.NewTextMessage(t, types.Assistant, "Hello, how can I help you today?"),
							FinishReason: "stop",
						},
					},
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
					return http.DefaultClient.Post(server.URL+"/chat/completions", "application/json", req.Body)
				}).
				AnyTimes()

			log, err := logger.NewLogger("test")
			require.NoError(t, err)

			providerCfg := map[types.Provider]*registry.ProviderConfig{
				constants.OpenaiID: {
					ID:       constants.OpenaiID,
					Name:     constants.OpenaiDisplayName,
					URL:      server.URL,
					Token:    "test-token",
					AuthType: constants.AuthTypeBearer,
					Endpoints: types.Endpoints{
						Chat: constants.OpenaiChatEndpoint,
					},
				},
			}

			registry := registry.NewProviderRegistry(providerCfg, log)

			cfg := config.Config{
				DisallowedModels: tt.disallowedModels,
				Server: &config.ServerConfig{
					ReadTimeout: time.Duration(5000) * time.Millisecond,
				},
				Providers: providerCfg,
			}

			router := api.NewRouter(cfg, log, registry, mockClient, nil)

			gin.SetMode(gin.TestMode)
			r := gin.New()
			r.POST("/v1/chat/completions", router.ChatCompletionsHandler)

			requestBody := map[string]any{
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

			var response map[string]any
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

func TestChatCompletionsHandler_AllowedModelsTakesPrecedence(t *testing.T) {
	tests := []struct {
		name             string
		allowedModels    string
		disallowedModels string
		requestModel     string
		expectedStatus   int
		expectedError    string
		description      string
	}{
		{
			name:             "Allowed models takes precedence - model in both lists is allowed",
			allowedModels:    "gpt-4",
			disallowedModels: "gpt-4",
			requestModel:     "openai/gpt-4",
			expectedStatus:   http.StatusOK,
			expectedError:    "",
			description:      "When both are set and model is in both, allowed models takes precedence",
		},
		{
			name:             "Allowed models takes precedence - model only in allowed is allowed",
			allowedModels:    "gpt-4",
			disallowedModels: "gpt-3.5-turbo",
			requestModel:     "openai/gpt-4",
			expectedStatus:   http.StatusOK,
			expectedError:    "",
			description:      "When both are set, only allowed models list is checked",
		},
		{
			name:             "Allowed models takes precedence - model not in allowed is blocked",
			allowedModels:    "gpt-3.5-turbo",
			disallowedModels: "",
			requestModel:     "openai/gpt-4",
			expectedStatus:   http.StatusForbidden,
			expectedError:    "Model not allowed",
			description:      "When allowed models is set, models not in it are blocked even if disallowed is empty",
		},
		{
			name:             "Only disallowed models set - model in disallowed is blocked",
			allowedModels:    "",
			disallowedModels: "gpt-4",
			requestModel:     "openai/gpt-4",
			expectedStatus:   http.StatusForbidden,
			expectedError:    "Model is disallowed",
			description:      "When only disallowed models is set, models in it are blocked",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)

				response := types.CreateChatCompletionResponse{
					ID:      "chatcmpl-123",
					Object:  "chat.completion",
					Created: 1677649963,
					Model:   "gpt-4",
					Choices: []types.ChatCompletionChoice{
						{
							Index:        0,
							Message:      types.NewTextMessage(t, types.Assistant, "Hello, how can I help you today?"),
							FinishReason: "stop",
						},
					},
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
					return http.DefaultClient.Post(server.URL+"/chat/completions", "application/json", req.Body)
				}).
				AnyTimes()

			log, err := logger.NewLogger("test")
			require.NoError(t, err)

			providerCfg := map[types.Provider]*registry.ProviderConfig{
				constants.OpenaiID: {
					ID:       constants.OpenaiID,
					Name:     constants.OpenaiDisplayName,
					URL:      server.URL,
					Token:    "test-token",
					AuthType: constants.AuthTypeBearer,
					Endpoints: types.Endpoints{
						Chat: constants.OpenaiChatEndpoint,
					},
				},
			}

			registry := registry.NewProviderRegistry(providerCfg, log)

			cfg := config.Config{
				AllowedModels:    tt.allowedModels,
				DisallowedModels: tt.disallowedModels,
				Server: &config.ServerConfig{
					ReadTimeout: time.Duration(5000) * time.Millisecond,
				},
				Providers: providerCfg,
			}

			router := api.NewRouter(cfg, log, registry, mockClient, nil)

			gin.SetMode(gin.TestMode)
			r := gin.New()
			r.POST("/v1/chat/completions", router.ChatCompletionsHandler)

			requestBody := map[string]any{
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

			var response map[string]any
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

func TestChatCompletionsHandler_StreamingErrorHandling(t *testing.T) {
	tests := []struct {
		name               string
		providerError      error
		expectedStatusCode int
		expectedError      string
		description        string
	}{
		{
			name:               "Generic streaming error returns 400 with full error message",
			providerError:      assert.AnError,
			expectedStatusCode: http.StatusBadRequest,
			expectedError:      assert.AnError.Error(),
			description:        "Generic errors should return 400 with the full error message",
		},
		{
			name:               "HTTP 401 error is returned with correct status code",
			providerError:      &core.HTTPError{StatusCode: http.StatusUnauthorized, Message: `{"error":{"message":"authentication failed"}}`},
			expectedStatusCode: http.StatusUnauthorized,
			expectedError:      "authentication failed",
			description:        "HTTP 401 errors should return with correct status code",
		},
		{
			name:               "HTTP 403 error is returned with correct status code",
			providerError:      &core.HTTPError{StatusCode: http.StatusForbidden, Message: `{"error":{"message":"forbidden access"}}`},
			expectedStatusCode: http.StatusForbidden,
			expectedError:      "forbidden access",
			description:        "HTTP 403 errors should return with correct status code",
		},
		{
			name:               "HTTP 404 error is returned with correct status code",
			providerError:      &core.HTTPError{StatusCode: http.StatusNotFound, Message: `{"error":{"message":"model not found"}}`},
			expectedStatusCode: http.StatusNotFound,
			expectedError:      "model not found",
			description:        "HTTP 404 errors should return with correct status code",
		},
		{
			name:               "HTTP 429 error is returned with correct status code",
			providerError:      &core.HTTPError{StatusCode: http.StatusTooManyRequests, Message: `{"error":{"message":"rate limit exceeded"}}`},
			expectedStatusCode: http.StatusTooManyRequests,
			expectedError:      "rate limit exceeded",
			description:        "HTTP 429 errors should return with correct status code",
		},
		{
			name:               "HTTP 500 error is returned with correct status code",
			providerError:      &core.HTTPError{StatusCode: http.StatusInternalServerError, Message: `{"error":{"message":"internal server error"}}`},
			expectedStatusCode: http.StatusInternalServerError,
			expectedError:      "internal server error",
			description:        "HTTP 500 errors should return with correct status code",
		},
		{
			name:               "HTTP 502 error is returned with correct status code",
			providerError:      &core.HTTPError{StatusCode: http.StatusBadGateway, Message: `{"error":{"message":"bad gateway"}}`},
			expectedStatusCode: http.StatusBadGateway,
			expectedError:      "bad gateway",
			description:        "HTTP 502 errors should return with correct status code",
		},
		{
			name:               "HTTP 503 error is returned with correct status code",
			providerError:      &core.HTTPError{StatusCode: http.StatusServiceUnavailable, Message: `{"error":{"message":"service unavailable"}}`},
			expectedStatusCode: http.StatusServiceUnavailable,
			expectedError:      "service unavailable",
			description:        "HTTP 503 errors should return with correct status code",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockProvider := providersmocks.NewMockIProvider(ctrl)
			mockClient := providersmocks.NewMockClient(ctrl)
			mockRegistry := providersmocks.NewMockProviderRegistry(ctrl)

			log, err := logger.NewLogger("test")
			require.NoError(t, err)

			providerCfg := map[types.Provider]*registry.ProviderConfig{
				constants.OpenaiID: {
					ID:       constants.OpenaiID,
					Name:     constants.OpenaiDisplayName,
					URL:      "http://localhost:8080",
					Token:    "test-token",
					AuthType: constants.AuthTypeBearer,
					Endpoints: types.Endpoints{
						Chat: constants.OpenaiChatEndpoint,
					},
				},
			}

			cfg := config.Config{
				Server: &config.ServerConfig{
					ReadTimeout: time.Duration(5000) * time.Millisecond,
				},
				Providers: providerCfg,
			}

			mockProvider.EXPECT().
				StreamChatCompletions(gomock.Any(), gomock.Any()).
				Return(nil, tt.providerError)

			mockRegistry.EXPECT().
				BuildProvider(constants.OpenaiID, mockClient).
				Return(mockProvider, nil)

			router := api.NewRouter(cfg, log, mockRegistry, mockClient, nil)

			gin.SetMode(gin.TestMode)
			r := gin.New()
			r.POST("/v1/chat/completions", router.ChatCompletionsHandler)

			stream := true
			requestBody := types.CreateChatCompletionRequest{
				Model:  "openai/gpt-4",
				Stream: &stream,
				Messages: []types.Message{
					types.NewTextMessage(t, types.User, "Hello, world!"),
				},
			}

			jsonBody, err := json.Marshal(requestBody)
			require.NoError(t, err)

			w := httptest.NewRecorder()
			req, err := http.NewRequest("POST", "/v1/chat/completions", strings.NewReader(string(jsonBody)))
			require.NoError(t, err)
			req.Header.Set("Content-Type", "application/json")

			r.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatusCode, w.Code, "Expected status code %d but got %d", tt.expectedStatusCode, w.Code)

			var response map[string]any
			err = json.Unmarshal(w.Body.Bytes(), &response)
			require.NoError(t, err)

			errorMsg, exists := response["error"]
			assert.True(t, exists, "Response should contain error field")
			assert.Contains(t, errorMsg.(string), tt.expectedError, "Error message should contain expected text")
		})
	}
}
