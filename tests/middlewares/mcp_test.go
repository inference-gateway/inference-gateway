package middleware_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	mcpMiddleware "github.com/inference-gateway/inference-gateway/api/middlewares"
	"github.com/inference-gateway/inference-gateway/config"
	logger "github.com/inference-gateway/inference-gateway/logger"
	mcp "github.com/inference-gateway/inference-gateway/mcp"
	mocks "github.com/inference-gateway/inference-gateway/tests/mocks"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func TestMCPMiddleware(t *testing.T) {
	tests := []struct {
		name            string
		config          config.Config
		requestBody     string
		setupExpections func(*gomock.Controller) (logger.Logger, mcp.MCPClientInterface)
		applyAssertions func(*testing.T, error)
	}{
		{
			name:        "MCP Enabled without configured MCP servers",
			config:      config.Config{McpServers: "", EnableMcp: true},
			requestBody: `{"model":"openai/gpt-3.5-turbo","messages":[{"role":"user","content":"Hello"}]}`,
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
			name:        "MCP Enabled with server but initialization fails",
			config:      config.Config{McpServers: "http://mcp-server:8080", EnableMcp: true},
			requestBody: `{"model":"openai/gpt-3.5-turbo","messages":[{"role":"user","content":"Hello"}]}`,
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
			name:        "MCP Enabled with server and successful initialization",
			config:      config.Config{McpServers: "http://mcp-server:8080", EnableMcp: true},
			requestBody: `{"model":"openai/gpt-3.5-turbo","messages":[{"role":"user","content":"Hello"}]}`,
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

			req, _ := http.NewRequest(http.MethodPost, "/v1/chat/completions", io.NopCloser(strings.NewReader(tt.requestBody)))

			resp := httptest.NewRecorder()
			r.ServeHTTP(resp, req)
		})
	}
}

