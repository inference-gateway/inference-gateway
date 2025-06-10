package middleware_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/inference-gateway/inference-gateway/a2a"
	"github.com/inference-gateway/inference-gateway/api/middlewares"
	"github.com/inference-gateway/inference-gateway/config"
	"github.com/inference-gateway/inference-gateway/providers"
	"github.com/inference-gateway/inference-gateway/tests/mocks"
	a2amocks "github.com/inference-gateway/inference-gateway/tests/mocks/a2a"
	providersmocks "github.com/inference-gateway/inference-gateway/tests/mocks/providers"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

func TestNewA2AMiddleware(t *testing.T) {
	tests := []struct {
		name       string
		a2aEnabled bool
		expectNoop bool
	}{
		{
			name:       "A2A disabled returns noop middleware",
			a2aEnabled: false,
			expectNoop: true,
		},
		{
			name:       "A2A enabled returns full middleware",
			a2aEnabled: true,
			expectNoop: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockRegistry := providersmocks.NewMockProviderRegistry(ctrl)
			mockA2AClient := a2amocks.NewMockA2AClientInterface(ctrl)
			mockA2AAgent := a2amocks.NewMockAgent(ctrl)
			mockLogger := mocks.NewMockLogger(ctrl)
			mockInferenceClient := providersmocks.NewMockClient(ctrl)

			if tt.a2aEnabled {
				mockLogger.EXPECT().Debug(gomock.Any(), gomock.Any()).AnyTimes()
				mockA2AClient.EXPECT().IsInitialized().Return(false)
			}

			cfg := config.Config{
				A2A: &config.A2AConfig{
					Enable: tt.a2aEnabled,
				},
			}

			middleware, err := middlewares.NewA2AMiddleware(mockRegistry, mockA2AClient, mockA2AAgent, mockLogger, mockInferenceClient, cfg)

			assert.NoError(t, err)
			assert.NotNil(t, middleware)

			ginHandler := middleware.Middleware()
			assert.NotNil(t, ginHandler)

			gin.SetMode(gin.TestMode)
			router := gin.New()
			router.Use(ginHandler)
			router.POST("/v1/chat/completions", func(c *gin.Context) {
				c.JSON(http.StatusOK, gin.H{"test": "passed"})
			})

			req := httptest.NewRequest("POST", "/v1/chat/completions", bytes.NewReader([]byte(`{"model": "test", "messages": []}`)))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			if tt.expectNoop {
				assert.Equal(t, http.StatusOK, w.Code)
			}
		})
	}
}

