package middleware_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	mcpMiddleware "github.com/inference-gateway/inference-gateway/api/middlewares"
	"github.com/inference-gateway/inference-gateway/config"
	logger "github.com/inference-gateway/inference-gateway/logger"
	mcp "github.com/inference-gateway/inference-gateway/mcp"
	"github.com/inference-gateway/inference-gateway/providers"
	mocks "github.com/inference-gateway/inference-gateway/tests/mocks"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func TestMCPMiddleware(t *testing.T) {
	tests := []struct {
		name            string
		config          config.Config
		requestBody     providers.CreateChatCompletionRequest
		setupExpections func(*gomock.Controller) (logger.Logger, mcp.MCPClientInterface)
		applyAssertions func(*testing.T, error)
	}{
		{
			name:   "MCP Enabled without configured MCP servers",
			config: config.Config{McpServers: "", EnableMcp: true},
			requestBody: providers.CreateChatCompletionRequest{
				Model: "openai/gpt-3.5-turbo",
				Messages: []providers.Message{{
					Role: "user", Content: "Hello"},
				},
			},
			setupExpections: func(ctrl *gomock.Controller) (logger.Logger, mcp.MCPClientInterface) {
				mockLogger := mocks.NewMockLogger(ctrl)
				mockLogger.EXPECT().Debug("no MCP server URLs provided").Times(1)
				mockClient := mocks.NewMockMCPClientInterface(ctrl)
				return mockLogger, mockClient
			},
			applyAssertions: func(t *testing.T, err error) {
				assert.NoError(t, err)
			},
		},
		{
			name:   "MCP Enabled with server but initialization fails",
			config: config.Config{McpServers: "http://mcp-server:8080", EnableMcp: true},
			requestBody: providers.CreateChatCompletionRequest{
				Model: "openai/gpt-3.5-turbo",
				Messages: []providers.Message{{
					Role: "user", Content: "Hello"},
				},
			},
			setupExpections: func(ctrl *gomock.Controller) (logger.Logger, mcp.MCPClientInterface) {
				mockLogger := mocks.NewMockLogger(ctrl)
				mockLogger.EXPECT().Error("Failed to initialize MCP client", gomock.Any()).Times(1)

				mockClient := mocks.NewMockMCPClientInterface(ctrl)
				mockClient.EXPECT().InitializeAll(gomock.Any()).Return(errors.New("initialization error")).Times(1)
				return mockLogger, mockClient
			},
			applyAssertions: func(t *testing.T, err error) {
				assert.Error(t, err)
				assert.Equal(t, "failed to initialize MCP client: initialization error", err.Error())
			},
		},
		{
			name:   "MCP Enabled with server and successful initialization",
			config: config.Config{McpServers: "http://mcp-server:8080", EnableMcp: true},
			requestBody: providers.CreateChatCompletionRequest{
				Model: "openai/gpt-3.5-turbo",
				Messages: []providers.Message{{
					Role: "user", Content: "Hello"},
				},
			},
			setupExpections: func(ctrl *gomock.Controller) (logger.Logger, mcp.MCPClientInterface) {
				mockLogger := mocks.NewMockLogger(ctrl)
				mockLogger.EXPECT().Debug(gomock.Any(), gomock.Any()).AnyTimes()
				mockLogger.EXPECT().Error(gomock.Any(), gomock.Any()).AnyTimes()

				mockClient := mocks.NewMockMCPClientInterface(ctrl)
				mockClient.EXPECT().InitializeAll(gomock.Any()).Return(nil).Times(1)
				mockClient.EXPECT().IsInitialized().Return(true).AnyTimes()
				mockClient.EXPECT().DiscoverCapabilities(gomock.Any()).Return(nil, nil).AnyTimes()
				return mockLogger, mockClient
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			logger, client := tt.setupExpections(ctrl)
			middleware, err := mcpMiddleware.NewMCPMiddleware(client, logger, tt.config)

			if tt.applyAssertions != nil {
				tt.applyAssertions(t, err)
			}

			if err != nil {
				return
			}

			r := gin.New()
			r.Use(middleware.Middleware())
			r.POST("/v1/chat/completions", func(c *gin.Context) {
				c.JSON(http.StatusOK, gin.H{"message": "pong"})
			})

			requestBodyBytes, err := json.Marshal(tt.requestBody)
			if err != nil {
				t.Fatalf("Failed to marshal request body: %v", err)
			}
			req, _ := http.NewRequest(http.MethodPost, "/v1/chat/completions", io.NopCloser(bytes.NewBuffer(requestBodyBytes)))

			resp := httptest.NewRecorder()
			r.ServeHTTP(resp, req)
		})
	}
}