func TestMCPMiddlewareRequestProcessing(t *testing.T) {
	tests := []struct {
		name            string
		requestBody     string
		headers         map[string]string
		setupClient     func(*gomock.Controller) (logger.Logger, mcp.MCPClientInterface)
		expectedStatus  int
		validateRequest func(*testing.T, *httptest.ResponseRecorder, string)
	}{
		{
			name:        "Request processed with MCP capabilities added",
			requestBody: `{"model":"openai/gpt-3.5-turbo","messages":[{"role":"user","content":"Hello"}]}`,
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
						"tools": []interface{}{
							map[string]interface{}{
								"name":        "test_tool",
								"description": "A test tool",
								"parameters": map[string]interface{}{
									"type":       "object",
									"properties": map[string]interface{}{},
								},
							},
						},
					},
				}
				mockClient.EXPECT().DiscoverCapabilities(gomock.Any()).Return(capabilities, nil).Times(1)

				return mockLogger, mockClient
			},
			expectedStatus: http.StatusOK,
			validateRequest: func(t *testing.T, resp *httptest.ResponseRecorder, body string) {
				assert.Equal(t, http.StatusOK, resp.Code)

				var respBody map[string]interface{}
				json.Unmarshal(resp.Body.Bytes(), &respBody)

				requestReceived, ok := respBody["request_received"].(map[string]interface{})
				assert.True(t, ok)

				tools, ok := requestReceived["tools"].([]interface{})
				assert.True(t, ok)
				assert.NotEmpty(t, tools)

				tool := tools[0].(map[string]interface{})
				assert.Equal(t, "test_tool", tool["name"])
			},
		},
		{
			name:        "Invalid request body does not cause error",
			requestBody: `{"invalid json`,
			setupClient: func(ctrl *gomock.Controller) (logger.Logger, mcp.MCPClientInterface) {
				mockLogger := mocks.NewMockLogger(ctrl)
				mockLogger.EXPECT().Debug("MCP Processing request for MCP enhancement").Times(1)
				mockLogger.EXPECT().Debug("Could not parse request body for MCP enhancement", gomock.Any()).Times(1)

				mockClient := mocks.NewMockMCPClientInterface(ctrl)
				mockClient.EXPECT().InitializeAll(gomock.Any()).Return(nil).Times(1)
				mockClient.EXPECT().IsInitialized().Return(true).AnyTimes()

				return mockLogger, mockClient
			},
			expectedStatus: http.StatusOK,
			validateRequest: func(t *testing.T, resp *httptest.ResponseRecorder, body string) {
				assert.Equal(t, http.StatusOK, resp.Code)
			},
		},
		{
			name:        "Request without messages passes through",
			requestBody: `{"model":"openai/gpt-3.5-turbo"}`,
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
			validateRequest: func(t *testing.T, resp *httptest.ResponseRecorder, body string) {
				assert.Equal(t, http.StatusOK, resp.Code)

				var respBody map[string]interface{}
				err := json.Unmarshal(resp.Body.Bytes(), &respBody)
				assert.NoError(t, err)

				requestReceived, exists := respBody["request_received"]
				assert.True(t, exists, "request_received should exist in response")

				if requestReceived != nil {
					requestMap, ok := requestReceived.(map[string]interface{})
					assert.True(t, ok, "request_received should be a map")

					_, toolsPresent := requestMap["tools"]
					assert.False(t, toolsPresent, "tools should not be present")
				}
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
				var requestBody map[string]interface{}
				c.ShouldBindJSON(&requestBody)
				c.JSON(http.StatusOK, gin.H{
					"message":          "processed",
					"request_received": requestBody,
				})
			})

			req, _ := http.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewBuffer([]byte(tt.requestBody)))
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
		requestBody    string
		responseBody   map[string]interface{}
		headers        map[string]string
		setupClient    func(*gomock.Controller) (logger.Logger, mcp.MCPClientInterface)
		expectedStatus int
		validateResult func(*testing.T, *httptest.ResponseRecorder)
	}{
		{
			name:        "Process response with tool calls",
			requestBody: `{"model":"openai/gpt-3.5-turbo","messages":[{"role":"user","content":"Use tool"}]}`,
			responseBody: map[string]interface{}{
				"id": "response-123",
				"choices": []interface{}{
					map[string]interface{}{
						"message": map[string]interface{}{
							"role":    "assistant",
							"content": "I'll use the tool",
							"tool_calls": []interface{}{
								map[string]interface{}{
									"id":   "tool-call-1",
									"type": "function",
									"function": map[string]interface{}{
										"name":      "test_tool",
										"arguments": `{"param1": "value1"}`,
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

				var result map[string]interface{}
				err := json.Unmarshal(resp.Body.Bytes(), &result)
				assert.NoError(t, err)

				responseData, exists := result["response_data"]
				assert.True(t, exists, "response_data should exist in the response")

				respMap, ok := responseData.(map[string]interface{})
				assert.True(t, ok, "response_data should be a map")

				if messages, hasMessages := respMap["messages"]; hasMessages {
					msgArray, ok := messages.([]interface{})
					assert.True(t, ok, "messages should be an array")
					assert.NotEmpty(t, msgArray, "messages should contain at least one tool response")

					if len(msgArray) > 0 {
						firstMsg, ok := msgArray[0].(map[string]interface{})
						if assert.True(t, ok, "message should be a map") {
							assert.Equal(t, "tool", firstMsg["role"], "message role should be 'tool'")
							assert.Contains(t, firstMsg["content"], "Tool executed successfully")
						}
					}
				} else {
					assert.Contains(t, respMap, "choices", "response should have choices")

					choices, ok := respMap["choices"].([]interface{})
					assert.True(t, ok, "choices should be an array")
					assert.NotEmpty(t, choices, "choices should not be empty")

					if len(choices) == 0 {
						return
					}

					firstChoice, ok := choices[0].(map[string]interface{})
					assert.True(t, ok, "choice should be a map")

					message, hasMessage := firstChoice["message"].(map[string]interface{})
					if assert.True(t, hasMessage, "choice should have a message") {
						if toolCalls, hasToolCalls := message["tool_calls"].([]interface{}); hasToolCalls {
							assert.NotEmpty(t, toolCalls, "tool_calls should not be empty")
						}
					}
				}
			},
		},
		{
			name:        "Response without tool calls is passed through",
			requestBody: `{"model":"openai/gpt-3.5-turbo","messages":[{"role":"user","content":"No tools"}]}`,
			responseBody: map[string]interface{}{
				"id": "response-123",
				"choices": []interface{}{
					map[string]interface{}{
						"message": map[string]interface{}{
							"role":    "assistant",
							"content": "No tool calls here",
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

				var result map[string]interface{}
				json.Unmarshal(resp.Body.Bytes(), &result)

				responseData, ok := result["response_data"].(map[string]interface{})
				assert.True(t, ok)

				_, hasMessages := responseData["messages"]
				assert.False(t, hasMessages)
			},
		},
		{
			name:         "Streaming request gets processed differently",
			requestBody:  `{"model":"openai/gpt-3.5-turbo","messages":[{"role":"user","content":"Stream"}],"stream":true}`,
			responseBody: nil,
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
				if tt.responseBody != nil {
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

			req, _ := http.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewBuffer([]byte(tt.requestBody)))
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
