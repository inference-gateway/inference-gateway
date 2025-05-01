package middlewares_test

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/inference-gateway/inference-gateway/api/middlewares"
	"github.com/inference-gateway/inference-gateway/logger"
	middlewareMocks "github.com/inference-gateway/inference-gateway/tests/mocks/middlewares"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// setupGinContext creates a new Gin context for testing with optional request body
func setupGinContext(t *testing.T, method, path, body string, headers map[string]string) (*gin.Context, *httptest.ResponseRecorder) {
	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)

	var req *http.Request
	if body != "" {
		req, _ = http.NewRequest(method, path, strings.NewReader(body))
	} else {
		req, _ = http.NewRequest(method, path, nil)
	}

	req.Header.Set("Content-Type", "application/json")

	for key, value := range headers {
		req.Header.Set(key, value)
	}

	ctx.Request = req
	return ctx, w
}

func TestMCPMiddleware(t *testing.T) {
	log, err := logger.NewLogger("test")
	require.NoError(t, err)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockClient := middlewareMocks.NewMockMCPClientInterface(ctrl)

	tests := []struct {
		name                     string
		setupMock                func(client *middlewareMocks.MockMCPClientInterface)
		requestBody              string
		requestHeaders           map[string]string
		mcpEnabled               bool
		expectedStatus           int
		expectedResponseContains string
		expectedToolCalls        bool
		expectedSSE              bool
		expectErrorAbort         bool
	}{
		{
			name:                     "MCP Disabled - Request Passes Through",
			setupMock:                func(client *middlewareMocks.MockMCPClientInterface) {},
			requestBody:              `{"model": "gpt-4", "messages": [{"role": "user", "content": "Hello"}]}`,
			mcpEnabled:               false,
			expectedStatus:           http.StatusOK,
			expectedResponseContains: `"model":"gpt-4"`,
			expectedToolCalls:        false,
		},
		{
			name: "Request Enhancement Error - Capabilities Failure",
			setupMock: func(client *middlewareMocks.MockMCPClientInterface) {
				client.EXPECT().
					DiscoverCapabilities(gomock.Any()).
					Return(nil, errors.New("failed to discover capabilities"))
			},
			requestBody:              `{"model": "gpt-4", "messages": [{"role": "user", "content": "Hello"}]}`,
			mcpEnabled:               true,
			expectedStatus:           http.StatusInternalServerError,
			expectedResponseContains: `"Failed to process MCP capabilities"`,
			expectedToolCalls:        false,
			expectErrorAbort:         true,
		},
		{
			name: "Request Enhancement Error - No Tools Found",
			setupMock: func(client *middlewareMocks.MockMCPClientInterface) {
				client.EXPECT().
					DiscoverCapabilities(gomock.Any()).
					Return([]map[string]interface{}{
						{"_server_url": "http://mcp-server"},
					}, nil)
			},
			requestBody:              `{"model": "gpt-4", "messages": [{"role": "user", "content": "Hello"}]}`,
			mcpEnabled:               true,
			expectedStatus:           http.StatusInternalServerError,
			expectedResponseContains: `"Failed to process MCP capabilities"`,
			expectedToolCalls:        false,
			expectErrorAbort:         true,
		},
		{
			name: "Successful Request Enhancement - No Tool Calls",
			setupMock: func(client *middlewareMocks.MockMCPClientInterface) {
				client.EXPECT().
					DiscoverCapabilities(gomock.Any()).
					Return([]map[string]interface{}{
						{
							"_server_url": "http://mcp-server",
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
					}, nil)
			},
			requestBody:              `{"model": "gpt-4", "messages": [{"role": "user", "content": "Hello"}]}`,
			mcpEnabled:               true,
			expectedStatus:           http.StatusOK,
			expectedResponseContains: `"model":"gpt-4"`,
			expectedToolCalls:        false,
		},
		{
			name: "Successful Tool Call Processing",
			setupMock: func(client *middlewareMocks.MockMCPClientInterface) {
				client.EXPECT().
					DiscoverCapabilities(gomock.Any()).
					Return([]map[string]interface{}{
						{
							"_server_url": "http://mcp-server",
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
					}, nil)

				client.EXPECT().
					ExecuteTool(
						gomock.Any(),
						"test_tool",
						`{"query":"test"}`,
						"http://mcp-server",
					).
					Return(map[string]interface{}{
						"result": "Tool executed successfully",
					}, nil)
			},
			requestBody:              `{"model": "gpt-4", "messages": [{"role": "user", "content": "Call a tool"}]}`,
			mcpEnabled:               true,
			expectedStatus:           http.StatusOK,
			expectedResponseContains: `"content":"Tool 'test_tool' result:`,
			expectedToolCalls:        true,
		},
		{
			name: "Tool Call Error",
			setupMock: func(client *middlewareMocks.MockMCPClientInterface) {
				client.EXPECT().
					DiscoverCapabilities(gomock.Any()).
					Return([]map[string]interface{}{
						{
							"_server_url": "http://mcp-server",
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
					}, nil)

				client.EXPECT().
					ExecuteTool(
						gomock.Any(),
						"test_tool",
						`{"query":"test"}`,
						"http://mcp-server",
					).
					Return(nil, errors.New("tool execution failed"))
			},
			requestBody:              `{"model": "gpt-4", "messages": [{"role": "user", "content": "Call a tool"}]}`,
			mcpEnabled:               true,
			expectedStatus:           http.StatusOK,
			expectedResponseContains: `"content":"Error executing tool: tool execution failed"`,
			expectedToolCalls:        true,
		},
		{
			name: "Missing Tool in Server Map",
			setupMock: func(client *middlewareMocks.MockMCPClientInterface) {
				client.EXPECT().
					DiscoverCapabilities(gomock.Any()).
					Return([]map[string]interface{}{
						{
							"_server_url": "http://mcp-server",
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
					}, nil)
			},
			requestBody:              `{"model": "gpt-4", "messages": [{"role": "user", "content": "Call a tool"}]}`,
			mcpEnabled:               true,
			expectedStatus:           http.StatusOK,
			expectedResponseContains: `"content":"Error: Tool 'unknown_tool' not found in any MCP server"`,
			expectedToolCalls:        true,
		},
		{
			name: "SSE Notifications For Tool Calls",
			setupMock: func(client *middlewareMocks.MockMCPClientInterface) {
				client.EXPECT().
					DiscoverCapabilities(gomock.Any()).
					Return([]map[string]interface{}{
						{
							"_server_url": "http://mcp-server",
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
					}, nil)

				client.EXPECT().
					ExecuteTool(
						gomock.Any(),
						"test_tool",
						`{"query":"test"}`,
						"http://mcp-server",
					).
					Return(map[string]interface{}{
						"result": "Tool executed successfully",
					}, nil)
			},
			requestBody:              `{"model": "gpt-4", "messages": [{"role": "user", "content": "Call a tool"}]}`,
			requestHeaders:           map[string]string{"Accept": "text/event-stream"},
			mcpEnabled:               true,
			expectedStatus:           http.StatusOK,
			expectedResponseContains: `event:tool_call_progress`,
			expectedToolCalls:        true,
			expectedSSE:              true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mcpMiddleware := &middlewares.MCPMiddleware{
				MCPClient:     mockClient,
				Logger:        log,
				Enabled:       tt.mcpEnabled,
				ToolServerMap: make(map[string]string),
			}

			tt.setupMock(mockClient)

			router := gin.New()
			router.Use(mcpMiddleware.Middleware())

			if tt.expectedToolCalls {
				router.POST("/test", func(c *gin.Context) {
					var toolName string
					if strings.Contains(tt.name, "Missing Tool") {
						toolName = "unknown_tool"
					} else {
						toolName = "test_tool"
					}

					c.JSON(http.StatusOK, map[string]interface{}{
						"choices": []map[string]interface{}{
							{
								"message": map[string]interface{}{
									"tool_calls": []map[string]interface{}{
										{
											"id":   "tool_call_id",
											"type": "function",
											"function": map[string]interface{}{
												"name":      toolName,
												"arguments": `{"query":"test"}`,
											},
										},
									},
								},
							},
						},
					})
				})
			} else {
				router.POST("/test", func(c *gin.Context) {
					var requestData map[string]interface{}
					if err := c.ShouldBindJSON(&requestData); err != nil {
						c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
						return
					}
					c.JSON(http.StatusOK, requestData)
				})
			}

			req, err := http.NewRequest(http.MethodPost, "/test", strings.NewReader(tt.requestBody))
			require.NoError(t, err)

			req.Header.Set("Content-Type", "application/json")
			for k, v := range tt.requestHeaders {
				req.Header.Set(k, v)
			}

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)

			if !tt.expectErrorAbort {
				if tt.expectedSSE {
					assert.Contains(t, w.Body.String(), tt.expectedResponseContains)
					assert.Contains(t, w.Header().Get("Content-Type"), "text/event-stream")
				} else {
					assert.Contains(t, w.Body.String(), tt.expectedResponseContains)
				}
			} else {
				assert.Contains(t, w.Body.String(), tt.expectedResponseContains)
			}
		})
	}
}

func TestExtractToolsFromCapabilities(t *testing.T) {
	log, err := logger.NewLogger("test")
	require.NoError(t, err)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockClient := middlewareMocks.NewMockMCPClientInterface(ctrl)

	mcpMiddleware := &middlewares.MCPMiddleware{
		MCPClient:     mockClient,
		Logger:        log,
		Enabled:       true,
		ToolServerMap: make(map[string]string),
	}

	tests := []struct {
		name                 string
		capabilities         []map[string]interface{}
		expectedToolCount    int
		expectedError        bool
		expectedErrorMessage string
	}{
		{
			name: "Valid tools in top level",
			capabilities: []map[string]interface{}{
				{
					"_server_url": "http://server1.com",
					"tools": []interface{}{
						map[string]interface{}{
							"name":        "tool1",
							"description": "Tool 1 description",
							"parameters": map[string]interface{}{
								"type": "object",
							},
						},
						map[string]interface{}{
							"name":        "tool2",
							"description": "Tool 2 description",
							"parameters": map[string]interface{}{
								"type": "object",
							},
						},
					},
				},
			},
			expectedToolCount: 2,
			expectedError:     false,
		},
		{
			name: "Valid tools in resources",
			capabilities: []map[string]interface{}{
				{
					"_server_url": "http://server2.com",
					"resources": map[string]interface{}{
						"tools": []interface{}{
							map[string]interface{}{
								"name":        "tool3",
								"description": "Tool 3 description",
								"parameters": map[string]interface{}{
									"type": "object",
								},
							},
						},
					},
				},
			},
			expectedToolCount: 1,
			expectedError:     false,
		},
		{
			name: "Multiple servers with tools",
			capabilities: []map[string]interface{}{
				{
					"_server_url": "http://server1.com",
					"tools": []interface{}{
						map[string]interface{}{
							"name":        "tool1",
							"description": "Tool 1 description",
							"parameters": map[string]interface{}{
								"type": "object",
							},
						},
					},
				},
				{
					"_server_url": "http://server2.com",
					"tools": []interface{}{
						map[string]interface{}{
							"name":        "tool2",
							"description": "Tool 2 description",
							"parameters": map[string]interface{}{
								"type": "object",
							},
						},
					},
				},
			},
			expectedToolCount: 2,
			expectedError:     false,
		},
		{
			name: "Missing server URL",
			capabilities: []map[string]interface{}{
				{
					"tools": []interface{}{
						map[string]interface{}{
							"name":        "tool1",
							"description": "Tool 1 description",
							"parameters": map[string]interface{}{
								"type": "object",
							},
						},
					},
				},
			},
			expectedToolCount:    0,
			expectedError:        true,
			expectedErrorMessage: "no tools found in any MCP server",
		},
		{
			name: "No tools found",
			capabilities: []map[string]interface{}{
				{
					"_server_url": "http://server1.com",
				},
			},
			expectedToolCount:    0,
			expectedError:        true,
			expectedErrorMessage: "no tools found in any MCP server",
		},
		{
			name: "Invalid tool structure",
			capabilities: []map[string]interface{}{
				{
					"_server_url": "http://server1.com",
					"tools": []interface{}{
						"not a map",
					},
				},
			},
			expectedToolCount:    0,
			expectedError:        true,
			expectedErrorMessage: "no tools found in any MCP server",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mcpMiddleware.ToolServerMap = make(map[string]string)

			tools, err := mcpMiddleware.ExtractToolsFromAllCapabilities(tt.capabilities)

			if tt.expectedError {
				assert.Error(t, err)
				if tt.expectedErrorMessage != "" {
					assert.Contains(t, err.Error(), tt.expectedErrorMessage)
				}
				assert.Nil(t, tools)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, tools)
				assert.Equal(t, tt.expectedToolCount, len(tools))

				for _, tool := range tools {
					assert.Equal(t, "function", tool["type"])
					function, ok := tool["function"].(map[string]interface{})
					assert.True(t, ok)
					assert.Contains(t, function, "name")
					assert.Contains(t, function, "description")
					assert.Contains(t, function, "parameters")
				}

				assert.Equal(t, tt.expectedToolCount, len(mcpMiddleware.ToolServerMap))
			}
		})
	}
}

func TestProcessToolCalls(t *testing.T) {
	log, err := logger.NewLogger("test")
	require.NoError(t, err)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockClient := middlewareMocks.NewMockMCPClientInterface(ctrl)

	tests := []struct {
		name               string
		response           map[string]interface{}
		toolServerMap      map[string]string
		setupMock          func(client *middlewareMocks.MockMCPClientInterface)
		setupSSE           bool
		expectedUnchanged  bool
		expectToolResponse bool
		expectError        bool
	}{
		{
			name: "No choices in response",
			response: map[string]interface{}{
				"id": "resp1",
			},
			expectedUnchanged: true,
			expectError:       false,
		},
		{
			name: "Empty choices array",
			response: map[string]interface{}{
				"id":      "resp1",
				"choices": []interface{}{},
			},
			expectedUnchanged: true,
			expectError:       false,
		},
		{
			name: "No message in choice",
			response: map[string]interface{}{
				"id":      "resp1",
				"choices": []interface{}{},
			},
			expectedUnchanged: true,
			expectError:       false,
		},
		{
			name: "No tool calls in message",
			response: map[string]interface{}{
				"id": "resp1",
				"choices": []interface{}{
					map[string]interface{}{
						"message": map[string]interface{}{
							"role":    "assistant",
							"content": "Hello",
						},
					},
				},
			},
			expectedUnchanged: true,
			expectError:       false,
		},
		{
			name: "Empty tool calls array",
			response: map[string]interface{}{
				"id": "resp1",
				"choices": []interface{}{
					map[string]interface{}{
						"message": map[string]interface{}{
							"role":       "assistant",
							"tool_calls": []interface{}{},
						},
					},
				},
			},
			expectedUnchanged: true,
			expectError:       false,
		},
		{
			name: "Successful tool call processing",
			response: map[string]interface{}{
				"id": "resp1",
				"choices": []interface{}{
					map[string]interface{}{
						"message": map[string]interface{}{
							"role": "assistant",
							"tool_calls": []interface{}{
								map[string]interface{}{
									"function": map[string]interface{}{
										"name":      "test_tool",
										"arguments": `{"query":"test"}`,
									},
								},
							},
						},
					},
				},
			},
			toolServerMap: map[string]string{
				"test_tool": "http://mcp-server",
			},
			setupMock: func(client *middlewareMocks.MockMCPClientInterface) {
				client.EXPECT().
					ExecuteTool(
						gomock.Any(),
						"test_tool",
						`{"query":"test"}`,
						"http://mcp-server",
					).
					Return(map[string]interface{}{
						"result": "Tool executed successfully",
					}, nil)
			},
			expectToolResponse: true,
			expectError:        false,
		},
		{
			name: "Tool call error",
			response: map[string]interface{}{
				"id": "resp1",
				"choices": []interface{}{
					map[string]interface{}{
						"message": map[string]interface{}{
							"role": "assistant",
							"tool_calls": []interface{}{
								map[string]interface{}{
									"function": map[string]interface{}{
										"name":      "test_tool",
										"arguments": `{"query":"test"}`,
									},
								},
							},
						},
					},
				},
			},
			toolServerMap: map[string]string{
				"test_tool": "http://mcp-server",
			},
			setupMock: func(client *middlewareMocks.MockMCPClientInterface) {
				client.EXPECT().
					ExecuteTool(
						gomock.Any(),
						"test_tool",
						`{"query":"test"}`,
						"http://mcp-server",
					).
					Return(nil, errors.New("tool execution failed"))
			},
			expectToolResponse: true,
			expectError:        false,
		},
		{
			name: "Tool not found in server map",
			response: map[string]interface{}{
				"id": "resp1",
				"choices": []interface{}{
					map[string]interface{}{
						"message": map[string]interface{}{
							"role": "assistant",
							"tool_calls": []interface{}{
								map[string]interface{}{
									"function": map[string]interface{}{
										"name":      "unknown_tool",
										"arguments": `{"query":"test"}`,
									},
								},
							},
						},
					},
				},
			},
			toolServerMap: map[string]string{
				"test_tool": "http://mcp-server",
			},
			setupMock:          func(client *middlewareMocks.MockMCPClientInterface) {},
			expectToolResponse: true,
			expectError:        false,
		},
		{
			name: "Multiple tool calls",
			response: map[string]interface{}{
				"id": "resp1",
				"choices": []interface{}{
					map[string]interface{}{
						"message": map[string]interface{}{
							"role": "assistant",
							"tool_calls": []interface{}{
								map[string]interface{}{
									"function": map[string]interface{}{
										"name":      "test_tool1",
										"arguments": `{"query":"test1"}`,
									},
								},
								map[string]interface{}{
									"function": map[string]interface{}{
										"name":      "test_tool2",
										"arguments": `{"query":"test2"}`,
									},
								},
							},
						},
					},
				},
			},
			toolServerMap: map[string]string{
				"test_tool1": "http://mcp-server-1",
				"test_tool2": "http://mcp-server-2",
			},
			setupMock: func(client *middlewareMocks.MockMCPClientInterface) {
				client.EXPECT().
					ExecuteTool(
						gomock.Any(),
						"test_tool1",
						`{"query":"test1"}`,
						"http://mcp-server-1",
					).
					Return(map[string]interface{}{
						"result": "Tool 1 executed successfully",
					}, nil)

				client.EXPECT().
					ExecuteTool(
						gomock.Any(),
						"test_tool2",
						`{"query":"test2"}`,
						"http://mcp-server-2",
					).
					Return(map[string]interface{}{
						"result": "Tool 2 executed successfully",
					}, nil)
			},
			expectToolResponse: true,
			expectError:        false,
		},
		{
			name: "SSE notifications for tool calls",
			response: map[string]interface{}{
				"id": "resp1",
				"choices": []interface{}{
					map[string]interface{}{
						"message": map[string]interface{}{
							"role": "assistant",
							"tool_calls": []interface{}{
								map[string]interface{}{
									"function": map[string]interface{}{
										"name":      "test_tool",
										"arguments": `{"query":"test"}`,
									},
								},
							},
						},
					},
				},
			},
			toolServerMap: map[string]string{
				"test_tool": "http://mcp-server",
			},
			setupMock: func(client *middlewareMocks.MockMCPClientInterface) {
				client.EXPECT().
					ExecuteTool(
						gomock.Any(),
						"test_tool",
						`{"query":"test"}`,
						"http://mcp-server",
					).
					Return(map[string]interface{}{
						"result": "Tool executed successfully",
					}, nil)
			},
			setupSSE:           true,
			expectToolResponse: true,
			expectError:        false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mcpMiddleware := &middlewares.MCPMiddleware{
				MCPClient:     mockClient,
				Logger:        log,
				Enabled:       true,
				ToolServerMap: tt.toolServerMap,
			}

			if tt.setupMock != nil {
				tt.setupMock(mockClient)
			}

			var sseContext *gin.Context
			var recorder *httptest.ResponseRecorder

			if tt.setupSSE {
				sseContext, recorder = setupGinContext(t, "POST", "/test", "", map[string]string{
					"Accept": "text/event-stream",
				})
			}

			originalResponse := deepCopy(tt.response)

			var processedResponse map[string]interface{}
			var err error

			if tt.setupSSE {
				processedResponse, err = mcpMiddleware.ProcessToolCalls(tt.response, sseContext)
			} else {
				processedResponse, err = mcpMiddleware.ProcessToolCalls(tt.response)
			}

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			if tt.expectedUnchanged {
				assert.Equal(t, originalResponse, processedResponse)
			}

			if tt.expectToolResponse {
				choices, ok := processedResponse["choices"].([]interface{})
				assert.True(t, ok)

				firstChoice, ok := choices[0].(map[string]interface{})
				assert.True(t, ok)

				message, ok := firstChoice["message"].(map[string]interface{})
				assert.True(t, ok)

				// Check that we have content in the message (where tool results are now placed)
				content, ok := message["content"].(string)
				assert.True(t, ok, "Message should contain content with tool results")
				assert.NotEmpty(t, content, "Message content should not be empty")

				toolCalls, ok := message["tool_calls"].([]interface{})
				assert.True(t, ok)

				// Only check that the tool calls structure is preserved
				for _, tc := range toolCalls {
					toolCall, ok := tc.(map[string]interface{})
					assert.True(t, ok)

					function, ok := toolCall["function"].(map[string]interface{})
					assert.True(t, ok)

					// No longer checking for response in function object
					// Instead, we verify the function has the basic properties
					assert.Contains(t, function, "name")
					assert.Contains(t, function, "arguments")
				}
			}

			if tt.setupSSE {
				responseBody := recorder.Body.String()
				assert.Contains(t, responseBody, "event:tool_call_progress")
				assert.Contains(t, responseBody, "event:tool_calls_complete")
			}
		})
	}
}

