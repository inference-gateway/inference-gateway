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
	"github.com/inference-gateway/a2a/adk"
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

// closeNotifierRecorder implements http.CloseNotifier for testing
type closeNotifierRecorder struct {
	*httptest.ResponseRecorder
	closeNotify chan bool
}

func newCloseNotifierRecorder() *closeNotifierRecorder {
	return &closeNotifierRecorder{
		ResponseRecorder: httptest.NewRecorder(),
		closeNotify:      make(chan bool, 1),
	}
}

func (c *closeNotifierRecorder) CloseNotify() <-chan bool {
	return c.closeNotify
}

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
				mockA2AClient.EXPECT().GetAllAgentStatuses().Return(map[string]a2a.AgentStatus{
					"http://agent1.example.com": a2a.AgentStatusAvailable,
				}).AnyTimes()
				mockA2AClient.EXPECT().GetAgents().Return([]string{}).AnyTimes()
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
				req.Header.Set("X-A2A-Bypass", "true")
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
		agentSkills          map[string][]adk.AgentSkill
		expectToolsAdded     bool
		expectedToolCount    int
	}{
		{
			name:                 "No agents available - only agent query tool added",
			a2aClientInitialized: true,
			agents:               []string{},
			agentSkills:          map[string][]adk.AgentSkill{},
			expectToolsAdded:     true,
			expectedToolCount:    2,
		},
		{
			name:                 "Single agent with skills - only query tool added initially",
			a2aClientInitialized: true,
			agents:               []string{"http://agent1.example.com"},
			agentSkills: map[string][]adk.AgentSkill{
				"http://agent1.example.com": {
					{ID: "add", Name: "Add Numbers", Description: "Add two numbers"},
					{ID: "multiply", Name: "Multiply Numbers", Description: "Multiply two numbers"},
				},
			},
			expectToolsAdded:  true,
			expectedToolCount: 2,
		},
		{
			name:                 "Multiple agents with skills - only query tool added initially",
			a2aClientInitialized: true,
			agents:               []string{"http://agent1.example.com", "http://agent2.example.com"},
			agentSkills: map[string][]adk.AgentSkill{
				"http://agent1.example.com": {
					{ID: "calculate", Name: "Calculate", Description: "Mathematical calculations"},
				},
				"http://agent2.example.com": {
					{ID: "weather", Name: "Weather", Description: "Get weather information"},
				},
			},
			expectToolsAdded:  true,
			expectedToolCount: 2,
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
				mockA2AClient.EXPECT().GetAllAgentStatuses().Return(map[string]a2a.AgentStatus{
					"http://agent1.example.com": a2a.AgentStatusAvailable,
				}).AnyTimes()
				mockA2AClient.EXPECT().GetAgents().Return(tt.agents).AnyTimes()
				mockRegistry.EXPECT().BuildProvider(providers.OpenaiID, mockInferenceClient).Return(mockProvider, nil)
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
				foundTaskSubmissionTool := false
				for _, tool := range *capturedRequest.Tools {
					if tool.Function.Name == a2a.ToolQueryAgentCard {
						foundAgentQueryTool = true
					}
					if tool.Function.Name == a2a.ToolSubmitTaskToAgent {
						foundTaskSubmissionTool = true
					}
				}
				assert.True(t, foundAgentQueryTool, "Agent query tool should be present")
				assert.True(t, foundTaskSubmissionTool, "Task submission tool should be present")
			}
		})
	}
}

