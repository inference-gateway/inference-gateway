package tests

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	gin "github.com/gin-gonic/gin"
	a2a "github.com/inference-gateway/inference-gateway/a2a"
	api "github.com/inference-gateway/inference-gateway/api"
	config "github.com/inference-gateway/inference-gateway/config"
	logger "github.com/inference-gateway/inference-gateway/logger"
	providers "github.com/inference-gateway/inference-gateway/providers"
	a2amocks "github.com/inference-gateway/inference-gateway/tests/mocks/a2a"
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
		name           string
		allowedModels  string
		mockModels     []providers.Model
		expectedModels []string
		description    string
	}{
		{
			name:          "Empty ALLOWED_MODELS returns all models",
			allowedModels: "",
			mockModels: []providers.Model{
				{ID: "openai/gpt-4", Object: "model", Created: 1677649963, OwnedBy: "openai", ServedBy: providers.OpenaiID},
				{ID: "openai/gpt-3.5-turbo", Object: "model", Created: 1677610602, OwnedBy: "openai", ServedBy: providers.OpenaiID},
				{ID: "anthropic/claude-3", Object: "model", Created: 1677649963, OwnedBy: "anthropic", ServedBy: providers.AnthropicID},
			},
			expectedModels: []string{"openai/gpt-4", "openai/gpt-3.5-turbo", "anthropic/claude-3"},
			description:    "When MODELS_LIST is empty, all models should be returned",
		},
		{
			name:          "Filter by exact model ID",
			allowedModels: "openai/gpt-4",
			mockModels: []providers.Model{
				{ID: "openai/gpt-4", Object: "model", Created: 1677649963, OwnedBy: "openai", ServedBy: providers.OpenaiID},
				{ID: "openai/gpt-3.5-turbo", Object: "model", Created: 1677610602, OwnedBy: "openai", ServedBy: providers.OpenaiID},
				{ID: "anthropic/claude-3", Object: "model", Created: 1677649963, OwnedBy: "anthropic", ServedBy: providers.AnthropicID},
			},
			expectedModels: []string{"openai/gpt-4"},
			description:    "Should return only the exact model ID match",
		},
		{
			name:          "Filter by model name without provider prefix",
			allowedModels: "gpt-4,claude-3",
			mockModels: []providers.Model{
				{ID: "openai/gpt-4", Object: "model", Created: 1677649963, OwnedBy: "openai", ServedBy: providers.OpenaiID},
				{ID: "openai/gpt-3.5-turbo", Object: "model", Created: 1677610602, OwnedBy: "openai", ServedBy: providers.OpenaiID},
				{ID: "anthropic/claude-3", Object: "model", Created: 1677649963, OwnedBy: "anthropic", ServedBy: providers.AnthropicID},
			},
			expectedModels: []string{"openai/gpt-4", "anthropic/claude-3"},
			description:    "Should match models by name without provider prefix",
		},
		{
			name:          "Case insensitive matching",
			allowedModels: "GPT-4,CLAUDE-3",
			mockModels: []providers.Model{
				{ID: "openai/gpt-4", Object: "model", Created: 1677649963, OwnedBy: "openai", ServedBy: providers.OpenaiID},
				{ID: "openai/gpt-3.5-turbo", Object: "model", Created: 1677610602, OwnedBy: "openai", ServedBy: providers.OpenaiID},
				{ID: "anthropic/claude-3", Object: "model", Created: 1677649963, OwnedBy: "anthropic", ServedBy: providers.AnthropicID},
			},
			expectedModels: []string{"openai/gpt-4", "anthropic/claude-3"},
			description:    "Should match models in a case-insensitive manner",
		},
		{
			name:          "Trim whitespace in ALLOWED_MODELS",
			allowedModels: " gpt-4 , claude-3 ",
			mockModels: []providers.Model{
				{ID: "openai/gpt-4", Object: "model", Created: 1677649963, OwnedBy: "openai", ServedBy: providers.OpenaiID},
				{ID: "openai/gpt-3.5-turbo", Object: "model", Created: 1677610602, OwnedBy: "openai", ServedBy: providers.OpenaiID},
				{ID: "anthropic/claude-3", Object: "model", Created: 1677649963, OwnedBy: "anthropic", ServedBy: providers.AnthropicID},
			},
			expectedModels: []string{"openai/gpt-4", "anthropic/claude-3"},
			description:    "Should handle whitespace correctly in the models list",
		},
		{
			name:          "No matches returns empty list",
			allowedModels: "nonexistent-model",
			mockModels: []providers.Model{
				{ID: "openai/gpt-4", Object: "model", Created: 1677649963, OwnedBy: "openai", ServedBy: providers.OpenaiID},
				{ID: "openai/gpt-3.5-turbo", Object: "model", Created: 1677610602, OwnedBy: "openai", ServedBy: providers.OpenaiID},
			},
			expectedModels: []string{},
			description:    "Should return empty list when no models match the filter",
		},
		{
			name:          "Mixed exact ID and name matching",
			allowedModels: "openai/gpt-4,claude-3",
			mockModels: []providers.Model{
				{ID: "openai/gpt-4", Object: "model", Created: 1677649963, OwnedBy: "openai", ServedBy: providers.OpenaiID},
				{ID: "openai/gpt-3.5-turbo", Object: "model", Created: 1677610602, OwnedBy: "openai", ServedBy: providers.OpenaiID},
				{ID: "anthropic/claude-3", Object: "model", Created: 1677649963, OwnedBy: "anthropic", ServedBy: providers.AnthropicID},
			},
			expectedModels: []string{"openai/gpt-4", "anthropic/claude-3"},
			description:    "Should handle mix of exact ID and name-only matching",
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
			}

			registry := providers.NewProviderRegistry(providerCfg, log)

			cfg := config.Config{
				AllowedModels: tt.allowedModels,
				Server: &config.ServerConfig{
					ReadTimeout: time.Duration(5000) * time.Millisecond,
				},
				Providers: providerCfg,
			}

			router := api.NewRouter(cfg, log, registry, mockClient, nil, nil)

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
				assert.Equal(t, len(tt.expectedModels), len(response.Data))

				actualModelIDs := make([]string, len(response.Data))
				for i, model := range response.Data {
					actualModelIDs[i] = model.ID
				}

				for _, expectedID := range tt.expectedModels {
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
				assert.Equal(t, len(tt.expectedModels), len(response.Data))

				actualModelIDs := make([]string, len(response.Data))
				for i, model := range response.Data {
					actualModelIDs[i] = model.ID
				}

				for _, expectedID := range tt.expectedModels {
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

			router := api.NewRouter(cfg, log, registry, mockClient, nil, nil)

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

			router := api.NewRouter(cfg, log, registry, mockClient, nil, nil)

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

func TestListAgentsHandler(t *testing.T) {
	tests := []struct {
		name                 string
		a2aExpose            bool
		a2aClientNil         bool
		a2aClientInitialized bool
		agentURLs            []string
		agentCards           map[string]*a2a.AgentCard
		agentCardErrors      map[string]error
		expectedStatus       int
		expectedError        string
		expectedAgentCount   int
		description          string
	}{
		{
			name:           "A2A not exposed returns 403",
			a2aExpose:      false,
			expectedStatus: http.StatusForbidden,
			expectedError:  "A2A agents endpoint is not exposed",
			description:    "When A2A_EXPOSE is false, should return 403 Forbidden",
		},
		{
			name:               "A2A client nil returns empty list",
			a2aExpose:          true,
			a2aClientNil:       true,
			expectedStatus:     http.StatusOK,
			expectedAgentCount: 0,
			description:        "When A2A client is nil, should return empty agents list",
		},
		{
			name:                 "A2A client not initialized returns empty list",
			a2aExpose:            true,
			a2aClientInitialized: false,
			expectedStatus:       http.StatusOK,
			expectedAgentCount:   0,
			description:          "When A2A client is not initialized, should return empty agents list",
		},
		{
			name:                 "No agents configured returns empty list",
			a2aExpose:            true,
			a2aClientInitialized: true,
			agentURLs:            []string{},
			expectedStatus:       http.StatusOK,
			expectedAgentCount:   0,
			description:          "When no agents are configured, should return empty agents list",
		},
		{
			name:                 "Single agent returns successfully",
			a2aExpose:            true,
			a2aClientInitialized: true,
			agentURLs:            []string{"https://agent1.example.com"},
			agentCards: map[string]*a2a.AgentCard{
				"https://agent1.example.com": {
					Name:        "Calculator Agent",
					Description: "An agent that can perform mathematical calculations",
					URL:         "https://agent1.example.com",
				},
			},
			expectedStatus:     http.StatusOK,
			expectedAgentCount: 1,
			description:        "Single agent should be returned successfully",
		},
		{
			name:                 "Multiple agents return successfully",
			a2aExpose:            true,
			a2aClientInitialized: true,
			agentURLs:            []string{"https://agent1.example.com", "https://agent2.example.com"},
			agentCards: map[string]*a2a.AgentCard{
				"https://agent1.example.com": {
					Name:        "Calculator Agent",
					Description: "An agent that can perform mathematical calculations",
					URL:         "https://agent1.example.com",
				},
				"https://agent2.example.com": {
					Name:        "Weather Agent",
					Description: "An agent that provides weather information",
					URL:         "https://agent2.example.com",
				},
			},
			expectedStatus:     http.StatusOK,
			expectedAgentCount: 2,
			description:        "Multiple agents should be returned successfully",
		},
		{
			name:                 "Failed agent card retrieval skips agent",
			a2aExpose:            true,
			a2aClientInitialized: true,
			agentURLs:            []string{"https://agent1.example.com", "https://agent2.example.com"},
			agentCards: map[string]*a2a.AgentCard{
				"https://agent1.example.com": {
					Name:        "Calculator Agent",
					Description: "An agent that can perform mathematical calculations",
					URL:         "https://agent1.example.com",
				},
			},
			agentCardErrors: map[string]error{
				"https://agent2.example.com": assert.AnError,
			},
			expectedStatus:     http.StatusOK,
			expectedAgentCount: 1,
			description:        "Should skip agents with card retrieval errors and continue with successful ones",
		},
		{
			name:                 "All agents fail to retrieve cards returns empty list",
			a2aExpose:            true,
			a2aClientInitialized: true,
			agentURLs:            []string{"https://agent1.example.com", "https://agent2.example.com"},
			agentCardErrors: map[string]error{
				"https://agent1.example.com": assert.AnError,
				"https://agent2.example.com": assert.AnError,
			},
			expectedStatus:     http.StatusOK,
			expectedAgentCount: 0,
			description:        "Should return empty list when all agent card retrievals fail",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockRegistry := providersmocks.NewMockProviderRegistry(ctrl)
			mockClient := providersmocks.NewMockClient(ctrl)
			var mockA2AClient *a2amocks.MockA2AClientInterface

			log, err := logger.NewLogger("test")
			require.NoError(t, err)

			if !tt.a2aClientNil {
				mockA2AClient = a2amocks.NewMockA2AClientInterface(ctrl)

				mockA2AClient.EXPECT().
					IsInitialized().
					Return(tt.a2aClientInitialized).
					AnyTimes()

				if tt.a2aClientInitialized {
					mockA2AClient.EXPECT().
						GetAgents().
						Return(tt.agentURLs).
						AnyTimes()

					for _, agentURL := range tt.agentURLs {
						if agentCard, exists := tt.agentCards[agentURL]; exists {
							mockAgentCard := &a2a.AgentCard{
								Name:        agentCard.Name,
								Description: agentCard.Description,
							}
							mockA2AClient.EXPECT().
								GetAgentCard(gomock.Any(), agentURL).
								Return(mockAgentCard, nil).
								Times(1)
						} else if err, hasError := tt.agentCardErrors[agentURL]; hasError {
							mockA2AClient.EXPECT().
								GetAgentCard(gomock.Any(), agentURL).
								Return(nil, err).
								Times(1)
						}
					}
				}
			}

			cfg := config.Config{
				A2A: &config.A2AConfig{
					Expose: tt.a2aExpose,
				},
				Server: &config.ServerConfig{
					ReadTimeout: time.Duration(5000) * time.Millisecond,
				},
			}

			var router api.Router
			if tt.a2aClientNil {
				router = api.NewRouter(cfg, log, mockRegistry, mockClient, nil, nil)
			} else {
				router = api.NewRouter(cfg, log, mockRegistry, mockClient, nil, mockA2AClient)
			}

			gin.SetMode(gin.TestMode)
			r := gin.New()
			r.GET("/a2a/agents", router.ListAgentsHandler)

			w := httptest.NewRecorder()
			req, err := http.NewRequest("GET", "/a2a/agents", nil)
			require.NoError(t, err)

			r.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code, "HTTP status code should match expected")

			var response map[string]interface{}
			err = json.Unmarshal(w.Body.Bytes(), &response)
			require.NoError(t, err, "Response should be valid JSON")

			if tt.expectedError != "" {
				errorMsg, exists := response["error"]
				assert.True(t, exists, "Response should contain error field")
				assert.Contains(t, errorMsg.(string), tt.expectedError, "Error message should contain expected text")
			} else {
				assert.Equal(t, "list", response["object"], "Response object should be 'list'")

				data, exists := response["data"]
				assert.True(t, exists, "Response should contain data field")

				dataArray, ok := data.([]interface{})
				assert.True(t, ok, "Data field should be an array")
				assert.Equal(t, tt.expectedAgentCount, len(dataArray), "Number of agents should match expected")

				if tt.expectedAgentCount > 0 {
					for i, agentInterface := range dataArray {
						agent, ok := agentInterface.(map[string]interface{})
						assert.True(t, ok, "Agent should be an object")

						assert.NotEmpty(t, agent["id"], "Agent %d should have non-empty id", i)
						assert.NotEmpty(t, agent["name"], "Agent %d should have non-empty name", i)

						_, hasDescription := agent["description"]
						assert.True(t, hasDescription, "Agent %d should have description field", i)

						_, hasUrl := agent["url"]
						assert.True(t, hasUrl, "Agent %d should have url field", i)
					}
				}
			}
		})
	}
}
