// filepath: /workspaces/inference-gateway/tests/mcp_integration_test.go
package tests

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/inference-gateway/inference-gateway/api/middlewares"
	mockBase "github.com/inference-gateway/inference-gateway/tests/mocks"
	mockMiddlewares "github.com/inference-gateway/inference-gateway/tests/mocks/middlewares"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

// Test cases for the MCP middleware
func TestMCPMiddlewareToolCalls(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name                   string
		requestBody            map[string]interface{}
		mcpEnabled             bool
		capabilities           []map[string]interface{}
		capabilitiesError      error
		initialLLMResponse     map[string]interface{}
		toolExecutionResponses map[string]map[string]interface{}
		toolExecutionErrors    map[string]error
		expectedFinalResponse  map[string]interface{}
		expectedStatusCode     int
		expectToolExecution    bool
	}{
		{
			name: "Successful tool call execution",
			requestBody: map[string]interface{}{
				"model": "openai/gpt-4",
				"messages": []map[string]interface{}{
					{"role": "user", "content": "What's the weather in San Francisco?"},
				},
			},
			mcpEnabled: true,
			capabilities: []map[string]interface{}{
				{
					"_server_url": "http://weather-server:3000",
					"tools": []interface{}{
						map[string]interface{}{
							"name":        "getWeather",
							"description": "Get weather for a location",
							"parameters": map[string]interface{}{
								"type": "object",
								"properties": map[string]interface{}{
									"location": map[string]interface{}{
										"type":        "string",
										"description": "The city and state, e.g. San Francisco, CA",
									},
								},
								"required": []interface{}{"location"},
							},
						},
					},
				},
			},
			capabilitiesError: nil,
			initialLLMResponse: map[string]interface{}{
				"choices": []interface{}{
					map[string]interface{}{
						"message": map[string]interface{}{
							"role": "assistant",
							"tool_calls": []interface{}{
								map[string]interface{}{
									"id": "call_123",
									"function": map[string]interface{}{
										"name":      "getWeather",
										"arguments": `{"location": "San Francisco, CA"}`,
									},
								},
							},
						},
					},
				},
			},
			toolExecutionResponses: map[string]map[string]interface{}{
				"getWeather": {
					"temperature": 72,
					"conditions":  "Sunny",
					"humidity":    45,
				},
			},
			toolExecutionErrors: map[string]error{
				"getWeather": nil,
			},
			expectedFinalResponse: map[string]interface{}{
				"choices": []interface{}{
					map[string]interface{}{
						"message": map[string]interface{}{
							"role": "assistant",
							"tool_calls": []interface{}{
								map[string]interface{}{
									"id": "call_123",
									"function": map[string]interface{}{
										"name":      "getWeather",
										"arguments": `{"location": "San Francisco, CA"}`,
										"response":  `{"temperature":72,"conditions":"Sunny","humidity":45}`,
									},
								},
							},
						},
					},
				},
			},
			expectedStatusCode:  200,
			expectToolExecution: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockClient := mockMiddlewares.NewMockMCPClientInterface(ctrl)
			mockLogger := mockBase.NewMockLogger(ctrl)

			mockLogger.EXPECT().Info(gomock.Any(), gomock.Any()).AnyTimes()
			mockLogger.EXPECT().Error(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
			mockLogger.EXPECT().Debug(gomock.Any(), gomock.Any()).AnyTimes()

			m := &middlewares.MCPMiddleware{
				MCPClient:     mockClient,
				Logger:        mockLogger,
				Enabled:       tt.mcpEnabled,
				ToolServerMap: make(map[string]string),
			}

			if tt.mcpEnabled && tt.capabilities != nil && tt.capabilitiesError == nil {
				for _, capability := range tt.capabilities {
					serverURL := capability["_server_url"].(string)
					tools := capability["tools"].([]interface{})
					for _, tool := range tools {
						toolMap := tool.(map[string]interface{})
						toolName := toolMap["name"].(string)
						m.ToolServerMap[toolName] = serverURL
					}
				}
			}

			if tt.mcpEnabled && tt.initialLLMResponse != nil && tt.expectToolExecution {
				if toolResponse, ok := tt.toolExecutionResponses["getWeather"]; ok {
					mockClient.EXPECT().
						ExecuteTool(gomock.Any(), "getWeather", gomock.Any(), gomock.Any()).
						Return(toolResponse, tt.toolExecutionErrors["getWeather"]).
						AnyTimes()
				}

				processedResponse, err := m.ProcessToolCalls(tt.initialLLMResponse)
				assert.NoError(t, err)

				processedChoices := processedResponse["choices"].([]interface{})
				expectedChoices := tt.expectedFinalResponse["choices"].([]interface{})

				assert.Equal(t, len(expectedChoices), len(processedChoices))

				processedMessage := processedChoices[0].(map[string]interface{})["message"].(map[string]interface{})
				expectedMessage := expectedChoices[0].(map[string]interface{})["message"].(map[string]interface{})

				assert.Equal(t, expectedMessage["role"], processedMessage["role"])

				processedToolCalls := processedMessage["tool_calls"].([]interface{})
				expectedToolCalls := expectedMessage["tool_calls"].([]interface{})

				assert.Equal(t, len(expectedToolCalls), len(processedToolCalls))

				for i := range expectedToolCalls {
					expTC := expectedToolCalls[i].(map[string]interface{})
					actTC := processedToolCalls[i].(map[string]interface{})

					assert.Equal(t, expTC["id"], actTC["id"])

					expFunc := expTC["function"].(map[string]interface{})
					actFunc := actTC["function"].(map[string]interface{})

					assert.Equal(t, expFunc["name"], actFunc["name"])
					assert.Equal(t, expFunc["arguments"], actFunc["arguments"])

					if expResponse, ok := expFunc["response"]; ok {
						assert.Contains(t, actFunc, "response")

						var expectedResp, actualResp map[string]interface{}
						err := json.Unmarshal([]byte(expResponse.(string)), &expectedResp)
						assert.NoError(t, err, "Expected response should be valid JSON")

						err = json.Unmarshal([]byte(actFunc["response"].(string)), &actualResp)
						assert.NoError(t, err, "Actual response should be valid JSON")

						for k, v := range expectedResp {
							assert.Contains(t, actualResp, k)
							assert.Equal(t, v, actualResp[k], "Response field values should match for %s", k)
						}
					}
				}
			}
		})
	}
}