func TestA2AMiddleware_LLMDecisionToSubmitTask(t *testing.T) {
	tests := []struct {
		name                  string
		toolCalls             []providers.ChatCompletionMessageToolCall
		availableAgents       []string
		agentSkills           map[string][]adk.AgentSkill
		agentCard             *adk.AgentCard
		agentCapabilities     map[string]adk.AgentCapabilities
		sendMessageResp       *adk.Task
		sendMessageError      error
		getTaskResp           *adk.Task
		getTaskError          error
		expectAgentCardCall   bool
		expectSendMessageCall bool
		expectGetTaskCall     bool
		expectedStatus        int
	}{
		{
			name: "LLM calls agent query tool",
			toolCalls: []providers.ChatCompletionMessageToolCall{
				{
					ID: "call_1",
					Function: providers.ChatCompletionMessageToolCallFunction{
						Name:      a2a.ToolQueryAgentCard,
						Arguments: `{"agent_url": "http://agent1.example.com"}`,
					},
				},
			},
			availableAgents: []string{"http://agent1.example.com"},
			agentSkills: map[string][]adk.AgentSkill{
				"http://agent1.example.com": {
					{ID: "calculate", Name: "Calculator", Description: "Mathematical calculations"},
				},
			},
			agentCard: &adk.AgentCard{
				Name:        "Test Agent",
				Description: "Test Description",
				Version:     "1.0",
				Skills: []adk.AgentSkill{
					{ID: "calculate", Name: "Calculator", Description: "Mathematical calculations"},
				},
			},
			agentCapabilities: map[string]adk.AgentCapabilities{
				"http://agent1.example.com": {
					PushNotifications:      boolPtr(false),
					StateTransitionHistory: boolPtr(false),
					Streaming:              boolPtr(false),
				},
			},
			expectAgentCardCall:   true,
			expectSendMessageCall: false,
			expectGetTaskCall:     false,
			expectedStatus:        http.StatusOK,
		},
		{
			name: "LLM calls a2a_submit_task_to_agent tool - successful execution",
			toolCalls: []providers.ChatCompletionMessageToolCall{
				{
					ID: "call_2",
					Function: providers.ChatCompletionMessageToolCallFunction{
						Name:      a2a.ToolSubmitTaskToAgent,
						Arguments: `{"agent_url": "http://agent1.example.com", "task_description": "Calculate 5+3"}`,
					},
				},
			},
			availableAgents: []string{"http://agent1.example.com"},
			agentSkills: map[string][]adk.AgentSkill{
				"http://agent1.example.com": {
					{ID: "calculate", Name: "Calculator", Description: "Mathematical calculations"},
				},
			},
			sendMessageResp: &adk.Task{
				ID: "task-123",
				Status: adk.TaskStatus{
					State: adk.TaskStateCompleted,
					Message: &adk.Message{
						Kind:      "message",
						MessageID: "msg-123",
						Role:      "assistant",
						Parts:     []adk.Part{},
					},
				},
			},
			getTaskResp: &adk.Task{
				ID: "task-123",
				Status: adk.TaskStatus{
					State: adk.TaskStateCompleted,
					Message: &adk.Message{
						Kind:      "message",
						MessageID: "msg-123",
						Role:      "assistant",
						Parts:     []adk.Part{},
					},
				},
			},
			expectAgentCardCall:   false,
			expectSendMessageCall: true,
			expectGetTaskCall:     true,
			expectedStatus:        http.StatusOK,
		},
		{
			name: "Task submission fails with connection error",
			toolCalls: []providers.ChatCompletionMessageToolCall{
				{
					ID: "call_3",
					Function: providers.ChatCompletionMessageToolCallFunction{
						Name:      a2a.ToolSubmitTaskToAgent,
						Arguments: `{"agent_url": "http://agent1.example.com", "task_description": "Calculate 5+3"}`,
					},
				},
			},
			availableAgents: []string{"http://agent1.example.com"},
			agentSkills: map[string][]adk.AgentSkill{
				"http://agent1.example.com": {
					{ID: "calculate", Name: "Calculator", Description: "Mathematical calculations"},
				},
			},
			sendMessageError:      fmt.Errorf("agent connection failed"),
			expectAgentCardCall:   false,
			expectSendMessageCall: true,
			expectGetTaskCall:     false,
			expectedStatus:        http.StatusOK,
		},
		{
			name: "Task submitted but polling fails",
			toolCalls: []providers.ChatCompletionMessageToolCall{
				{
					ID: "call_4",
					Function: providers.ChatCompletionMessageToolCallFunction{
						Name:      a2a.ToolSubmitTaskToAgent,
						Arguments: `{"agent_url": "http://agent1.example.com", "task_description": "Calculate 5+3"}`,
					},
				},
			},
			availableAgents: []string{"http://agent1.example.com"},
			agentSkills: map[string][]adk.AgentSkill{
				"http://agent1.example.com": {
					{ID: "calculate", Name: "Calculator", Description: "Mathematical calculations"},
				},
			},
			sendMessageResp: &adk.Task{
				ID: "task-456",
				Status: adk.TaskStatus{
					State: adk.TaskStateSubmitted,
				},
			},
			getTaskError:          fmt.Errorf("polling failed"),
			expectAgentCardCall:   false,
			expectSendMessageCall: true,
			expectGetTaskCall:     true,
			expectedStatus:        http.StatusOK,
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
			mockLogger.EXPECT().Info(gomock.Any(), gomock.Any()).AnyTimes()
			mockLogger.EXPECT().Info(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

			mockA2AClient.EXPECT().IsInitialized().Return(true)
			mockA2AClient.EXPECT().GetAllAgentStatuses().Return(map[string]a2a.AgentStatus{
				"http://agent1.example.com": a2a.AgentStatusAvailable,
			}).AnyTimes()
			mockA2AClient.EXPECT().GetAgents().Return(tt.availableAgents).AnyTimes()
			mockRegistry.EXPECT().BuildProvider(providers.OpenaiID, mockInferenceClient).Return(mockProvider, nil)
			mockA2AAgent.EXPECT().SetProvider(mockProvider).AnyTimes()
			mockA2AAgent.EXPECT().SetModel(gomock.Any()).AnyTimes()
			mockA2AAgent.EXPECT().Run(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

			for agentURL, skills := range tt.agentSkills {
				mockA2AClient.EXPECT().GetAgentSkills(agentURL).Return(skills, nil).AnyTimes()
			}

			if tt.expectAgentCardCall {
				mockA2AClient.EXPECT().GetAgentCard(gomock.Any(), gomock.Any()).Return(tt.agentCard, nil).AnyTimes()
				if tt.agentCapabilities != nil {
					mockA2AClient.EXPECT().GetAgentCapabilities().Return(tt.agentCapabilities).AnyTimes()
				}
			}

			if tt.expectSendMessageCall {
				if tt.sendMessageError != nil {
					mockA2AClient.EXPECT().SendMessage(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, tt.sendMessageError).AnyTimes()
				} else if tt.sendMessageResp != nil {
					sendMessageResp := &adk.SendMessageSuccessResponse{
						JSONRPC: "2.0",
						Result:  tt.sendMessageResp,
					}
					mockA2AClient.EXPECT().SendMessage(gomock.Any(), gomock.Any(), gomock.Any()).Return(sendMessageResp, nil).AnyTimes()
				}
			}

			if tt.expectGetTaskCall {
				if tt.getTaskError != nil {
					mockA2AClient.EXPECT().GetTask(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, tt.getTaskError).AnyTimes()
				} else if tt.getTaskResp != nil {
					getTaskResp := &adk.GetTaskSuccessResponse{
						JSONRPC: "2.0",
						Result:  *tt.getTaskResp,
					}
					mockA2AClient.EXPECT().GetTask(gomock.Any(), gomock.Any(), gomock.Any()).Return(getTaskResp, nil).AnyTimes()
				}
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

			assert.Equal(t, tt.expectedStatus, w.Code)

			var response map[string]interface{}
			err = json.Unmarshal(w.Body.Bytes(), &response)
			assert.NoError(t, err)
			assert.NotNil(t, response)
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
		expectStreamEvents bool
	}{
		{
			name:               "Synchronous task execution success",
			isStreaming:        false,
			taskID:             "task_123",
			taskStatus:         string(adk.TaskStateCompleted),
			taskResult:         "Result: 8",
			expectStreamEvents: false,
		},
		{
			name:               "Asynchronous task execution with streaming",
			isStreaming:        true,
			taskID:             "task_456",
			taskStatus:         string(adk.TaskStateCompleted),
			taskResult:         "Result: 15",
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
			mockA2AClient.EXPECT().GetAllAgentStatuses().Return(map[string]a2a.AgentStatus{
				"http://agent1.example.com": a2a.AgentStatusAvailable,
			}).AnyTimes()
			mockA2AClient.EXPECT().GetAgents().Return([]string{"http://agent1.example.com"}).AnyTimes()

			mockRegistry.EXPECT().BuildProvider(providers.OpenaiID, mockInferenceClient).Return(mockProvider, nil)
			mockA2AAgent.EXPECT().SetProvider(mockProvider).AnyTimes()
			mockA2AAgent.EXPECT().SetModel(gomock.Any()).AnyTimes()
			if tt.isStreaming {
				mockA2AAgent.EXPECT().RunWithStream(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(
					func(ctx context.Context, middlewareStreamCh chan []byte, c *gin.Context, body *providers.CreateChatCompletionRequest) error {
						middlewareStreamCh <- []byte("data: {\"id\":\"test\",\"object\":\"chat.completion.chunk\",\"choices\":[{\"delta\":{\"content\":\"Test streaming response\"}}]}\n\n")
						middlewareStreamCh <- []byte("data: [DONE]\n\n")
						return nil
					}).AnyTimes()
			} else {
				mockA2AAgent.EXPECT().Run(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
			}

			sendMessageTask := &adk.Task{
				ID: tt.taskID,
				Status: adk.TaskStatus{
					State: adk.TaskStateCompleted,
					Message: &adk.Message{
						Kind:      "message",
						MessageID: "msg-123",
						Role:      "assistant",
						Parts:     []adk.Part{},
					},
				},
			}

			// Mock SendMessage returning task ID
			sendMessageResp := &adk.SendMessageSuccessResponse{
				JSONRPC: "2.0",
				Result:  sendMessageTask,
			}
			mockA2AClient.EXPECT().SendMessage(gomock.Any(), gomock.Any(), "http://agent1.example.com").Return(sendMessageResp, nil).AnyTimes()

			// Mock GetTask returning completed task (for polling)
			getTaskResp := &adk.GetTaskSuccessResponse{
				JSONRPC: "2.0",
				Result:  *sendMessageTask,
			}
			mockA2AClient.EXPECT().GetTask(gomock.Any(), gomock.Any(), "http://agent1.example.com").Return(getTaskResp, nil).AnyTimes()

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

			var w *httptest.ResponseRecorder
			if tt.isStreaming {
				cnr := newCloseNotifierRecorder()
				w = cnr.ResponseRecorder
				router.ServeHTTP(cnr, req)
			} else {
				w = httptest.NewRecorder()
				router.ServeHTTP(w, req)
			}

			assert.Equal(t, http.StatusOK, w.Code)

			if !tt.isStreaming {
				var response map[string]interface{}
				err = json.Unmarshal(w.Body.Bytes(), &response)
				assert.NoError(t, err)
			} else {
				assert.Greater(t, w.Body.Len(), 0, "Streaming response should have content")
			}
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
			taskStatus:         string(adk.TaskStateFailed),
			expectedStatus:     http.StatusOK,
			expectErrorMessage: true,
		},
		{
			name:               "Task execution fails with failed state",
			taskFailure:        "task_failed",
			taskStatus:         string(adk.TaskStateFailed),
			expectedStatus:     http.StatusOK,
			expectErrorMessage: true,
		},
		{
			name:               "Task execution fails with rejected state",
			taskFailure:        "task_rejected",
			taskStatus:         string(adk.TaskStateRejected),
			expectedStatus:     http.StatusOK,
			expectErrorMessage: true,
		},
		{
			name:               "Task execution fails with canceled state",
			taskFailure:        "task_canceled",
			taskStatus:         string(adk.TaskStateCanceled),
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
			mockA2AClient.EXPECT().GetAllAgentStatuses().Return(map[string]a2a.AgentStatus{
				"http://agent1.example.com": a2a.AgentStatusAvailable,
			}).AnyTimes()
			mockA2AClient.EXPECT().GetAgents().Return([]string{"http://agent1.example.com"}).AnyTimes()

			mockRegistry.EXPECT().BuildProvider(providers.OpenaiID, mockInferenceClient).Return(mockProvider, nil)

			mockA2AAgent.EXPECT().SetProvider(mockProvider).AnyTimes()
			mockA2AAgent.EXPECT().SetModel(gomock.Any()).AnyTimes()
			mockA2AAgent.EXPECT().Run(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

			if tt.sendMessageError != nil {
				mockA2AClient.EXPECT().SendMessage(gomock.Any(), gomock.Any(), "http://agent1.example.com").Return(nil, tt.sendMessageError).AnyTimes()
			} else {
				sendMessageTask := &adk.Task{
					ID: "task_failed_123",
					Status: adk.TaskStatus{
						State: adk.TaskState(tt.taskStatus),
						Message: &adk.Message{
							Kind:      "message",
							MessageID: "msg-123",
							Role:      "assistant",
							Parts:     []adk.Part{},
						},
					},
				}

				// Mock SendMessage returning task ID
				sendMessageResp := &adk.SendMessageSuccessResponse{
					JSONRPC: "2.0",
					Result:  sendMessageTask,
				}
				mockA2AClient.EXPECT().SendMessage(gomock.Any(), gomock.Any(), "http://agent1.example.com").Return(sendMessageResp, nil).AnyTimes()

				// Mock GetTask returning task with specified status (for polling)
				getTaskResp := &adk.GetTaskSuccessResponse{
					JSONRPC: "2.0",
					Result:  *sendMessageTask,
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

func boolPtr(b bool) *bool {
	return &b
}
