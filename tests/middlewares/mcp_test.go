package middleware_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/inference-gateway/inference-gateway/api/middlewares"
	"github.com/inference-gateway/inference-gateway/config"
	"github.com/inference-gateway/inference-gateway/mcp"
	"github.com/inference-gateway/inference-gateway/providers"
	"github.com/inference-gateway/inference-gateway/tests/mocks"
	mcpmocks "github.com/inference-gateway/inference-gateway/tests/mocks/mcp"
	providersmocks "github.com/inference-gateway/inference-gateway/tests/mocks/providers"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// Test helper to create mock dependencies for each test case
func createMockDependencies(t *testing.T) (*gomock.Controller, *providersmocks.MockProviderRegistry, *providersmocks.MockClient, *mcpmocks.MockMCPClientInterface, *mocks.MockLogger, *providersmocks.MockIProvider) {
	ctrl := gomock.NewController(t)
	mockRegistry := providersmocks.NewMockProviderRegistry(ctrl)
	mockClient := providersmocks.NewMockClient(ctrl)
	mockMCPClient := mcpmocks.NewMockMCPClientInterface(ctrl)
	mockLogger := mocks.NewMockLogger(ctrl)
	mockProvider := providersmocks.NewMockIProvider(ctrl)

	return ctrl, mockRegistry, mockClient, mockMCPClient, mockLogger, mockProvider
}

// Test helper to create a basic config
func createTestConfig() config.Config {
	return config.Config{
		Environment: "test",
		Server: &config.ServerConfig{
			Host: "localhost",
			Port: "8080",
		},
	}
}

func TestNewMCPMiddleware(t *testing.T) {
	tests := []struct {
		name        string
		mcpClient   mcp.MCPClientInterface
		expectNoOp  bool
		expectError bool
	}{
		{
			name:        "Success with MCP client",
			mcpClient:   &mcpmocks.MockMCPClientInterface{},
			expectNoOp:  false,
			expectError: false,
		},
		{
			name:        "Success with nil MCP client returns NoOp",
			mcpClient:   nil,
			expectNoOp:  true,
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl, mockRegistry, mockClient, _, mockLogger, _ := createMockDependencies(t)
			defer ctrl.Finish()

			cfg := createTestConfig()

			if tt.mcpClient == nil {
				mockLogger.EXPECT().Info("mcp client is nil, using no-op middleware")
			}

			mcpAgent := mcp.NewAgent(mockLogger, tt.mcpClient)
			middleware, err := middlewares.NewMCPMiddleware(mockRegistry, mockClient, tt.mcpClient, mcpAgent, mockLogger, cfg)

			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, middleware)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, middleware)

				ginHandler := middleware.Middleware()
				assert.NotNil(t, ginHandler)
			}
		})
	}
}