// TestMCPMiddlewareEnhanceRequest tests the EnhanceRequest function separately
func TestMCPMiddlewareEnhanceRequest(t *testing.T) {
	tests := []struct {
		name              string
		requestBody       map[string]interface{}
		capabilities      []map[string]interface{}
		capabilitiesError error
		expectedTools     []map[string]interface{}
		expectedError     bool
	}{
		{
			name: "Successfully add tools to request",
			requestBody: map[string]interface{}{
				"model": "openai/gpt-4",
				"messages": []map[string]interface{}{
					{"role": "user", "content": "What's the weather?"},
				},
			},
			capabilities: []map[string]interface{}{
				{
					"_server_url": "http://weather-server:3000",
					"tools": []interface{}{
						map[string]interface{}{
							"name":        "getWeather",
							"description": "Get weather for a location",
							"parameters": map[string]interface{}{
								"type": "object",
								"properties": map[string]interface{}{
									"location": map[string]interface{}{"type": "string"},
								},
							},
						},
					},
				},
			},
			capabilitiesError: nil,
			expectedTools: []map[string]interface{}{
				{
					"type": "function",
					"function": map[string]interface{}{
						"name":        "getWeather",
						"description": "Get weather for a location",
						"parameters": map[string]interface{}{
							"type": "object",
							"properties": map[string]interface{}{
								"location": map[string]interface{}{"type": "string"},
							},
						},
					},
				},
			},
			expectedError: false,
		},
		{
			name: "Error discovering capabilities",
			requestBody: map[string]interface{}{
				"model": "openai/gpt-4",
				"messages": []map[string]interface{}{
					{"role": "user", "content": "What's the weather?"},
				},
			},
			capabilities:      nil,
			capabilitiesError: fmt.Errorf("server unavailable"),
			expectedTools:     nil,
			expectedError:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockClient := mockMiddlewares.NewMockMCPClientInterface(ctrl)
			mockLogger := mockBase.NewMockLogger(ctrl)

			mockClient.EXPECT().
				DiscoverCapabilities(gomock.Any()).
				Return(tt.capabilities, tt.capabilitiesError)

			mockLogger.EXPECT().Info(gomock.Any(), gomock.Any()).AnyTimes()
			mockLogger.EXPECT().Error(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
			mockLogger.EXPECT().Debug(gomock.Any(), gomock.Any()).AnyTimes()

			m := &middlewares.MCPMiddleware{
				MCPClient:     mockClient,
				Logger:        mockLogger,
				Enabled:       true,
				ToolServerMap: make(map[string]string),
			}

			requestBodyBytes, _ := json.Marshal(tt.requestBody)
			req, _ := http.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewBuffer(requestBodyBytes))
			req.Header.Set("Content-Type", "application/json")

			err := m.EnhanceRequest(req)

			if tt.expectedError {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)

			bodyBytes, _ := io.ReadAll(req.Body)
			var modifiedBody map[string]interface{}
			if err := json.Unmarshal(bodyBytes, &modifiedBody); err != nil {
				t.Fatalf("failed to unmarshal body: %v", err)
			}

			tools, ok := modifiedBody["tools"].([]interface{})
			assert.True(t, ok)
			assert.Equal(t, len(tt.expectedTools), len(tools))

			if tt.capabilities != nil {
				assert.Equal(t, 1, len(m.ToolServerMap))
				assert.Equal(t, "http://weather-server:3000", m.ToolServerMap["getWeather"])
			}
		})
	}
}