func TestEnhanceRequest(t *testing.T) {
	log, err := logger.NewLogger("test")
	require.NoError(t, err)

	tests := []struct {
		name              string
		requestBody       string
		mcpEnabled        bool
		setupMock         func(client *middlewareMocks.MockMCPClientInterface)
		expectedTools     int
		expectError       bool
		expectBodyChanged bool
	}{
		{
			name:        "MCP Disabled",
			requestBody: `{"model": "gpt-4", "messages": [{"role": "user", "content": "Hello"}]}`,
			mcpEnabled:  false,
			setupMock:   func(client *middlewareMocks.MockMCPClientInterface) {},
			expectError: false,
		},
		{
			name:        "Discover Capabilities Error",
			requestBody: `{"model": "gpt-4", "messages": [{"role": "user", "content": "Hello"}]}`,
			mcpEnabled:  true,
			setupMock: func(client *middlewareMocks.MockMCPClientInterface) {
				client.EXPECT().
					DiscoverCapabilities(gomock.Any()).
					Return(nil, errors.New("failed to discover capabilities"))
			},
			expectError: true,
		},
		{
			name:        "Valid Capabilities With Tools",
			requestBody: `{"model": "gpt-4", "messages": [{"role": "user", "content": "Hello"}]}`,
			mcpEnabled:  true,
			setupMock: func(client *middlewareMocks.MockMCPClientInterface) {
				client.EXPECT().
					DiscoverCapabilities(gomock.Any()).
					Return([]map[string]interface{}{
						{
							"_server_url": "http://mcp-server",
							"tools": []interface{}{
								map[string]interface{}{
									"name":        "test_tool",
									"description": "A test tool",
									"parameters": map[string]interface{}{
										"type": "object",
									},
								},
							},
						},
					}, nil)
			},
			expectedTools:     1,
			expectError:       false,
			expectBodyChanged: true,
		},
		{
			name:        "No Tools in Capabilities",
			requestBody: `{"model": "gpt-4", "messages": [{"role": "user", "content": "Hello"}]}`,
			mcpEnabled:  true,
			setupMock: func(client *middlewareMocks.MockMCPClientInterface) {
				client.EXPECT().
					DiscoverCapabilities(gomock.Any()).
					Return([]map[string]interface{}{
						{
							"_server_url": "http://mcp-server",
							// No tools
						},
					}, nil)
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockClient := middlewareMocks.NewMockMCPClientInterface(ctrl)

			mcpMiddleware := &middlewares.MCPMiddleware{
				MCPClient:     mockClient,
				Logger:        log,
				Enabled:       tt.mcpEnabled,
				ToolServerMap: make(map[string]string),
			}

			tt.setupMock(mockClient)

			req, err := http.NewRequest("POST", "/test", strings.NewReader(tt.requestBody))
			require.NoError(t, err)
			req.Header.Set("Content-Type", "application/json")

			originalBody := []byte(tt.requestBody)

			err = mcpMiddleware.EnhanceRequest(req)

			if tt.expectError {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)

			if !tt.mcpEnabled {
				newBody, _ := io.ReadAll(req.Body)
				assert.Equal(t, string(originalBody), string(newBody))
			} else if tt.expectBodyChanged {
				newBody, _ := io.ReadAll(req.Body)
				var enhancedRequest map[string]interface{}
				err := json.Unmarshal(newBody, &enhancedRequest)
				assert.NoError(t, err)

				tools, ok := enhancedRequest["tools"].([]interface{})
				assert.True(t, ok, "Request should contain tools array")
				assert.Equal(t, tt.expectedTools, len(tools))

				assert.Equal(t, "application/json", req.Header.Get("Content-Type"))
			}
		})
	}
}

// Helper funcion to make a deep copy of a map[string]interface{}
func deepCopy(original map[string]interface{}) map[string]interface{} {
	jsonBytes, _ := json.Marshal(original)
	var copy map[string]interface{}
	_ = json.Unmarshal(jsonBytes, &copy)
	return copy
}