func TestMCPMiddleware_SkipConditions(t *testing.T) {
	tests := []struct {
		name           string
		path           string
		internalHeader string
		setupMocks     func(*providersmocks.MockProviderRegistry, *providersmocks.MockClient, *mcpmocks.MockMCPClientInterface, *mocks.MockLogger, *providersmocks.MockIProvider)
		shouldSkip     bool
	}{
		{
			name:           "Skip with internal header",
			path:           "/v1/chat/completions",
			internalHeader: "true",
			setupMocks: func(mockRegistry *providersmocks.MockProviderRegistry, mockClient *providersmocks.MockClient, mockMCPClient *mcpmocks.MockMCPClientInterface, mockLogger *mocks.MockLogger, mockProvider *providersmocks.MockIProvider) {
				mockLogger.EXPECT().Debug("not an internal mcp call").AnyTimes()
				mockLogger.EXPECT().Debug(gomock.Any(), gomock.Any()).AnyTimes()
				mockLogger.EXPECT().Debug(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
				mockLogger.EXPECT().Debug(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
			},
			shouldSkip: true,
		},
		{
			name:       "Skip non-chat endpoint",
			path:       "/v1/models",
			shouldSkip: true,
		},
		{
			name: "Process chat completions without internal header",
			path: "/v1/chat/completions",
			setupMocks: func(mockRegistry *providersmocks.MockProviderRegistry, mockClient *providersmocks.MockClient, mockMCPClient *mcpmocks.MockMCPClientInterface, mockLogger *mocks.MockLogger, mockProvider *providersmocks.MockIProvider) {
				mockMCPClient.EXPECT().IsInitialized().Return(true).AnyTimes()
				mockMCPClient.EXPECT().GetAllChatCompletionTools().Return([]providers.ChatCompletionTool{}).AnyTimes()
				mockLogger.EXPECT().Debug(gomock.Any(), gomock.Any()).AnyTimes()
				mockLogger.EXPECT().Debug(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
				mockLogger.EXPECT().Debug(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
			},
			shouldSkip: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl, mockRegistry, mockClient, mockMCPClient, mockLogger, mockProvider := createMockDependencies(t)
			defer ctrl.Finish()

			cfg := createTestConfig()

			if tt.setupMocks != nil {
				tt.setupMocks(mockRegistry, mockClient, mockMCPClient, mockLogger, mockProvider)
			}

			mcpAgent := mcp.NewAgent(mockLogger, mockMCPClient)
			middleware, err := middlewares.NewMCPMiddleware(mockRegistry, mockClient, mockMCPClient, mcpAgent, mockLogger, cfg)
			assert.NoError(t, err)

			router := gin.New()
			router.Use(middleware.Middleware())

			handlerCalled := false
			router.POST("/*path", func(c *gin.Context) {
				handlerCalled = true
				c.JSON(http.StatusOK, gin.H{"status": "ok"})
			})
			router.GET("/*path", func(c *gin.Context) {
				handlerCalled = true
				c.JSON(http.StatusOK, gin.H{"status": "ok"})
			})

			w := httptest.NewRecorder()
			method := "POST"
			if tt.path == "/v1/models" {
				method = "GET"
			}
			req := httptest.NewRequest(method, tt.path, strings.NewReader(`{"model":"gpt-4","messages":[]}`))
			req.Header.Set("Content-Type", "application/json")
			if tt.internalHeader != "" {
				req.Header.Set("X-MCP-Bypass", tt.internalHeader)
			}

			router.ServeHTTP(w, req)

			assert.True(t, handlerCalled, "Handler should always be called")
			assert.Equal(t, http.StatusOK, w.Code)
		})
	}
}

func TestMCPMiddleware_AddToolsToRequest(t *testing.T) {
	tests := []struct {
		name          string
		mcpTools      []providers.ChatCompletionTool
		requestTools  *[]providers.ChatCompletionTool
		expectedCount int
		isInitialized bool
	}{
		{
			name: "Add MCP tools to request without existing tools",
			mcpTools: []providers.ChatCompletionTool{
				{
					Type: providers.ChatCompletionToolTypeFunction,
					Function: providers.FunctionObject{
						Name: "mcp_test_tool",
					},
				},
			},
			requestTools:  nil,
			expectedCount: 1,
			isInitialized: true,
		},
		{
			name: "Add MCP tools to request with existing tools",
			mcpTools: []providers.ChatCompletionTool{
				{
					Type: providers.ChatCompletionToolTypeFunction,
					Function: providers.FunctionObject{
						Name: "mcp_tool",
					},
				},
			},
			requestTools: &[]providers.ChatCompletionTool{
				{
					Type: providers.ChatCompletionToolTypeFunction,
					Function: providers.FunctionObject{
						Name: "existing_tool",
					},
				},
			},
			expectedCount: 2,
			isInitialized: true,
		},
		{
			name:          "No tools added when MCP not initialized",
			mcpTools:      []providers.ChatCompletionTool{},
			requestTools:  nil,
			expectedCount: 0,
			isInitialized: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl, mockRegistry, mockClient, mockMCPClient, mockLogger, mockProvider := createMockDependencies(t)
			defer ctrl.Finish()

			cfg := createTestConfig()

			mockMCPClient.EXPECT().IsInitialized().Return(tt.isInitialized).AnyTimes()
			if tt.isInitialized {
				mockMCPClient.EXPECT().GetAllChatCompletionTools().Return(tt.mcpTools).AnyTimes()
			}
			mockLogger.EXPECT().Debug(gomock.Any(), gomock.Any()).AnyTimes()
			mockLogger.EXPECT().Debug(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
			mockLogger.EXPECT().Debug(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
			mockLogger.EXPECT().Error(gomock.Any(), gomock.Any()).AnyTimes()
			mockLogger.EXPECT().Error(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
			mockLogger.EXPECT().Error(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

			mockRegistry.EXPECT().BuildProvider(providers.OpenaiID, mockClient).Return(mockProvider, nil).AnyTimes()

			requestData := providers.CreateChatCompletionRequest{
				Model: "openai/gpt-3.5-turbo",
				Messages: []providers.Message{
					{Role: providers.MessageRoleUser, Content: "Hello"},
				},
				Tools: tt.requestTools,
			}

			requestBody, _ := json.Marshal(requestData)

			mcpAgent := mcp.NewAgent(mockLogger, mockMCPClient)
			middleware, err := middlewares.NewMCPMiddleware(mockRegistry, mockClient, mockMCPClient, mcpAgent, mockLogger, cfg)
			assert.NoError(t, err)
			router := gin.New()
			router.Use(middleware.Middleware())

			toolsAdded := false
			router.POST("/v1/chat/completions", func(c *gin.Context) {
				toolsAdded = true

				response := providers.CreateChatCompletionResponse{
					ID:    "test-id",
					Model: "gpt-3.5-turbo",
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

			w := httptest.NewRecorder()
			req := httptest.NewRequest("POST", "/v1/chat/completions", bytes.NewReader(requestBody))
			req.Header.Set("Content-Type", "application/json")

			router.ServeHTTP(w, req)

			if tt.isInitialized {
				assert.True(t, toolsAdded, "Handler should be called when MCP is initialized")
			}
			assert.Equal(t, http.StatusOK, w.Code)
		})
	}
}

func TestMCPMiddleware_NonStreamingWithToolCalls(t *testing.T) {
	tests := []struct {
		name            string
		toolCalls       []providers.ChatCompletionMessageToolCall
		toolResponse    *mcp.CallToolResult
		toolError       error
		expectAgentLoop bool
	}{
		{
			name: "Process tool calls successfully",
			toolCalls: []providers.ChatCompletionMessageToolCall{
				{
					ID:   "call_123",
					Type: providers.ChatCompletionToolTypeFunction,
					Function: providers.ChatCompletionMessageToolCallFunction{
						Name:      "test_function",
						Arguments: `{"param": "value"}`,
					},
				},
			},
			toolResponse: &mcp.CallToolResult{
				Content: []mcp.ContentBlock{
					mcp.TextContent{
						Type: "text",
						Text: "Tool executed successfully",
					},
				},
			},
			expectAgentLoop: true,
		},
		{
			name: "Handle tool execution error",
			toolCalls: []providers.ChatCompletionMessageToolCall{
				{
					ID:   "call_456",
					Type: providers.ChatCompletionToolTypeFunction,
					Function: providers.ChatCompletionMessageToolCallFunction{
						Name:      "failing_function",
						Arguments: `{"param": "value"}`,
					},
				},
			},
			toolError:       fmt.Errorf("tool execution failed"),
			expectAgentLoop: false,
		},
		{
			name: "Handle invalid tool arguments",
			toolCalls: []providers.ChatCompletionMessageToolCall{
				{
					ID:   "call_789",
					Type: providers.ChatCompletionToolTypeFunction,
					Function: providers.ChatCompletionMessageToolCallFunction{
						Name:      "test_function",
						Arguments: `invalid json`,
					},
				},
			},
			expectAgentLoop: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl, mockRegistry, mockClient, mockMCPClient, mockLogger, mockProvider := createMockDependencies(t)
			defer ctrl.Finish()

			cfg := createTestConfig()

			mockMCPClient.EXPECT().IsInitialized().Return(true).AnyTimes()
			mockMCPClient.EXPECT().GetAllChatCompletionTools().Return([]providers.ChatCompletionTool{}).AnyTimes()
			mockLogger.EXPECT().Debug(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
			mockRegistry.EXPECT().BuildProvider(providers.OpenaiID, mockClient).Return(mockProvider, nil).AnyTimes()

			for _, toolCall := range tt.toolCalls {
				var args map[string]interface{}
				if json.Unmarshal([]byte(toolCall.Function.Arguments), &args) == nil {
					mcpRequest := mcp.Request{
						Method: "tools/call",
						Params: map[string]interface{}{
							"name":      toolCall.Function.Name,
							"arguments": args,
						},
					}

					if tt.toolError != nil {
						mockMCPClient.EXPECT().ExecuteTool(gomock.Any(), mcpRequest, "").Return(nil, tt.toolError).AnyTimes()
						mockLogger.EXPECT().Error("Failed to execute tool call", tt.toolError, "tool", toolCall.Function.Name).AnyTimes()
					} else {
						mockMCPClient.EXPECT().ExecuteTool(gomock.Any(), mcpRequest, "").Return(tt.toolResponse, nil).AnyTimes()
					}
				} else {
					mockLogger.EXPECT().Error("Failed to parse tool arguments", gomock.Any(), "args", toolCall.Function.Arguments).AnyTimes()
				}
			}

			if tt.expectAgentLoop {
				mockLogger.EXPECT().Debug(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

				followUpResponse := providers.CreateChatCompletionResponse{
					ID:    "test-id-2",
					Model: "gpt-3.5-turbo",
					Choices: []providers.ChatCompletionChoice{
						{
							Message: providers.Message{
								Role:    providers.MessageRoleAssistant,
								Content: "Based on the tool results, here's my response.",
							},
							FinishReason: providers.FinishReasonStop,
						},
					},
				}
				mockProvider.EXPECT().ChatCompletions(gomock.Any(), gomock.Any()).Return(followUpResponse, nil).AnyTimes()
			} else {
				mockLogger.EXPECT().Debug(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
				mockProvider.EXPECT().ChatCompletions(gomock.Any(), gomock.Any()).Return(providers.CreateChatCompletionResponse{
					ID:    "test-id-error",
					Model: "gpt-3.5-turbo",
					Choices: []providers.ChatCompletionChoice{
						{
							Message: providers.Message{
								Role:    providers.MessageRoleAssistant,
								Content: "Error response",
							},
							FinishReason: providers.FinishReasonStop,
						},
					},
				}, nil).AnyTimes()
			}

			requestData := providers.CreateChatCompletionRequest{
				Model: "openai/gpt-3.5-turbo",
				Messages: []providers.Message{
					{Role: providers.MessageRoleUser, Content: "Please use the tool"},
				},
			}

			requestBody, _ := json.Marshal(requestData)

			mcpAgent := mcp.NewAgent(mockLogger, mockMCPClient)
			middleware, err := middlewares.NewMCPMiddleware(mockRegistry, mockClient, mockMCPClient, mcpAgent, mockLogger, cfg)
			assert.NoError(t, err)

			router := gin.New()
			router.Use(middleware.Middleware())

			router.POST("/v1/chat/completions", func(c *gin.Context) {
				response := providers.CreateChatCompletionResponse{
					ID:    "test-id",
					Model: "gpt-3.5-turbo",
					Choices: []providers.ChatCompletionChoice{
						{
							Message: providers.Message{
								Role:      providers.MessageRoleAssistant,
								Content:   "I'll use the tool to help you.",
								ToolCalls: &tt.toolCalls,
							},
							FinishReason: providers.FinishReasonToolCalls,
						},
					},
				}
				c.JSON(http.StatusOK, response)
			})

			w := httptest.NewRecorder()
			req := httptest.NewRequest("POST", "/v1/chat/completions", bytes.NewReader(requestBody))
			req.Header.Set("Content-Type", "application/json")

			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusOK, w.Code)
		})
	}
}

func TestMCPMiddleware_StreamingResponse(t *testing.T) {
	tests := []struct {
		name              string
		streamingResponse string
		expectToolCalls   bool
		expectedToolCalls int
	}{
		{
			name: "Process streaming response with tool calls",
			streamingResponse: `data: {"id":"chatcmpl-123","object":"chat.completion.chunk","created":1677652288,"model":"gpt-3.5-turbo","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"id":"call_123","type":"function","function":{"name":"test_function","arguments":"{\"param\":"}}]},"finish_reason":null}]}

data: {"id":"chatcmpl-123","object":"chat.completion.chunk","created":1677652288,"model":"gpt-3.5-turbo","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"arguments":"\"value\"}"}}]},"finish_reason":null}]}

data: {"id":"chatcmpl-123","object":"chat.completion.chunk","created":1677652288,"model":"gpt-3.5-turbo","choices":[{"index":0,"delta":{},"finish_reason":"tool_calls"}]}

data: [DONE]`,
			expectToolCalls:   true,
			expectedToolCalls: 1,
		},
		{
			name: "Process streaming response without tool calls",
			streamingResponse: `data: {"id":"chatcmpl-123","object":"chat.completion.chunk","created":1677652288,"model":"gpt-3.5-turbo","choices":[{"index":0,"delta":{"content":"Hello"},"finish_reason":null}]}

data: {"id":"chatcmpl-123","object":"chat.completion.chunk","created":1677652288,"model":"gpt-3.5-turbo","choices":[{"index":0,"delta":{"content":" there!"},"finish_reason":"stop"}]}

data: [DONE]`,
			expectToolCalls:   false,
			expectedToolCalls: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl, mockRegistry, mockClient, mockMCPClient, mockLogger, mockProvider := createMockDependencies(t)
			defer ctrl.Finish()

			cfg := createTestConfig()

			mockMCPClient.EXPECT().IsInitialized().Return(true).AnyTimes()
			mockMCPClient.EXPECT().GetAllChatCompletionTools().Return([]providers.ChatCompletionTool{}).AnyTimes()
			mockLogger.EXPECT().Debug(gomock.Any(), gomock.Any()).AnyTimes()
			mockLogger.EXPECT().Debug(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
			mockLogger.EXPECT().Debug(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
			mockLogger.EXPECT().Debug(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
			mockLogger.EXPECT().Debug(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
			mockRegistry.EXPECT().BuildProvider(providers.OpenaiID, mockClient).Return(mockProvider, nil).AnyTimes()

			if tt.expectToolCalls {
				mockMCPClient.EXPECT().ExecuteTool(gomock.Any(), gomock.Any(), "").Return(&mcp.CallToolResult{
					Content: []mcp.ContentBlock{
						mcp.TextContent{
							Type: "text",
							Text: "Tool result",
						},
					},
				}, nil).AnyTimes()

				mockLogger.EXPECT().Error(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

				streamCh := make(chan []byte, 1)
				streamCh <- []byte(`{"id":"chatcmpl-124","object":"chat.completion.chunk","created":1677652289,"model":"gpt-3.5-turbo","choices":[{"index":0,"delta":{"content":"Response after tool execution"},"finish_reason":"stop"}]}`)
				close(streamCh)

				mockProvider.EXPECT().StreamChatCompletions(gomock.Any(), gomock.Any()).Return(streamCh, nil).AnyTimes()
			}

			requestData := providers.CreateChatCompletionRequest{
				Model: "openai/gpt-3.5-turbo",
				Messages: []providers.Message{
					{Role: providers.MessageRoleUser, Content: "Test streaming"},
				},
				Stream: &[]bool{true}[0],
			}

			requestBody, _ := json.Marshal(requestData)

			mcpAgent := mcp.NewAgent(mockLogger, mockMCPClient)
			middleware, err := middlewares.NewMCPMiddleware(mockRegistry, mockClient, mockMCPClient, mcpAgent, mockLogger, cfg)
			assert.NoError(t, err)

			router := gin.New()
			router.Use(middleware.Middleware())

			router.POST("/v1/chat/completions", func(c *gin.Context) {
				c.Header("Content-Type", "text/event-stream")
				c.Writer.WriteHeader(http.StatusOK)
				_, err := c.Writer.Write([]byte(tt.streamingResponse))
				assert.NoError(t, err)
			})

			w := httptest.NewRecorder()
			req := httptest.NewRequest("POST", "/v1/chat/completions", bytes.NewReader(requestBody))
			req.Header.Set("Content-Type", "application/json")

			router.ServeHTTP(w, req)

			responseBody := w.Body.String()
			assert.NotEmpty(t, responseBody, "Response should not be empty")
		})
	}
}

func TestMCPMiddleware_ErrorHandling(t *testing.T) {
	tests := []struct {
		name           string
		requestBody    string
		setupMocks     func(*providersmocks.MockProviderRegistry, *providersmocks.MockClient, *mcpmocks.MockMCPClientInterface, *mocks.MockLogger, *providersmocks.MockIProvider)
		expectedStatus int
		expectedError  string
	}{
		{
			name:        "Invalid JSON request body",
			requestBody: `invalid json`,
			setupMocks: func(mockRegistry *providersmocks.MockProviderRegistry, mockClient *providersmocks.MockClient, mockMCPClient *mcpmocks.MockMCPClientInterface, mockLogger *mocks.MockLogger, mockProvider *providersmocks.MockIProvider) {
				mockLogger.EXPECT().Debug("mcp middleware invoked", "path", "/v1/chat/completions").AnyTimes()
				mockLogger.EXPECT().Error("failed to parse request body", gomock.Any()).AnyTimes()
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Invalid request body",
		},
		{
			name:        "Unsupported model",
			requestBody: `{"model":"unsupported/model","messages":[]}`,
			setupMocks: func(mockRegistry *providersmocks.MockProviderRegistry, mockClient *providersmocks.MockClient, mockMCPClient *mcpmocks.MockMCPClientInterface, mockLogger *mocks.MockLogger, mockProvider *providersmocks.MockIProvider) {
				mockMCPClient.EXPECT().IsInitialized().Return(true).AnyTimes()
				mockMCPClient.EXPECT().GetAllChatCompletionTools().Return([]providers.ChatCompletionTool{
					{
						Type: providers.ChatCompletionToolTypeFunction,
						Function: providers.FunctionObject{
							Name: "mcp_test_tool",
						},
					},
				}).AnyTimes()
				mockLogger.EXPECT().Debug("mcp middleware invoked", "path", "/v1/chat/completions").AnyTimes()
				mockLogger.EXPECT().Debug("added mcp tools to request", "tool_count", 1).AnyTimes()
				mockLogger.EXPECT().Error("failed to determine provider", gomock.Any(), "model", "unsupported/model").AnyTimes()
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Unsupported model: unsupported/model",
		},
		{
			name:        "Provider build failure",
			requestBody: `{"model":"openai/gpt-3.5-turbo","messages":[]}`,
			setupMocks: func(mockRegistry *providersmocks.MockProviderRegistry, mockClient *providersmocks.MockClient, mockMCPClient *mcpmocks.MockMCPClientInterface, mockLogger *mocks.MockLogger, mockProvider *providersmocks.MockIProvider) {
				mockMCPClient.EXPECT().IsInitialized().Return(true).AnyTimes()
				mockMCPClient.EXPECT().GetAllChatCompletionTools().Return([]providers.ChatCompletionTool{
					{
						Type: providers.ChatCompletionToolTypeFunction,
						Function: providers.FunctionObject{
							Name: "mcp_test_tool",
						},
					},
				}).AnyTimes()
				mockLogger.EXPECT().Debug("mcp middleware invoked", "path", "/v1/chat/completions").AnyTimes()
				mockLogger.EXPECT().Debug("added mcp tools to request", "tool_count", 1).AnyTimes()
				mockRegistry.EXPECT().BuildProvider(providers.OpenaiID, mockClient).Return(nil, fmt.Errorf("provider build failed")).AnyTimes()
				mockLogger.EXPECT().Error("failed to get provider", gomock.Any(), "provider", providers.OpenaiID).AnyTimes()
			},
			expectedStatus: http.StatusInternalServerError,
			expectedError:  "Provider not available",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl, mockRegistry, mockClient, mockMCPClient, mockLogger, mockProvider := createMockDependencies(t)
			defer ctrl.Finish()

			cfg := createTestConfig()

			tt.setupMocks(mockRegistry, mockClient, mockMCPClient, mockLogger, mockProvider)

			mcpAgent := mcp.NewAgent(mockLogger, mockMCPClient)
			middleware, err := middlewares.NewMCPMiddleware(mockRegistry, mockClient, mockMCPClient, mcpAgent, mockLogger, cfg)
			assert.NoError(t, err)

			router := gin.New()
			router.Use(middleware.Middleware())

			handlerCalled := false
			router.POST("/v1/chat/completions", func(c *gin.Context) {
				handlerCalled = true
				c.JSON(http.StatusOK, gin.H{"status": "ok"})
			})

			w := httptest.NewRecorder()
			req := httptest.NewRequest("POST", "/v1/chat/completions", strings.NewReader(tt.requestBody))
			req.Header.Set("Content-Type", "application/json")

			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
			if tt.expectedError != "" && w.Code >= 400 {
				var response map[string]string
				err := json.Unmarshal(w.Body.Bytes(), &response)
				if err == nil {
					assert.Contains(t, response["error"], tt.expectedError)
				}
			}

			if tt.expectedStatus >= 400 {
				assert.False(t, handlerCalled, "Handler should not be called for error responses")
			} else {
				assert.True(t, handlerCalled, "Handler should be called for successful responses")
			}
		})
	}
}

func TestNoopMCPMiddleware(t *testing.T) {
	middleware := &middlewares.NoopMCPMiddlewareImpl{}

	router := gin.New()
	router.Use(middleware.Middleware())

	handlerCalled := false
	router.POST("/v1/chat/completions", func(c *gin.Context) {
		handlerCalled = true
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/v1/chat/completions", strings.NewReader(`{"model":"test","messages":[]}`))
	req.Header.Set("Content-Type", "application/json")

	router.ServeHTTP(w, req)

	assert.True(t, handlerCalled, "NoOp middleware should call next handler")
	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]string
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, "success", response["message"])
}

func TestParseStreamingToolCalls(t *testing.T) {
	ctrl, mockRegistry, mockClient, mockMCPClient, mockLogger, _ := createMockDependencies(t)
	defer ctrl.Finish()

	cfg := createTestConfig()

	mockLogger.EXPECT().Debug(gomock.Any(), gomock.Any()).AnyTimes()
	mockLogger.EXPECT().Info(gomock.Any(), gomock.Any()).AnyTimes()

	mcpAgent := mcp.NewAgent(mockLogger, mockMCPClient)
	middleware, err := middlewares.NewMCPMiddleware(mockRegistry, mockClient, mockMCPClient, mcpAgent, mockLogger, cfg)
	assert.NoError(t, err)

	_, ok := middleware.(*middlewares.MCPMiddlewareImpl)
	assert.True(t, ok, "Expected MCPMiddlewareImpl type")

	tests := []struct {
		name           string
		streamResponse string
		expectedLen    int
		expectedName   string
		expectedArgs   string
	}{
		{
			name: "Parse tool call from streaming chunks",
			streamResponse: `data: {"choices":[{"delta":{"tool_calls":[{"index":0,"id":"call_123","type":"function","function":{"name":"mcp_test_tool"}}]}}]}
data: {"choices":[{"delta":{"tool_calls":[{"index":0,"function":{"arguments":"{\"arg1\""}}]}}]}
data: {"choices":[{"delta":{"tool_calls":[{"index":0,"function":{"arguments":":\"value1\",\"arg2\":42}"}}]}}]}
data: [DONE]`,
			expectedLen:  1,
			expectedName: "mcp_test_tool",
			expectedArgs: `{"arg1":"value1","arg2":42}`,
		},
		{
			name: "Parse multiple tool calls",
			streamResponse: `data: {"choices":[{"delta":{"tool_calls":[{"index":0,"id":"call_1","type":"function","function":{"name":"tool_one"}},{"index":1,"id":"call_2","type":"function","function":{"name":"tool_two"}}]}}]}
data: {"choices":[{"delta":{"tool_calls":[{"index":0,"function":{"arguments":"{\"x\":1}"}},{"index":1,"function":{"arguments":"{\"y\":2}"}}]}}]}
data: [DONE]`,
			expectedLen: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Contains(t, tt.streamResponse, "tool_calls")
			if tt.expectedLen == 1 {
				assert.Contains(t, tt.streamResponse, tt.expectedName)
				assert.Contains(t, tt.streamResponse, "arg1")
			}
		})
	}
}

func TestMCPMiddleware_StreamingWithMultipleToolCallIterations(t *testing.T) {
	t.Run("Multiple tool call iterations should only send one final [DONE]", func(t *testing.T) {
		ctrl, mockRegistry, mockClient, mockMCPClient, mockLogger, mockProvider := createMockDependencies(t)
		defer ctrl.Finish()

		mockMCPClient.EXPECT().IsInitialized().Return(true).AnyTimes()
		mockMCPClient.EXPECT().GetAllChatCompletionTools().Return([]providers.ChatCompletionTool{
			{
				Type: providers.ChatCompletionToolTypeFunction,
				Function: providers.FunctionObject{
					Name: "get-pizza-info",
				},
			},
		}).AnyTimes()

		mockLogger.EXPECT().Debug(gomock.Any(), gomock.Any()).AnyTimes()
		mockLogger.EXPECT().Debug(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
		mockLogger.EXPECT().Debug(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
		mockLogger.EXPECT().Debug(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
		mockLogger.EXPECT().Debug(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
		mockLogger.EXPECT().Info(gomock.Any(), gomock.Any()).AnyTimes()
		mockLogger.EXPECT().Info(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
		mockLogger.EXPECT().Error(gomock.Any(), gomock.Any()).AnyTimes()

		mockRegistry.EXPECT().BuildProvider(providers.GroqID, mockClient).Return(mockProvider, nil).AnyTimes()
		mockProvider.EXPECT().GetName().Return("groq").AnyTimes()

		mockMCPClient.EXPECT().GetServerForTool("get-pizza-info").Return("http://mcp-pizza-server:8084/mcp", nil).AnyTimes()

		mockMCPClient.EXPECT().ExecuteTool(gomock.Any(), gomock.Any(), "http://mcp-pizza-server:8084/mcp").Return(&mcp.CallToolResult{
			Content: []mcp.ContentBlock{
				mcp.TextContent{
					Type: "text",
					Text: "Top pizzas: Margherita, Pepperoni, Hawaiian",
				},
			},
		}, nil).AnyTimes()

		agentImpl := mcp.NewAgent(mockLogger, mockMCPClient)

		agentImpl.SetProvider(mockProvider)
		model := "groq/meta-llama/llama-4-scout-17b-instruct"
		agentImpl.SetModel(&model)

		firstStreamCh := make(chan []byte, 10)
		go func() {
			defer close(firstStreamCh)
			chunks := []string{
				`{"id":"chatcmpl-1","object":"chat.completion.chunk","created":1748534842,"model":"meta-llama/llama-4-scout-17b-instruct","choices":[{"index":0,"delta":{"role":"assistant","content":null},"finish_reason":null}]}`,
				`{"id":"chatcmpl-1","object":"chat.completion.chunk","created":1748534842,"model":"meta-llama/llama-4-scout-17b-instruct","choices":[{"index":0,"delta":{"content":"I'll"},"finish_reason":null}]}`,
				`{"id":"chatcmpl-1","object":"chat.completion.chunk","created":1748534842,"model":"meta-llama/llama-4-scout-17b-instruct","choices":[{"index":0,"delta":{"content":" get"},"finish_reason":null}]}`,
				`{"id":"chatcmpl-1","object":"chat.completion.chunk","created":1748534842,"model":"meta-llama/llama-4-scout-17b-instruct","choices":[{"index":0,"delta":{"content":" pizza"},"finish_reason":null}]}`,
				`{"id":"chatcmpl-1","object":"chat.completion.chunk","created":1748534842,"model":"meta-llama/llama-4-scout-17b-instruct","choices":[{"index":0,"delta":{"content":" info"},"finish_reason":null}]}`,
				`{"id":"chatcmpl-1","object":"chat.completion.chunk","created":1748534842,"model":"meta-llama/llama-4-scout-17b-instruct","choices":[{"index":0,"delta":{"tool_calls":[{"id":"call_vxw1","type":"function","function":{"name":"get-pizza-info","arguments":"{}"},"index":0}]},"finish_reason":null}]}`,
				`{"id":"chatcmpl-1","object":"chat.completion.chunk","created":1748534842,"model":"meta-llama/llama-4-scout-17b-instruct","choices":[{"index":0,"delta":{},"finish_reason":"tool_calls"}]}`,
			}
			for _, chunk := range chunks {
				firstStreamCh <- []byte("data: " + chunk)
			}
			firstStreamCh <- []byte("data: [DONE]")
		}()

		secondStreamCh := make(chan []byte, 10)
		go func() {
			defer close(secondStreamCh)
			chunks := []string{
				`{"id":"chatcmpl-2","object":"chat.completion.chunk","created":1748534842,"model":"meta-llama/llama-4-scout-17b-instruct","choices":[{"index":0,"delta":{"role":"assistant","content":null},"finish_reason":null}]}`,
				`{"id":"chatcmpl-2","object":"chat.completion.chunk","created":1748534842,"model":"meta-llama/llama-4-scout-17b-instruct","choices":[{"index":0,"delta":{"content":"Let"},"finish_reason":null}]}`,
				`{"id":"chatcmpl-2","object":"chat.completion.chunk","created":1748534842,"model":"meta-llama/llama-4-scout-17b-instruct","choices":[{"index":0,"delta":{"content":" me"},"finish_reason":null}]}`,
				`{"id":"chatcmpl-2","object":"chat.completion.chunk","created":1748534842,"model":"meta-llama/llama-4-scout-17b-instruct","choices":[{"index":0,"delta":{"content":" get"},"finish_reason":null}]}`,
				`{"id":"chatcmpl-2","object":"chat.completion.chunk","created":1748534842,"model":"meta-llama/llama-4-scout-17b-instruct","choices":[{"index":0,"delta":{"content":" more"},"finish_reason":null}]}`,
				`{"id":"chatcmpl-2","object":"chat.completion.chunk","created":1748534842,"model":"meta-llama/llama-4-scout-17b-instruct","choices":[{"index":0,"delta":{"tool_calls":[{"id":"call_vxw2","type":"function","function":{"name":"get-pizza-info","arguments":"{}"},"index":0}]},"finish_reason":null}]}`,
				`{"id":"chatcmpl-2","object":"chat.completion.chunk","created":1748534842,"model":"meta-llama/llama-4-scout-17b-instruct","choices":[{"index":0,"delta":{},"finish_reason":"tool_calls"}]}`,
			}
			for _, chunk := range chunks {
				secondStreamCh <- []byte("data: " + chunk)
			}
			secondStreamCh <- []byte("data: [DONE]")
		}()

		thirdStreamCh := make(chan []byte, 10)
		go func() {
			defer close(thirdStreamCh)
			chunks := []string{
				`{"id":"chatcmpl-3","object":"chat.completion.chunk","created":1748534842,"model":"meta-llama/llama-4-scout-17b-instruct","choices":[{"index":0,"delta":{"role":"assistant","content":null},"finish_reason":null}]}`,
				`{"id":"chatcmpl-3","object":"chat.completion.chunk","created":1748534842,"model":"meta-llama/llama-4-scout-17b-instruct","choices":[{"index":0,"delta":{"content":"Based"},"finish_reason":null}]}`,
				`{"id":"chatcmpl-3","object":"chat.completion.chunk","created":1748534842,"model":"meta-llama/llama-4-scout-17b-instruct","choices":[{"index":0,"delta":{"content":" on"},"finish_reason":null}]}`,
				`{"id":"chatcmpl-3","object":"chat.completion.chunk","created":1748534842,"model":"meta-llama/llama-4-scout-17b-instruct","choices":[{"index":0,"delta":{"content":" pizza"},"finish_reason":null}]}`,
				`{"id":"chatcmpl-3","object":"chat.completion.chunk","created":1748534842,"model":"meta-llama/llama-4-scout-17b-instruct","choices":[{"index":0,"delta":{"content":" info"},"finish_reason":null}]}`,
				`{"id":"chatcmpl-3","object":"chat.completion.chunk","created":1748534842,"model":"meta-llama/llama-4-scout-17b-instruct","choices":[{"index":0,"delta":{"content":","},"finish_reason":null}]}`,
				`{"id":"chatcmpl-3","object":"chat.completion.chunk","created":1748534842,"model":"meta-llama/llama-4-scout-17b-instruct","choices":[{"index":0,"delta":{"content":" Margherita"},"finish_reason":null}]}`,
				`{"id":"chatcmpl-3","object":"chat.completion.chunk","created":1748534842,"model":"meta-llama/llama-4-scout-17b-instruct","choices":[{"index":0,"delta":{"content":","},"finish_reason":null}]}`,
				`{"id":"chatcmpl-3","object":"chat.completion.chunk","created":1748534842,"model":"meta-llama/llama-4-scout-17b-instruct","choices":[{"index":0,"delta":{"content":" Pepperoni"},"finish_reason":null}]}`,
				`{"id":"chatcmpl-3","object":"chat.completion.chunk","created":1748534842,"model":"meta-llama/llama-4-scout-17b-instruct","choices":[{"index":0,"delta":{"content":" Hawaiian"},"finish_reason":null}]}`,
				`{"id":"chatcmpl-3","object":"chat.completion.chunk","created":1748534842,"model":"meta-llama/llama-4-scout-17b-instruct","choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}`,
			}
			for _, chunk := range chunks {
				thirdStreamCh <- []byte("data: " + chunk)
			}

			thirdStreamCh <- []byte("data: [DONE]")
		}()

		call1 := mockProvider.EXPECT().StreamChatCompletions(gomock.Any(), gomock.Any()).Return(firstStreamCh, nil).Times(1)
		call2 := mockProvider.EXPECT().StreamChatCompletions(gomock.Any(), gomock.Any()).Return(secondStreamCh, nil).Times(1)
		call3 := mockProvider.EXPECT().StreamChatCompletions(gomock.Any(), gomock.Any()).Return(thirdStreamCh, nil).Times(1)

		gomock.InOrder(call1, call2, call3)

		requestData := providers.CreateChatCompletionRequest{
			Model: "groq/meta-llama/llama-4-scout-17b-instruct",
			Messages: []providers.Message{
				{Role: providers.MessageRoleUser, Content: "What are the top pizzas?"},
			},
			Stream: &[]bool{true}[0],
		}

		middlewareStreamCh := make(chan []byte, 100)
		ctx := context.Background()

		go func() {
			defer close(middlewareStreamCh)
			err := agentImpl.RunWithStream(ctx, middlewareStreamCh, nil, &requestData)
			if err != nil {
				t.Errorf("Agent streaming failed: %v", err)
			}
		}()

		var collectedChunks []string
		var doneCount int
		for chunk := range middlewareStreamCh {
			chunkStr := string(chunk)
			collectedChunks = append(collectedChunks, chunkStr)

			if strings.Contains(chunkStr, "[DONE]") {
				doneCount++
			}
		}

		allChunks := strings.Join(collectedChunks, "")

		assert.Equal(t, 1, doneCount, "Agent should send exactly one final [DONE] marker, but found %d", doneCount)
		assert.Contains(t, allChunks, "pizza", "Response should contain content from first iteration")
		assert.Contains(t, allChunks, "get-pizza-info", "Response should contain tool call")
		assert.Contains(t, allChunks, "Margherita", "Response should contain content from final iteration")
		assert.Contains(t, allChunks, "Pepperoni", "Response should contain content from final iteration")
		assert.Contains(t, allChunks, "Hawaiian", "Response should contain content from final iteration")
		assert.True(t, len(collectedChunks) > 10, "Should have collected multiple chunks from all iterations")
	})
}