// TestMCPMiddlewareProcessToolCalls tests the processToolCalls function separately
func TestMCPMiddlewareProcessToolCalls(t *testing.T) {
	tests := []struct {
		name                string
		response            map[string]interface{}
		toolServerMap       map[string]string
		toolExecutionResult map[string]interface{}
		toolExecutionError  error
		expectedResponse    map[string]interface{}
	}{
		{
			name: "Successfully process tool call",
			response: map[string]interface{}{
				"choices": []interface{}{
					map[string]interface{}{
						"message": map[string]interface{}{
							"role": "assistant",
							"tool_calls": []interface{}{
								map[string]interface{}{
									"id": "call_123",
									"function": map[string]interface{}{
										"name":      "getWeather",
										"arguments": `{"location": "San Francisco, CA"}`,
									},
								},
							},
						},
					},
				},
			},
			toolServerMap: map[string]string{
				"getWeather": "http://weather-server:3000",
			},
			toolExecutionResult: map[string]interface{}{
				"temperature": 72,
				"conditions":  "Sunny",
			},
			toolExecutionError: nil,
			expectedResponse: map[string]interface{}{
				"choices": []interface{}{
					map[string]interface{}{
						"message": map[string]interface{}{
							"role": "assistant",
							"tool_calls": []interface{}{
								map[string]interface{}{
									"id": "call_123",
									"function": map[string]interface{}{
										"name":      "getWeather",
										"arguments": `{"location": "San Francisco, CA"}`,
										"response":  `{"temperature":72,"conditions":"Sunny"}`,
									},
								},
							},
						},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockClient := mockMiddlewares.NewMockMCPClientInterface(ctrl)
			mockLogger := mockBase.NewMockLogger(ctrl)

			if len(tt.toolServerMap) > 0 && tt.toolExecutionResult != nil {
				mockClient.EXPECT().
					ExecuteTool(gomock.Any(), "getWeather", gomock.Any(), gomock.Any()).
					Return(tt.toolExecutionResult, tt.toolExecutionError)
			}

			mockLogger.EXPECT().Info(gomock.Any(), gomock.Any()).AnyTimes()
			mockLogger.EXPECT().Error(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
			mockLogger.EXPECT().Debug(gomock.Any(), gomock.Any()).AnyTimes()

			m := &middlewares.MCPMiddleware{
				MCPClient:     mockClient,
				Logger:        mockLogger,
				Enabled:       true,
				ToolServerMap: tt.toolServerMap,
			}

			result, err := m.ProcessToolCalls(tt.response)
			assert.NoError(t, err)

			actChoices, ok := result["choices"].([]interface{})
			assert.True(t, ok, "Result should have choices array")
			expChoices, ok := tt.expectedResponse["choices"].([]interface{})
			assert.True(t, ok, "Expected response should have choices array")
			assert.Equal(t, len(expChoices), len(actChoices), "Should have same number of choices")

			actMessage := actChoices[0].(map[string]interface{})["message"].(map[string]interface{})
			expMessage := expChoices[0].(map[string]interface{})["message"].(map[string]interface{})

			assert.Equal(t, expMessage["role"], actMessage["role"], "Message roles should match")

			actToolCalls := actMessage["tool_calls"].([]interface{})
			expToolCalls := expMessage["tool_calls"].([]interface{})
			assert.Equal(t, len(expToolCalls), len(actToolCalls), "Should have same number of tool calls")

			for i := range expToolCalls {
				expTC := expToolCalls[i].(map[string]interface{})
				actTC := actToolCalls[i].(map[string]interface{})

				assert.Equal(t, expTC["id"], actTC["id"], "Tool call IDs should match")

				expFunc := expTC["function"].(map[string]interface{})
				actFunc := actTC["function"].(map[string]interface{})

				assert.Equal(t, expFunc["name"], actFunc["name"], "Function names should match")
				assert.Equal(t, expFunc["arguments"], actFunc["arguments"], "Arguments should match")

				if expResp, ok := expFunc["response"]; ok {
					assert.Contains(t, actFunc, "response", "Response should be present")

					var expectedResp, actualResp map[string]interface{}
					err := json.Unmarshal([]byte(expResp.(string)), &expectedResp)
					assert.NoError(t, err, "Expected response should be valid JSON")

					err = json.Unmarshal([]byte(actFunc["response"].(string)), &actualResp)
					assert.NoError(t, err, "Actual response should be valid JSON")

					for k, v := range expectedResp {
						assert.Contains(t, actualResp, k)
						assert.Equal(t, v, actualResp[k], "Response field %s should match", k)
					}

					assert.Equal(t, len(expectedResp), len(actualResp), "Response should have same number of fields")
				}
			}
		})
	}
}