func TestA2AMiddleware_RequestWithA2AMiddlewareEnabled(t *testing.T) {
	tests := []struct {
		name                 string
		path                 string
		hasInternalHeader    bool
		a2aClientInitialized bool
		expectA2AProcessing  bool
		requestBody          string
	}{
		{
			name:                 "Internal A2A call skips middleware",
			path:                 "/v1/chat/completions",
			hasInternalHeader:    true,
			a2aClientInitialized: true,
			expectA2AProcessing:  false,
			requestBody:          `{"model": "openai/gpt-4", "messages": []}`,
		},
		{
			name:                 "Non-chat endpoint skips middleware",
			path:                 "/v1/models",
			hasInternalHeader:    false,
			a2aClientInitialized: true,
			expectA2AProcessing:  false,
			requestBody:          `{}`,
		},
		{
			name:                 "A2A client not initialized skips processing",
			path:                 "/v1/chat/completions",
			hasInternalHeader:    false,
			a2aClientInitialized: false,
			expectA2AProcessing:  false,
			requestBody:          `{"model": "openai/gpt-4", "messages": []}`,
		},
		{
			name:                 "Valid A2A request gets processed",
			path:                 "/v1/chat/completions",
			hasInternalHeader:    false,
			a2aClientInitialized: true,
			expectA2AProcessing:  true,
			requestBody:          `{"model": "openai/gpt-4", "messages": [{"role": "user", "content": "Hello"}]}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockRegistry := providersmocks.NewMockProviderRegistry(ctrl)
			mockA2AClient := a2amocks.NewMockA2AClientInterface(ctrl)
			mockA2AAgent := a2amocks.NewMockAgent(ctrl)
			mockLogger := mocks.NewMockLogger(ctrl)
			mockInferenceClient := providersmocks.NewMockClient(ctrl)
			mockProvider := providersmocks.NewMockIProvider(ctrl)

			mockLogger.EXPECT().Debug(gomock.Any(), gomock.Any()).AnyTimes()
			mockLogger.EXPECT().Debug(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
			mockLogger.EXPECT().Error(gomock.Any(), gomock.Any()).AnyTimes()

			if tt.expectA2AProcessing {
				mockA2AClient.EXPECT().IsInitialized().Return(tt.a2aClientInitialized)
				mockA2AClient.EXPECT().GetAgents().Return([]string{})
				mockRegistry.EXPECT().BuildProvider(providers.OpenaiID, mockInferenceClient).Return(mockProvider, nil)
			} else if tt.path == "/v1/chat/completions" && !tt.hasInternalHeader {
				mockA2AClient.EXPECT().IsInitialized().Return(tt.a2aClientInitialized)
			}

			cfg := config.Config{
				A2A: &config.A2AConfig{
					Enable: true,
				},
			}

			middleware, err := middlewares.NewA2AMiddleware(mockRegistry, mockA2AClient, mockA2AAgent, mockLogger, mockInferenceClient, cfg)
			assert.NoError(t, err)

			gin.SetMode(gin.TestMode)
			router := gin.New()
			router.Use(middleware.Middleware())

			handlerCalled := false
			router.Any(tt.path, func(c *gin.Context) {
				handlerCalled = true
				c.JSON(http.StatusOK, gin.H{"test": "passed"})
			})

			req := httptest.NewRequest("POST", tt.path, bytes.NewReader([]byte(tt.requestBody)))
			req.Header.Set("Content-Type", "application/json")
			if tt.hasInternalHeader {
				req.Header.Set("X-A2A-Internal", "true")
			}
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			assert.True(t, handlerCalled, "Handler should be called")
			assert.Equal(t, http.StatusOK, w.Code)
		})
	}
}

func TestA2AMiddleware_AgentsAreInjectedAsTools(t *testing.T) {
	tests := []struct {
		name                 string
		a2aClientInitialized bool
		agents               []string
		agentSkills          map[string][]a2a.AgentSkill
		expectToolsAdded     bool
		expectedToolCount    int
	}{
		{
			name:                 "No agents available - only agent query tool added",
			a2aClientInitialized: true,
			agents:               []string{},
			agentSkills:          map[string][]a2a.AgentSkill{},
			expectToolsAdded:     true,
			expectedToolCount:    1,
		},
		{
			name:                 "Single agent with skills - only query tool added initially",
			a2aClientInitialized: true,
			agents:               []string{"http://agent1.example.com"},
			agentSkills: map[string][]a2a.AgentSkill{
				"http://agent1.example.com": {
					{ID: "add", Name: "Add Numbers", Description: "Add two numbers"},
					{ID: "multiply", Name: "Multiply Numbers", Description: "Multiply two numbers"},
				},
			},
			expectToolsAdded:  true,
			expectedToolCount: 1,
		},
		{
			name:                 "Multiple agents with skills - only query tool added initially",
			a2aClientInitialized: true,
			agents:               []string{"http://agent1.example.com", "http://agent2.example.com"},
			agentSkills: map[string][]a2a.AgentSkill{
				"http://agent1.example.com": {
					{ID: "calculate", Name: "Calculate", Description: "Mathematical calculations"},
				},
				"http://agent2.example.com": {
					{ID: "weather", Name: "Weather", Description: "Get weather information"},
				},
			},
			expectToolsAdded:  true,
			expectedToolCount: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockRegistry := providersmocks.NewMockProviderRegistry(ctrl)
			mockA2AClient := a2amocks.NewMockA2AClientInterface(ctrl)
			mockA2AAgent := a2amocks.NewMockAgent(ctrl)
			mockLogger := mocks.NewMockLogger(ctrl)
			mockInferenceClient := providersmocks.NewMockClient(ctrl)
			mockProvider := providersmocks.NewMockIProvider(ctrl)

			mockLogger.EXPECT().Debug(gomock.Any(), gomock.Any()).AnyTimes()
			mockLogger.EXPECT().Debug(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
			mockLogger.EXPECT().Error(gomock.Any(), gomock.Any()).AnyTimes()

			mockA2AClient.EXPECT().IsInitialized().Return(tt.a2aClientInitialized)
			if tt.a2aClientInitialized {
				mockA2AClient.EXPECT().GetAgents().Return(tt.agents)
				// Note: GetAgentSkills is no longer called upfront due to progressive discovery
				mockRegistry.EXPECT().BuildProvider(providers.OpenaiID, mockInferenceClient).Return(mockProvider, nil)
				// Add mock expectations for agent calls that may happen during middleware processing
				mockA2AAgent.EXPECT().SetProvider(mockProvider).AnyTimes()
				mockA2AAgent.EXPECT().SetModel(gomock.Any()).AnyTimes()
				mockA2AAgent.EXPECT().Run(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
			}

			cfg := config.Config{
				A2A: &config.A2AConfig{
					Enable: true,
				},
			}

			middleware, err := middlewares.NewA2AMiddleware(mockRegistry, mockA2AClient, mockA2AAgent, mockLogger, mockInferenceClient, cfg)
			assert.NoError(t, err)

			gin.SetMode(gin.TestMode)
			router := gin.New()
			router.Use(middleware.Middleware())

			var capturedRequest *providers.CreateChatCompletionRequest
			router.POST("/v1/chat/completions", func(c *gin.Context) {
				var req providers.CreateChatCompletionRequest
				if err := c.ShouldBindJSON(&req); err == nil {
					capturedRequest = &req
				}

				response := providers.CreateChatCompletionResponse{
					ID:    "test-id",
					Model: "gpt-4",
					Choices: []providers.ChatCompletionChoice{
						{
							Message: providers.Message{
								Role:    providers.MessageRoleAssistant,
								Content: "Hello! How can I help you?",
							},
							FinishReason: providers.FinishReasonStop,
						},
					},
				}
				c.JSON(http.StatusOK, response)
			})

			requestData := providers.CreateChatCompletionRequest{
				Model: "openai/gpt-4",
				Messages: []providers.Message{
					{Role: providers.MessageRoleUser, Content: "Hello"},
				},
			}

			requestBody, _ := json.Marshal(requestData)
			req := httptest.NewRequest("POST", "/v1/chat/completions", bytes.NewReader(requestBody))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusOK, w.Code)

			if tt.expectToolsAdded && capturedRequest != nil {
				assert.NotNil(t, capturedRequest.Tools, "Tools should be added to the request")
				assert.Equal(t, tt.expectedToolCount, len(*capturedRequest.Tools), "Expected number of tools should be added")

				foundAgentQueryTool := false
				for _, tool := range *capturedRequest.Tools {
					if tool.Function.Name == "query_a2a_agent_card" {
						foundAgentQueryTool = true
						break
					}
				}
				assert.True(t, foundAgentQueryTool, "Agent query tool should be present")
			}
		})
	}
}

func TestA2AMiddleware_LLMDecisionToSubmitTask(t *testing.T) {
	tests := []struct {
		name                 string
		toolCalls            []providers.ChatCompletionMessageToolCall
		availableAgents      []string
		agentSkills          map[string][]a2a.AgentSkill
		sendMessageResp      *a2a.SendMessageSuccessResponse
		sendMessageError     error
		expectTaskSubmission bool
		expectError          bool
	}{
		{
			name: "LLM calls agent query tool",
			toolCalls: []providers.ChatCompletionMessageToolCall{
				{
					ID: "call_1",
					Function: providers.ChatCompletionMessageToolCallFunction{
						Name:      "query_a2a_agent_card",
						Arguments: `{"agent_url": "http://agent1.example.com"}`,
					},
				},
			},
			availableAgents: []string{"http://agent1.example.com"},
			agentSkills: map[string][]a2a.AgentSkill{
				"http://agent1.example.com": {
					{ID: "calculate", Name: "Calculator", Description: "Mathematical calculations"},
				},
			},
			expectTaskSubmission: false,
			expectError:          false,
		},
		{
			name: "LLM calls actual agent skill without query - should not work with progressive discovery",
			toolCalls: []providers.ChatCompletionMessageToolCall{
				{
					ID: "call_2",
					Function: providers.ChatCompletionMessageToolCallFunction{
						Name:      "calculate",
						Arguments: `{"operation": "add", "a": 5, "b": 3}`,
					},
				},
			},
			availableAgents: []string{"http://agent1.example.com"},
			agentSkills: map[string][]a2a.AgentSkill{
				"http://agent1.example.com": {
					{ID: "calculate", Name: "Calculator", Description: "Mathematical calculations"},
				},
			},
			expectTaskSubmission: false,
			expectError:          false,
		},
		{
			name: "Task submission fails",
			toolCalls: []providers.ChatCompletionMessageToolCall{
				{
					ID: "call_3",
					Function: providers.ChatCompletionMessageToolCallFunction{
						Name:      "calculate",
						Arguments: `{"operation": "add", "a": 5, "b": 3}`,
					},
				},
			},
			availableAgents: []string{"http://agent1.example.com"},
			agentSkills: map[string][]a2a.AgentSkill{
				"http://agent1.example.com": {
					{ID: "calculate", Name: "Calculator", Description: "Mathematical calculations"},
				},
			},
			sendMessageError:     fmt.Errorf("agent connection failed"),
			expectTaskSubmission: false,
			expectError:          false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockRegistry := providersmocks.NewMockProviderRegistry(ctrl)
			mockA2AClient := a2amocks.NewMockA2AClientInterface(ctrl)
			mockA2AAgent := a2amocks.NewMockAgent(ctrl)
			mockLogger := mocks.NewMockLogger(ctrl)
			mockInferenceClient := providersmocks.NewMockClient(ctrl)
			mockProvider := providersmocks.NewMockIProvider(ctrl)

			mockLogger.EXPECT().Debug(gomock.Any(), gomock.Any()).AnyTimes()
			mockLogger.EXPECT().Debug(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
			mockLogger.EXPECT().Debug(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
			mockLogger.EXPECT().Debug(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
			mockLogger.EXPECT().Debug(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
			mockLogger.EXPECT().Error(gomock.Any(), gomock.Any()).AnyTimes()
			mockLogger.EXPECT().Error(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
			mockLogger.EXPECT().Error(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
			mockLogger.EXPECT().Warn(gomock.Any(), gomock.Any()).AnyTimes()
			mockLogger.EXPECT().Warn(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

			mockA2AClient.EXPECT().IsInitialized().Return(true)
			mockA2AClient.EXPECT().GetAgents().Return(tt.availableAgents).AnyTimes()

			for agentURL, skills := range tt.agentSkills {
				mockA2AClient.EXPECT().GetAgentSkills(agentURL).Return(skills, nil).AnyTimes()
			}

			mockRegistry.EXPECT().BuildProvider(providers.OpenaiID, mockInferenceClient).Return(mockProvider, nil)

			for _, toolCall := range tt.toolCalls {
				if toolCall.Function.Name == "query_a2a_agent_card" {
					mockA2AClient.EXPECT().GetAgentCard(gomock.Any(), gomock.Any()).Return(&a2a.AgentCard{
						Name:        "Test Agent",
						Description: "Test Description",
						Version:     "1.0",
						Skills:      tt.agentSkills["http://agent1.example.com"],
					}, nil).AnyTimes()
					mockA2AClient.EXPECT().GetAgentCapabilities().Return(map[string]a2a.AgentCapabilities{
						"http://agent1.example.com": {
							PushNotifications:      boolPtr(false),
							StateTransitionHistory: boolPtr(false),
							Streaming:              boolPtr(false),
						},
					}).AnyTimes()
				}
			}

			if tt.expectTaskSubmission {
				if tt.sendMessageError != nil {
					mockA2AClient.EXPECT().SendMessage(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, tt.sendMessageError).AnyTimes()
				} else if tt.sendMessageResp != nil {
					mockA2AClient.EXPECT().SendMessage(gomock.Any(), gomock.Any(), gomock.Any()).Return(tt.sendMessageResp, nil).AnyTimes()
				}
			}

			mockA2AAgent.EXPECT().SetProvider(mockProvider).AnyTimes()
			mockA2AAgent.EXPECT().SetModel(gomock.Any()).AnyTimes()
			mockA2AAgent.EXPECT().Run(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

			cfg := config.Config{
				A2A: &config.A2AConfig{
					Enable: true,
				},
			}

			middleware, err := middlewares.NewA2AMiddleware(mockRegistry, mockA2AClient, mockA2AAgent, mockLogger, mockInferenceClient, cfg)
			assert.NoError(t, err)

			gin.SetMode(gin.TestMode)
			router := gin.New()
			router.Use(middleware.Middleware())

			router.POST("/v1/chat/completions", func(c *gin.Context) {
				response := providers.CreateChatCompletionResponse{
					ID:    "test-id",
					Model: "gpt-4",
					Choices: []providers.ChatCompletionChoice{
						{
							Message: providers.Message{
								Role:      providers.MessageRoleAssistant,
								Content:   "I'll help you with that task.",
								ToolCalls: &tt.toolCalls,
							},
							FinishReason: providers.FinishReasonToolCalls,
						},
					},
				}
				c.JSON(http.StatusOK, response)
			})

			requestData := providers.CreateChatCompletionRequest{
				Model: "openai/gpt-4",
				Messages: []providers.Message{
					{Role: providers.MessageRoleUser, Content: "Can you help me calculate 5+3?"},
				},
			}

			requestBody, _ := json.Marshal(requestData)
			req := httptest.NewRequest("POST", "/v1/chat/completions", bytes.NewReader(requestBody))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusOK, w.Code)

			var response map[string]interface{}
			err = json.Unmarshal(w.Body.Bytes(), &response)
			assert.NoError(t, err)
		})
	}
}

func TestA2AMiddleware_TaskSuccessfulExecution(t *testing.T) {
	tests := []struct {
		name               string
		isStreaming        bool
		taskID             string
		taskStatus         string
		taskResult         string
		pollSuccessful     bool
		expectStreamEvents bool
	}{
		{
			name:               "Synchronous task execution success",
			isStreaming:        false,
			taskID:             "task_123",
			taskStatus:         string(a2a.TaskStateCompleted),
			taskResult:         "Result: 8",
			pollSuccessful:     true,
			expectStreamEvents: false,
		},
		{
			name:               "Asynchronous task execution with streaming",
			isStreaming:        true,
			taskID:             "task_456",
			taskStatus:         string(a2a.TaskStateCompleted),
			taskResult:         "Result: 15",
			pollSuccessful:     true,
			expectStreamEvents: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockRegistry := providersmocks.NewMockProviderRegistry(ctrl)
			mockA2AClient := a2amocks.NewMockA2AClientInterface(ctrl)
			mockA2AAgent := a2amocks.NewMockAgent(ctrl)
			mockLogger := mocks.NewMockLogger(ctrl)
			mockInferenceClient := providersmocks.NewMockClient(ctrl)
			mockProvider := providersmocks.NewMockIProvider(ctrl)

			mockLogger.EXPECT().Debug(gomock.Any(), gomock.Any()).AnyTimes()
			mockLogger.EXPECT().Debug(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
			mockLogger.EXPECT().Debug(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
			mockLogger.EXPECT().Debug(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
			mockLogger.EXPECT().Debug(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
			mockLogger.EXPECT().Error(gomock.Any(), gomock.Any()).AnyTimes()
			mockLogger.EXPECT().Warn(gomock.Any(), gomock.Any()).AnyTimes()

			mockA2AClient.EXPECT().IsInitialized().Return(true)
			mockA2AClient.EXPECT().GetAgents().Return([]string{"http://agent1.example.com"}).AnyTimes()

			mockRegistry.EXPECT().BuildProvider(providers.OpenaiID, mockInferenceClient).Return(mockProvider, nil)
			mockA2AAgent.EXPECT().SetProvider(mockProvider).AnyTimes()
			mockA2AAgent.EXPECT().SetModel(gomock.Any()).AnyTimes()
			mockA2AAgent.EXPECT().Run(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

			sendMessageResp := &a2a.SendMessageSuccessResponse{
				Result: a2a.Task{
					ID: tt.taskID,
				},
			}
			mockA2AClient.EXPECT().SendMessage(gomock.Any(), gomock.Any(), "http://agent1.example.com").Return(sendMessageResp, nil).AnyTimes()

			if tt.pollSuccessful {
				getTaskResp := &a2a.GetTaskSuccessResponse{
					Result: a2a.Task{
						Status: a2a.TaskStatus{
							State: a2a.TaskState(tt.taskStatus),
							Message: &a2a.Message{
								Kind:      "message",
								MessageID: "msg-123",
								Role:      "assistant",
								Parts:     []a2a.Part{},
							},
						},
						History: []a2a.Message{
							{
								Kind:      "message",
								MessageID: "msg-456",
								Role:      "assistant",
								Parts:     []a2a.Part{},
							},
						},
					},
				}
				mockA2AClient.EXPECT().GetTask(gomock.Any(), gomock.Any(), "http://agent1.example.com").Return(getTaskResp, nil).AnyTimes()
			}

			cfg := config.Config{
				A2A: &config.A2AConfig{
					Enable: true,
				},
			}

			middleware, err := middlewares.NewA2AMiddleware(mockRegistry, mockA2AClient, mockA2AAgent, mockLogger, mockInferenceClient, cfg)
			assert.NoError(t, err)

			gin.SetMode(gin.TestMode)
			router := gin.New()
			router.Use(middleware.Middleware())

			router.POST("/v1/chat/completions", func(c *gin.Context) {
				toolCalls := []providers.ChatCompletionMessageToolCall{
					{
						ID: "call_1",
						Function: providers.ChatCompletionMessageToolCallFunction{
							Name:      "calculate",
							Arguments: `{"operation": "add", "a": 5, "b": 3}`,
						},
					},
				}

				response := providers.CreateChatCompletionResponse{
					ID:    "test-id",
					Model: "gpt-4",
					Choices: []providers.ChatCompletionChoice{
						{
							Message: providers.Message{
								Role:      providers.MessageRoleAssistant,
								Content:   "I'll calculate that for you.",
								ToolCalls: &toolCalls,
							},
							FinishReason: providers.FinishReasonToolCalls,
						},
					},
				}
				c.JSON(http.StatusOK, response)
			})

			requestData := providers.CreateChatCompletionRequest{
				Model: "openai/gpt-4",
				Messages: []providers.Message{
					{Role: providers.MessageRoleUser, Content: "Calculate 5+3"},
				},
			}

			if tt.isStreaming {
				requestData.Stream = &[]bool{true}[0]
			}

			requestBody, _ := json.Marshal(requestData)
			req := httptest.NewRequest("POST", "/v1/chat/completions", bytes.NewReader(requestBody))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusOK, w.Code)

			var response map[string]interface{}
			err = json.Unmarshal(w.Body.Bytes(), &response)
			assert.NoError(t, err)
		})
	}
}

func TestA2AMiddleware_TaskFailedExecution(t *testing.T) {
	tests := []struct {
		name               string
		taskFailure        string
		sendMessageError   error
		taskStatus         string
		expectedStatus     int
		expectErrorMessage bool
	}{
		{
			name:               "Task execution fails with agent connection error",
			taskFailure:        "agent_connection_error",
			sendMessageError:   fmt.Errorf("agent connection failed"),
			expectedStatus:     http.StatusOK,
			expectErrorMessage: true,
		},
		{
			name:               "Task execution fails with timeout",
			taskFailure:        "polling_timeout",
			taskStatus:         string(a2a.TaskStateFailed),
			expectedStatus:     http.StatusOK,
			expectErrorMessage: true,
		},
		{
			name:               "Task execution fails with failed state",
			taskFailure:        "task_failed",
			taskStatus:         string(a2a.TaskStateFailed),
			expectedStatus:     http.StatusOK,
			expectErrorMessage: true,
		},
		{
			name:               "Task execution fails with rejected state",
			taskFailure:        "task_rejected",
			taskStatus:         string(a2a.TaskStateRejected),
			expectedStatus:     http.StatusOK,
			expectErrorMessage: true,
		},
		{
			name:               "Task execution fails with canceled state",
			taskFailure:        "task_canceled",
			taskStatus:         string(a2a.TaskStateCanceled),
			expectedStatus:     http.StatusOK,
			expectErrorMessage: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockRegistry := providersmocks.NewMockProviderRegistry(ctrl)
			mockA2AClient := a2amocks.NewMockA2AClientInterface(ctrl)
			mockA2AAgent := a2amocks.NewMockAgent(ctrl)
			mockLogger := mocks.NewMockLogger(ctrl)
			mockInferenceClient := providersmocks.NewMockClient(ctrl)
			mockProvider := providersmocks.NewMockIProvider(ctrl)

			mockLogger.EXPECT().Debug(gomock.Any(), gomock.Any()).AnyTimes()
			mockLogger.EXPECT().Debug(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
			mockLogger.EXPECT().Debug(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
			mockLogger.EXPECT().Debug(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
			mockLogger.EXPECT().Debug(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
			mockLogger.EXPECT().Error(gomock.Any(), gomock.Any()).AnyTimes()
			mockLogger.EXPECT().Error(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
			mockLogger.EXPECT().Error(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
			mockLogger.EXPECT().Warn(gomock.Any(), gomock.Any()).AnyTimes()

			mockA2AClient.EXPECT().IsInitialized().Return(true)
			mockA2AClient.EXPECT().GetAgents().Return([]string{"http://agent1.example.com"}).AnyTimes()

			mockRegistry.EXPECT().BuildProvider(providers.OpenaiID, mockInferenceClient).Return(mockProvider, nil)

			mockA2AAgent.EXPECT().SetProvider(mockProvider).AnyTimes()
			mockA2AAgent.EXPECT().SetModel(gomock.Any()).AnyTimes()
			mockA2AAgent.EXPECT().Run(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

			if tt.sendMessageError != nil {
				mockA2AClient.EXPECT().SendMessage(gomock.Any(), gomock.Any(), "http://agent1.example.com").Return(nil, tt.sendMessageError).AnyTimes()
			} else {
				sendMessageResp := &a2a.SendMessageSuccessResponse{
					Result: a2a.Task{
						ID: "task_failed_123",
					},
				}
				mockA2AClient.EXPECT().SendMessage(gomock.Any(), gomock.Any(), "http://agent1.example.com").Return(sendMessageResp, nil).AnyTimes()

				getTaskResp := &a2a.GetTaskSuccessResponse{
					Result: a2a.Task{
						Status: a2a.TaskStatus{
							State: a2a.TaskState(tt.taskStatus),
							Message: &a2a.Message{
								Kind:      "message",
								MessageID: "msg-123",
								Role:      "assistant",
								Parts:     []a2a.Part{},
							},
						},
						History: []a2a.Message{
							{
								Kind:      "message",
								MessageID: "msg-456",
								Role:      "assistant",
								Parts:     []a2a.Part{},
							},
						},
					},
				}
				mockA2AClient.EXPECT().GetTask(gomock.Any(), gomock.Any(), "http://agent1.example.com").Return(getTaskResp, nil).AnyTimes()
			}

			cfg := config.Config{
				A2A: &config.A2AConfig{
					Enable: true,
				},
			}

			middleware, err := middlewares.NewA2AMiddleware(mockRegistry, mockA2AClient, mockA2AAgent, mockLogger, mockInferenceClient, cfg)
			assert.NoError(t, err)

			gin.SetMode(gin.TestMode)
			router := gin.New()
			router.Use(middleware.Middleware())

			router.POST("/v1/chat/completions", func(c *gin.Context) {
				toolCalls := []providers.ChatCompletionMessageToolCall{
					{
						ID: "call_1",
						Function: providers.ChatCompletionMessageToolCallFunction{
							Name:      "calculate",
							Arguments: `{"operation": "add", "a": 5, "b": 3}`,
						},
					},
				}

				response := providers.CreateChatCompletionResponse{
					ID:    "test-id",
					Model: "gpt-4",
					Choices: []providers.ChatCompletionChoice{
						{
							Message: providers.Message{
								Role:      providers.MessageRoleAssistant,
								Content:   "I'll calculate that for you.",
								ToolCalls: &toolCalls,
							},
							FinishReason: providers.FinishReasonToolCalls,
						},
					},
				}
				c.JSON(http.StatusOK, response)
			})

			requestData := providers.CreateChatCompletionRequest{
				Model: "openai/gpt-4",
				Messages: []providers.Message{
					{Role: providers.MessageRoleUser, Content: "Calculate 5+3"},
				},
			}

			requestBody, _ := json.Marshal(requestData)
			req := httptest.NewRequest("POST", "/v1/chat/completions", bytes.NewReader(requestBody))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)

			var response map[string]interface{}
			err = json.Unmarshal(w.Body.Bytes(), &response)
			assert.NoError(t, err)

			assert.NotNil(t, response)
		})
	}
}

func TestA2AMiddleware_InvalidRequests(t *testing.T) {
	tests := []struct {
		name           string
		requestBody    string
		expectedStatus int
		expectedError  string
	}{
		{
			name:           "Invalid JSON request body",
			requestBody:    `{"model": "openai/gpt-4", "messages": [}`,
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Invalid request body",
		},
		{
			name:           "Empty request body",
			requestBody:    ``,
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Invalid request body",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockRegistry := providersmocks.NewMockProviderRegistry(ctrl)
			mockA2AClient := a2amocks.NewMockA2AClientInterface(ctrl)
			mockA2AAgent := a2amocks.NewMockAgent(ctrl)
			mockLogger := mocks.NewMockLogger(ctrl)
			mockInferenceClient := providersmocks.NewMockClient(ctrl)

			mockLogger.EXPECT().Debug(gomock.Any(), gomock.Any()).AnyTimes()
			mockLogger.EXPECT().Error(gomock.Any(), gomock.Any()).AnyTimes()

			cfg := config.Config{
				A2A: &config.A2AConfig{
					Enable: true,
				},
			}

			middleware, err := middlewares.NewA2AMiddleware(mockRegistry, mockA2AClient, mockA2AAgent, mockLogger, mockInferenceClient, cfg)
			assert.NoError(t, err)

			gin.SetMode(gin.TestMode)
			router := gin.New()
			router.Use(middleware.Middleware())

			router.POST("/v1/chat/completions", func(c *gin.Context) {
				c.JSON(http.StatusOK, gin.H{"test": "should not reach here"})
			})

			req := httptest.NewRequest("POST", "/v1/chat/completions", bytes.NewReader([]byte(tt.requestBody)))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)

			if tt.expectedError != "" {
				var response map[string]interface{}
				err = json.Unmarshal(w.Body.Bytes(), &response)
				assert.NoError(t, err)
				assert.Contains(t, response["error"], tt.expectedError)
			}
		})
	}
}

func TestA2AMiddleware_ProgressiveDiscoveryPreventsRepeatedQueries(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRegistry := providersmocks.NewMockProviderRegistry(ctrl)
	mockA2AClient := a2amocks.NewMockA2AClientInterface(ctrl)
	mockLogger := mocks.NewMockLogger(ctrl)
	mockInferenceClient := providersmocks.NewMockClient(ctrl)
	mockProvider := providersmocks.NewMockIProvider(ctrl)

	mockLogger.EXPECT().Debug(gomock.Any(), gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Debug(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Debug(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Debug(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Debug(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Info(gomock.Any(), gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Info(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Info(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Info(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Warn(gomock.Any(), gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Error(gomock.Any(), gomock.Any()).AnyTimes()

	mockA2AClient.EXPECT().IsInitialized().Return(true).AnyTimes()
	mockA2AClient.EXPECT().GetAgents().Return([]string{"http://test-agent.com"}).AnyTimes()
	mockRegistry.EXPECT().BuildProvider(providers.OpenaiID, mockInferenceClient).Return(mockProvider, nil).AnyTimes()
	mockProvider.EXPECT().GetName().Return("openai").AnyTimes()

	realA2AAgent := a2a.NewAgent(mockLogger, mockA2AClient)

	agentSkills := []a2a.AgentSkill{
		{
			ID:          "calculate",
			Name:        "calculate",
			Description: "Perform mathematical calculations",
			Examples:    []string{"Add 2 + 3", "Multiply 5 * 7"},
		},
	}

	mockA2AClient.EXPECT().GetAgentCard(gomock.Any(), "http://test-agent.com").Return(&a2a.AgentCard{
		Name:        "Test Agent",
		Description: "A test agent for calculations",
		Version:     "1.0",
		Skills:      agentSkills,
	}, nil).Times(1)

	mockA2AClient.EXPECT().SendMessage(gomock.Any(), gomock.Any(), "http://test-agent.com").Return(&a2a.SendMessageSuccessResponse{
		ID:      "test-response-id",
		JSONRPC: "2.0",
		Result: a2a.Message{
			Kind:      "message",
			MessageID: "msg-123",
			Role:      "assistant",
			Parts:     []a2a.Part{},
		},
	}, nil).AnyTimes()

	realA2AAgent.SetProvider(mockProvider)
	model := "gpt-4"
	realA2AAgent.SetModel(&model)

	firstCall := true
	var capturedChatRequests []*providers.CreateChatCompletionRequest
	mockProvider.EXPECT().ChatCompletions(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, request providers.CreateChatCompletionRequest) (providers.CreateChatCompletionResponse, error) {
			capturedChatRequests = append(capturedChatRequests, &request)

			if firstCall {
				firstCall = false
				return providers.CreateChatCompletionResponse{
					ID:    "test-id-1",
					Model: "gpt-4",
					Choices: []providers.ChatCompletionChoice{
						{
							Message: providers.Message{
								Role:    providers.MessageRoleAssistant,
								Content: "I'll use the calculate tool to solve this.",
								ToolCalls: &[]providers.ChatCompletionMessageToolCall{
									{
										ID:   "call_2",
										Type: providers.ChatCompletionToolTypeFunction,
										Function: providers.ChatCompletionMessageToolCallFunction{
											Name:      "calculate",
											Arguments: `{"operation": "add", "numbers": [2, 3]}`,
										},
									},
								},
							},
							FinishReason: providers.FinishReasonToolCalls,
						},
					},
				}, nil
			}

			return providers.CreateChatCompletionResponse{
				ID:    "test-id-2",
				Model: "gpt-4",
				Choices: []providers.ChatCompletionChoice{
					{
						Message: providers.Message{
							Role:    providers.MessageRoleAssistant,
							Content: "The calculation result is 5.",
						},
						FinishReason: providers.FinishReasonStop,
					},
				},
			}, nil
		}).AnyTimes()

	cfg := config.Config{
		A2A: &config.A2AConfig{
			Enable: true,
		},
	}

	middleware, err := middlewares.NewA2AMiddleware(mockRegistry, mockA2AClient, realA2AAgent, mockLogger, mockInferenceClient, cfg)
	assert.NoError(t, err)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(middleware.Middleware())

	var capturedRequests []*providers.CreateChatCompletionRequest
	router.POST("/v1/chat/completions", func(c *gin.Context) {
		var req providers.CreateChatCompletionRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		capturedRequests = append(capturedRequests, &req)

		response := providers.CreateChatCompletionResponse{
			ID:    "test-id",
			Model: "gpt-4",
			Choices: []providers.ChatCompletionChoice{
				{
					Message: providers.Message{
						Role:    providers.MessageRoleAssistant,
						Content: "I'll query the agent card to find available tools.",
						ToolCalls: &[]providers.ChatCompletionMessageToolCall{
							{
								ID:   "call_1",
								Type: providers.ChatCompletionToolTypeFunction,
								Function: providers.ChatCompletionMessageToolCallFunction{
									Name:      "query_a2a_agent_card",
									Arguments: `{"agent_url": "http://test-agent.com"}`,
								},
							},
						},
					},
					FinishReason: providers.FinishReasonToolCalls,
				},
			},
		}

		err := realA2AAgent.Run(c.Request.Context(), &req, &response)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, response)
	})

	requestBody := providers.CreateChatCompletionRequest{
		Model: "openai/gpt-4",
		Messages: []providers.Message{
			{
				Role:    providers.MessageRoleUser,
				Content: "I need to calculate 2 + 3",
			},
		},
	}

	requestBytes, err := json.Marshal(requestBody)
	assert.NoError(t, err)

	req := httptest.NewRequest("POST", "/v1/chat/completions", bytes.NewReader(requestBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	if len(capturedChatRequests) >= 2 {
		secondRequest := capturedChatRequests[1]
		if secondRequest.Tools != nil {
			foundCalculateTool := false
			for _, tool := range *secondRequest.Tools {
				if tool.Function.Name == "calculate" {
					foundCalculateTool = true
					break
				}
			}
			assert.True(t, foundCalculateTool, "Calculate tool should be available in second iteration due to progressive discovery")
		}
	}
}

func boolPtr(b bool) *bool {
	return &b
}