func TestMCPMiddlewareRequestProcessing(t *testing.T) {
	tests := []struct {
		name            string
		requestBody     providers.CreateChatCompletionRequest
		headers         map[string]string
		setupClient     func(*gomock.Controller) (logger.Logger, mcp.MCPClientInterface)
		expectedStatus  int
		validateRequest func(*testing.T, *httptest.ResponseRecorder, providers.CreateChatCompletionRequest)
	}{
		{
			name: "Request processed with MCP capabilities added",
			requestBody: providers.CreateChatCompletionRequest{
				Model: "openai/gpt-3.5-turbo",
				Messages: []providers.Message{{
					Role: "user", Content: "Hello"},
				},
			},
			setupClient: func(ctrl *gomock.Controller) (logger.Logger, mcp.MCPClientInterface) {
				mockLogger := mocks.NewMockLogger(ctrl)
				mockLogger.EXPECT().Debug("MCP Processing request for MCP enhancement").Times(1)
				mockLogger.EXPECT().Debug("MCP Checking if request is a streaming request").Times(1)
				mockLogger.EXPECT().Debug("MCP Request is a streaming request:", false).Times(1)
				mockLogger.EXPECT().Debug("MCP Checking if request contains messages").Times(1)
				mockLogger.EXPECT().Debug("MCP Discovering MCP capabilities").Times(1)
				mockLogger.EXPECT().Debug("MCP Extracting tools from capabilities").Times(1)

				mockClient := mocks.NewMockMCPClientInterface(ctrl)
				mockClient.EXPECT().InitializeAll(gomock.Any()).Return(nil).Times(1)
				mockClient.EXPECT().IsInitialized().Return(true).AnyTimes()

				capabilities := []map[string]interface{}{
					{
						"_server_url": "http://mcp-server:8080",
						"tools": []providers.ChatCompletionTool{
							{
								Type: "function",
								Function: providers.FunctionObject{
									Name:        "test_tool",
									Description: strPtr("Test tool description"),
									Parameters: &providers.FunctionParameters{
										"type":       "object",
										"properties": map[string]interface{}{},
									},
								},
							},
						},
					},
				}
				mockClient.EXPECT().DiscoverCapabilities(gomock.Any()).Return(capabilities, nil).Times(1)

				return mockLogger, mockClient
			},
			expectedStatus: http.StatusOK,
			validateRequest: func(t *testing.T, resp *httptest.ResponseRecorder, body providers.CreateChatCompletionRequest) {
				assert.Equal(t, http.StatusOK, resp.Code)

				type Tool struct {
					Name        string                 `json:"name"`
					Description string                 `json:"description,omitempty"`
					Parameters  map[string]interface{} `json:"parameters,omitempty"`
				}

				type RequestReceived struct {
					Model    string `json:"model"`
					Messages []any  `json:"messages"`
					Tools    []Tool `json:"tools"`
				}

				type Response struct {
					Message         string          `json:"message"`
					RequestReceived RequestReceived `json:"request_received"`
				}

				var respData Response
				err := json.Unmarshal(resp.Body.Bytes(), &respData)
				assert.NoError(t, err)

				assert.NotEmpty(t, respData.RequestReceived.Tools, "Tools should not be empty")
				assert.Equal(t, "test_tool", respData.RequestReceived.Tools[0].Name)
			},
		},
		{
			name: "Request without messages passes through",
			requestBody: providers.CreateChatCompletionRequest{
				Model:    "openai/gpt-3.5-turbo",
				Messages: []providers.Message{},
			},
			setupClient: func(ctrl *gomock.Controller) (logger.Logger, mcp.MCPClientInterface) {
				mockLogger := mocks.NewMockLogger(ctrl)
				mockLogger.EXPECT().Debug("MCP Processing request for MCP enhancement").Times(1)
				mockLogger.EXPECT().Debug("MCP Checking if request is a streaming request").Times(1)
				mockLogger.EXPECT().Debug("MCP Request is a streaming request:", false).Times(1)
				mockLogger.EXPECT().Debug("MCP Checking if request contains messages").Times(1)
				mockLogger.EXPECT().Debug("MCP No messages found in request, continuing without MCP enhancement").Times(1)

				mockClient := mocks.NewMockMCPClientInterface(ctrl)
				mockClient.EXPECT().InitializeAll(gomock.Any()).Return(nil).Times(1)
				mockClient.EXPECT().IsInitialized().Return(true).AnyTimes()

				return mockLogger, mockClient
			},
			expectedStatus: http.StatusOK,
			validateRequest: func(t *testing.T, resp *httptest.ResponseRecorder, body providers.CreateChatCompletionRequest) {
				assert.Equal(t, http.StatusOK, resp.Code)

				type RequestReceived struct {
					Model    string                         `json:"model"`
					Messages []providers.Message            `json:"messages"`
					Tools    []providers.ChatCompletionTool `json:"tools,omitempty"`
				}

				type Response struct {
					Message         string          `json:"message"`
					RequestReceived RequestReceived `json:"request_received"`
				}

				var respData Response
				err := json.Unmarshal(resp.Body.Bytes(), &respData)
				assert.NoError(t, err)

				assert.Empty(t, respData.RequestReceived.Tools, "tools should be empty or nil")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			logger, client := tt.setupClient(ctrl)

			config := config.Config{
				McpServers: "http://mcp-server:8080",
				EnableMcp:  true,
			}

			middleware, err := mcpMiddleware.NewMCPMiddleware(client, logger, config)
			assert.NoError(t, err)

			r := gin.New()
			r.Use(middleware.Middleware())
			r.POST("/v1/chat/completions", func(c *gin.Context) {
				var requestBody providers.CreateChatCompletionRequest
				err := c.ShouldBindJSON(&requestBody)
				assert.NoError(t, err)
				c.JSON(http.StatusOK, gin.H{
					"message":          "processed",
					"request_received": requestBody,
				})
			})

			requestBodyBytes, err := json.Marshal(tt.requestBody)
			if err != nil {
				t.Fatalf("Failed to marshal request body: %v", err)
			}
			req, _ := http.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(requestBodyBytes))
			req.Header.Set("Content-Type", "application/json")

			for k, v := range tt.headers {
				req.Header.Set(k, v)
			}

			resp := httptest.NewRecorder()
			r.ServeHTTP(resp, req)

			tt.validateRequest(t, resp, tt.requestBody)
		})
	}
}

func TestMCPMiddlewareResponseProcessing(t *testing.T) {
	tests := []struct {
		name           string
		requestBody    providers.CreateChatCompletionRequest
		responseBody   providers.CreateChatCompletionResponse
		headers        map[string]string
		setupClient    func(*gomock.Controller) (logger.Logger, mcp.MCPClientInterface)
		expectedStatus int
		validateResult func(*testing.T, *httptest.ResponseRecorder)
	}{
		{
			name: "Process response with tool calls",
			requestBody: providers.CreateChatCompletionRequest{
				Model:    "openai/gpt-3.5-turbo",
				Messages: []providers.Message{{Role: "user", Content: "Use tool"}},
			},
			responseBody: providers.CreateChatCompletionResponse{
				ID: "response-123",
				Choices: []providers.ChatCompletionChoice{
					{
						Index: 0,
						Message: providers.Message{
							Role:    "assistant",
							Content: "I'll use the tool",
							ToolCalls: &[]providers.ChatCompletionMessageToolCall{
								{
									ID:   "tool-call-1",
									Type: "function",
									Function: providers.ChatCompletionMessageToolCallFunction{
										Name:      "test_tool",
										Arguments: `{"param1": "value1"}`,
									},
								},
							},
						},
					},
				},
			},
			setupClient: func(ctrl *gomock.Controller) (logger.Logger, mcp.MCPClientInterface) {
				mockLogger := mocks.NewMockLogger(ctrl)
				mockLogger.EXPECT().Debug(gomock.Any(), gomock.Any()).AnyTimes()
				mockLogger.EXPECT().Error(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

				mockClient := mocks.NewMockMCPClientInterface(ctrl)
				mockClient.EXPECT().InitializeAll(gomock.Any()).Return(nil).Times(1)
				mockClient.EXPECT().IsInitialized().Return(true).AnyTimes()
				mockClient.EXPECT().DiscoverCapabilities(gomock.Any()).Return([]map[string]interface{}{
					{
						"_server_url": "http://mcp-server:8080",
						"tools": []interface{}{
							map[string]interface{}{
								"name": "test_tool",
							},
						},
					},
				}, nil).Times(1)

				mockClient.EXPECT().GetServerCapabilities().Return(map[string]map[string]interface{}{
					"http://mcp-server:8080": {
						"tools": []interface{}{
							map[string]interface{}{
								"name": "test_tool",
							},
						},
					},
				}).Times(1)

				toolResult := map[string]interface{}{
					"content": []interface{}{
						map[string]interface{}{
							"type": "text",
							"text": "Tool executed successfully",
						},
					},
				}
				mockClient.EXPECT().ExecuteTool(gomock.Any(), "test_tool", gomock.Any(), "http://mcp-server:8080").Return(toolResult, nil).Times(1)

				return mockLogger, mockClient
			},
			expectedStatus: http.StatusOK,
			validateResult: func(t *testing.T, resp *httptest.ResponseRecorder) {
				assert.Equal(t, http.StatusOK, resp.Code)

				type ToolContent struct {
					Type string `json:"type"`
					Text string `json:"text"`
				}

				type ToolCallFunction struct {
					Name      string `json:"name"`
					Arguments string `json:"arguments"`
				}

				type ToolCall struct {
					ID       string           `json:"id"`
					Type     string           `json:"type"`
					Function ToolCallFunction `json:"function"`
				}

				type Message struct {
					Role      string     `json:"role"`
					Content   string     `json:"content,omitempty"`
					ToolCalls []ToolCall `json:"tool_calls,omitempty"`
				}

				type Choice struct {
					Index   int     `json:"index"`
					Message Message `json:"message"`
				}

				type ResponseData struct {
					ID       string    `json:"id,omitempty"`
					Choices  []Choice  `json:"choices,omitempty"`
					Messages []Message `json:"messages,omitempty"`
				}

				type Result struct {
					ResponseData ResponseData `json:"response_data"`
				}

				var result Result
				err := json.Unmarshal(resp.Body.Bytes(), &result)
				assert.NoError(t, err)

				if len(result.ResponseData.Messages) > 0 {
					assert.NotEmpty(t, result.ResponseData.Messages, "messages should contain at least one tool response")
					assert.Equal(t, "tool", result.ResponseData.Messages[0].Role, "message role should be 'tool'")
					assert.Contains(t, result.ResponseData.Messages[0].Content, "Tool executed successfully")
				} else {
					assert.NotEmpty(t, result.ResponseData.Choices, "choices should not be empty")
					assert.NotEmpty(t, result.ResponseData.Choices[0].Message.ToolCalls, "tool_calls should not be empty")
				}
			},
		},
		{
			name: "Response without tool calls is passed through",
			requestBody: providers.CreateChatCompletionRequest{
				Model:    "openai/gpt-3.5-turbo",
				Messages: []providers.Message{{Role: "user", Content: "No tools"}},
			},
			responseBody: providers.CreateChatCompletionResponse{
				ID: "response-123",
				Choices: []providers.ChatCompletionChoice{
					{
						Index: 0,
						Message: providers.Message{
							Role:    "assistant",
							Content: "No tool calls here",
						},
					},
				},
			},
			setupClient: func(ctrl *gomock.Controller) (logger.Logger, mcp.MCPClientInterface) {
				mockLogger := mocks.NewMockLogger(ctrl)
				mockLogger.EXPECT().Debug("MCP Processing request for MCP enhancement").Times(1)
				mockLogger.EXPECT().Debug("MCP Checking if request is a streaming request").Times(1)
				mockLogger.EXPECT().Debug("MCP Request is a streaming request:", false).Times(1)
				mockLogger.EXPECT().Debug("MCP Checking if request contains messages").Times(1)
				mockLogger.EXPECT().Debug("MCP Discovering MCP capabilities").Times(1)
				mockLogger.EXPECT().Debug("MCP Extracting tools from capabilities").Times(1)

				mockClient := mocks.NewMockMCPClientInterface(ctrl)
				mockClient.EXPECT().InitializeAll(gomock.Any()).Return(nil).Times(1)
				mockClient.EXPECT().IsInitialized().Return(true).AnyTimes()
				mockClient.EXPECT().DiscoverCapabilities(gomock.Any()).Return([]map[string]interface{}{
					{
						"_server_url": "http://mcp-server:8080",
						"tools": []interface{}{
							map[string]interface{}{
								"name": "test_tool",
							},
						},
					},
				}, nil).Times(1)

				return mockLogger, mockClient
			},
			expectedStatus: http.StatusOK,
			validateResult: func(t *testing.T, resp *httptest.ResponseRecorder) {
				assert.Equal(t, http.StatusOK, resp.Code)

				type Message struct {
					Role    string `json:"role"`
					Content string `json:"content"`
				}

				type Choice struct {
					Index   int     `json:"index"`
					Message Message `json:"message"`
				}

				type ResponseData struct {
					ID      string   `json:"id"`
					Choices []Choice `json:"choices"`
				}

				type Result struct {
					ResponseData ResponseData `json:"response_data"`
				}

				var result Result
				err := json.Unmarshal(resp.Body.Bytes(), &result)
				assert.NoError(t, err)

				assert.Equal(t, "response-123", result.ResponseData.ID)
				assert.Equal(t, 1, len(result.ResponseData.Choices))
				assert.Equal(t, "No tool calls here", result.ResponseData.Choices[0].Message.Content)

				var rawResult map[string]interface{}
				err = json.Unmarshal(resp.Body.Bytes(), &rawResult)
				assert.NoError(t, err)

				responseDataMap, ok := rawResult["response_data"].(map[string]interface{})
				assert.True(t, ok)

				_, hasMessages := responseDataMap["messages"]
				assert.False(t, hasMessages, "messages field should not exist in the response")
			},
		},
		{
			name: "Streaming request gets processed differently",
			requestBody: providers.CreateChatCompletionRequest{
				Model:    "openai/gpt-3.5-turbo",
				Messages: []providers.Message{{Role: "user", Content: "Stream"}},
				Stream:   boolPtr(true),
			},
			responseBody: providers.CreateChatCompletionResponse{},
			headers: map[string]string{
				"Accept": "text/event-stream",
			},
			setupClient: func(ctrl *gomock.Controller) (logger.Logger, mcp.MCPClientInterface) {
				mockLogger := mocks.NewMockLogger(ctrl)
				mockLogger.EXPECT().Debug("MCP Processing request for MCP enhancement").Times(1)
				mockLogger.EXPECT().Debug("MCP Checking if request is a streaming request").Times(1)
				mockLogger.EXPECT().Debug("MCP Request is a streaming request:", true).Times(1)
				mockLogger.EXPECT().Debug("MCP Checking if request contains messages").Times(1)
				mockLogger.EXPECT().Debug("MCP Discovering MCP capabilities").Times(1)
				mockLogger.EXPECT().Debug("MCP Extracting tools from capabilities").Times(1)
				mockLogger.EXPECT().Debug("MCP streaming response processing not fully implemented yet").Times(1)

				mockClient := mocks.NewMockMCPClientInterface(ctrl)
				mockClient.EXPECT().InitializeAll(gomock.Any()).Return(nil).Times(1)
				mockClient.EXPECT().IsInitialized().Return(true).AnyTimes()
				mockClient.EXPECT().DiscoverCapabilities(gomock.Any()).Return([]map[string]interface{}{
					{
						"_server_url": "http://mcp-server:8080",
						"tools": []interface{}{
							map[string]interface{}{
								"name": "test_tool",
							},
						},
					},
				}, nil).Times(1)

				return mockLogger, mockClient
			},
			expectedStatus: http.StatusOK,
			validateResult: func(t *testing.T, resp *httptest.ResponseRecorder) {
				assert.Equal(t, http.StatusOK, resp.Code)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			logger, client := tt.setupClient(ctrl)

			config := config.Config{
				McpServers: "http://mcp-server:8080",
				EnableMcp:  true,
			}

			middleware, err := mcpMiddleware.NewMCPMiddleware(client, logger, config)
			assert.NoError(t, err)

			r := gin.New()
			r.Use(middleware.Middleware())
			r.POST("/v1/chat/completions", func(c *gin.Context) {
				if tt.responseBody.ID != "" || len(tt.responseBody.Choices) > 0 {
					c.Set("response_data", tt.responseBody)
				}
				responseData, exists := c.Get("response_data")
				if exists {
					c.JSON(http.StatusOK, gin.H{
						"response_data": responseData,
					})
				} else {
					c.JSON(http.StatusOK, gin.H{
						"message": "No response data found",
					})
				}
			})

			requestBodyBytes, _ := json.Marshal(tt.requestBody)
			req, _ := http.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewBuffer(requestBodyBytes))
			req.Header.Set("Content-Type", "application/json")

			for k, v := range tt.headers {
				req.Header.Set(k, v)
			}

			resp := httptest.NewRecorder()
			r.ServeHTTP(resp, req)

			tt.validateResult(t, resp)
		})
	}
}

func boolPtr(b bool) *bool {
	return &b
}

func strPtr(s string) *string {
	return &s
}
