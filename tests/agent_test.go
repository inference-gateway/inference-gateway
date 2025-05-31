package tests

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/inference-gateway/inference-gateway/agent"
	"github.com/inference-gateway/inference-gateway/mcp"
	"github.com/inference-gateway/inference-gateway/providers"
	"github.com/inference-gateway/inference-gateway/tests/mocks"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func TestNewAgent(t *testing.T) {
	tests := []struct {
		name        string
		model       string
		expectAgent bool
	}{
		{
			name:        "creates agent successfully",
			model:       "test-model",
			expectAgent: true,
		},
		{
			name:        "creates agent with empty model",
			model:       "",
			expectAgent: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockLogger := mocks.NewMockLogger(ctrl)
			mockMCPClient := mocks.NewMockMCPClientInterface(ctrl)
			mockProvider := mocks.NewMockIProvider(ctrl)

			agentInstance := agent.NewAgent(mockLogger, mockMCPClient, mockProvider, tt.model)

			if tt.expectAgent {
				assert.NotNil(t, agentInstance)
				assert.Implements(t, (*agent.Agent)(nil), agentInstance)
			} else {
				assert.Nil(t, agentInstance)
			}
		})
	}
}

func TestAgent_Run(t *testing.T) {
	tests := []struct {
		name           string
		setupMocks     func(*mocks.MockLogger, *mocks.MockMCPClientInterface, *mocks.MockIProvider)
		request        *providers.CreateChatCompletionRequest
		response       *providers.CreateChatCompletionResponse
		expectError    bool
		expectedResult string
	}{
		{
			name: "no tool calls",
			setupMocks: func(mockLogger *mocks.MockLogger, mockMCPClient *mocks.MockMCPClientInterface, mockProvider *mocks.MockIProvider) {
				mockLogger.EXPECT().Debug("Agent: Agent loop completed", "iterations", 0, "finalChoices", 1).Times(1)
			},
			request: &providers.CreateChatCompletionRequest{
				Model: "test-model",
				Messages: []providers.Message{
					{Role: providers.MessageRoleUser, Content: "Hello"},
				},
			},
			response: &providers.CreateChatCompletionResponse{
				ID:    "test-id",
				Model: "test-model",
				Choices: []providers.ChatCompletionChoice{
					{
						Message: providers.Message{
							Role:    providers.MessageRoleAssistant,
							Content: "Hello! How can I help you?",
						},
						FinishReason: providers.FinishReasonStop,
					},
				},
			},
			expectError:    false,
			expectedResult: "Hello! How can I help you?",
		},
		{
			name: "with tool calls",
			setupMocks: func(mockLogger *mocks.MockLogger, mockMCPClient *mocks.MockMCPClientInterface, mockProvider *mocks.MockIProvider) {
				mockLogger.EXPECT().Debug("Agent: Agent loop iteration", "iteration", 1, "toolCalls", 1).Times(1)
				mockLogger.EXPECT().Debug("Agent: Executing tool calls", "count", 1).Times(1)
				mockLogger.EXPECT().Info("Agent: Executing tool call", "toolCall", "id=call_123 name=test_tool args=map[param:value] server=").Times(1)
				mockLogger.EXPECT().Debug("Agent: Agent loop completed", "iterations", 1, "finalChoices", 1).Times(1)

				mockMCPClient.EXPECT().ExecuteTool(
					gomock.Any(),
					mcp.Request{
						Method: "tools/call",
						Params: map[string]interface{}{
							"name":      "test_tool",
							"arguments": map[string]interface{}{"param": "value"},
						},
					},
					"",
				).Return(&mcp.CallToolResult{
					Content: []interface{}{
						map[string]interface{}{
							"type": "text",
							"text": "Tool executed successfully",
						},
					},
				}, nil).Times(1)

				mockProvider.EXPECT().ChatCompletions(gomock.Any(), gomock.Any()).Return(providers.CreateChatCompletionResponse{
					ID:    "test-id-2",
					Model: "test-model",
					Choices: []providers.ChatCompletionChoice{
						{
							Message: providers.Message{
								Role:    providers.MessageRoleAssistant,
								Content: "Based on the tool result, here's my answer.",
							},
							FinishReason: providers.FinishReasonStop,
						},
					},
				}, nil).Times(1)
			},
			request: &providers.CreateChatCompletionRequest{
				Model: "test-model",
				Messages: []providers.Message{
					{Role: providers.MessageRoleUser, Content: "Use the test tool"},
				},
			},
			response: &providers.CreateChatCompletionResponse{
				ID:    "test-id",
				Model: "test-model",
				Choices: []providers.ChatCompletionChoice{
					{
						Message: providers.Message{
							Role:    providers.MessageRoleAssistant,
							Content: "I'll use the tool to help you.",
							ToolCalls: &[]providers.ChatCompletionMessageToolCall{
								{
									ID:   "call_123",
									Type: providers.ChatCompletionToolTypeFunction,
									Function: providers.ChatCompletionMessageToolCallFunction{
										Name:      "test_tool",
										Arguments: `{"param": "value"}`,
									},
								},
							},
						},
						FinishReason: providers.FinishReasonToolCalls,
					},
				},
			},
			expectError:    false,
			expectedResult: "Based on the tool result, here's my answer.",
		},
		{
			name: "max iterations reached",
			setupMocks: func(mockLogger *mocks.MockLogger, mockMCPClient *mocks.MockMCPClientInterface, mockProvider *mocks.MockIProvider) {
				mockLogger.EXPECT().Debug("Agent: Agent loop iteration", "iteration", gomock.Any(), "toolCalls", 1).Times(10) // MaxAgentIterations
				mockLogger.EXPECT().Debug("Agent: Executing tool calls", "count", 1).Times(10)
				mockLogger.EXPECT().Info("Agent: Executing tool call", "toolCall", gomock.Any()).Times(10)
				mockLogger.EXPECT().Error("Agent: Agent loop reached maximum iterations", gomock.Any()).Times(1)
				mockLogger.EXPECT().Debug("Agent: Agent loop completed", "iterations", 10, "finalChoices", 1).Times(1)

				mockMCPClient.EXPECT().ExecuteTool(gomock.Any(), gomock.Any(), gomock.Any()).Return(&mcp.CallToolResult{
					Content: []interface{}{
						map[string]interface{}{"type": "text", "text": "Tool result"},
					},
				}, nil).Times(10)

				mockProvider.EXPECT().ChatCompletions(gomock.Any(), gomock.Any()).Return(providers.CreateChatCompletionResponse{
					ID:    "test-id",
					Model: "test-model",
					Choices: []providers.ChatCompletionChoice{
						{
							Message: providers.Message{
								Role:    providers.MessageRoleAssistant,
								Content: "More tool calls needed",
								ToolCalls: &[]providers.ChatCompletionMessageToolCall{
									{
										ID:   "call_123",
										Type: providers.ChatCompletionToolTypeFunction,
										Function: providers.ChatCompletionMessageToolCallFunction{
											Name:      "test_tool",
											Arguments: `{"param": "value"}`,
										},
									},
								},
							},
							FinishReason: providers.FinishReasonToolCalls,
						},
					},
				}, nil).Times(10)
			},
			request: &providers.CreateChatCompletionRequest{
				Model: "test-model",
				Messages: []providers.Message{
					{Role: providers.MessageRoleUser, Content: "Use the test tool"},
				},
			},
			response: &providers.CreateChatCompletionResponse{
				ID:    "test-id",
				Model: "test-model",
				Choices: []providers.ChatCompletionChoice{
					{
						Message: providers.Message{
							Role:    providers.MessageRoleAssistant,
							Content: "I'll use the tool to help you.",
							ToolCalls: &[]providers.ChatCompletionMessageToolCall{
								{
									ID:   "call_123",
									Type: providers.ChatCompletionToolTypeFunction,
									Function: providers.ChatCompletionMessageToolCallFunction{
										Name:      "test_tool",
										Arguments: `{"param": "value"}`,
									},
								},
							},
						},
						FinishReason: providers.FinishReasonToolCalls,
					},
				},
			},
			expectError:    false,
			expectedResult: "More tool calls needed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockLogger := mocks.NewMockLogger(ctrl)
			mockMCPClient := mocks.NewMockMCPClientInterface(ctrl)
			mockProvider := mocks.NewMockIProvider(ctrl)

			tt.setupMocks(mockLogger, mockMCPClient, mockProvider)

			agentInstance := agent.NewAgent(mockLogger, mockMCPClient, mockProvider, "test-model")

			err := agentInstance.Run(context.Background(), tt.request, tt.response)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				if tt.expectedResult != "" {
					assert.Equal(t, tt.expectedResult, tt.response.Choices[0].Message.Content)
				}
			}
		})
	}
}

func TestAgent_ExecuteTools(t *testing.T) {
	tests := []struct {
		name            string
		setupMocks      func(*mocks.MockLogger, *mocks.MockMCPClientInterface, *mocks.MockIProvider)
		toolCalls       []providers.ChatCompletionMessageToolCall
		expectError     bool
		expectedResults int
		expectedContent string
	}{
		{
			name: "successful tool execution",
			setupMocks: func(mockLogger *mocks.MockLogger, mockMCPClient *mocks.MockMCPClientInterface, mockProvider *mocks.MockIProvider) {
				mockLogger.EXPECT().Info("Agent: Executing tool call", "toolCall", "id=call_123 name=test_tool args=map[param:value] server=").Times(1)

				mockMCPClient.EXPECT().ExecuteTool(
					gomock.Any(),
					mcp.Request{
						Method: "tools/call",
						Params: map[string]interface{}{
							"name":      "test_tool",
							"arguments": map[string]interface{}{"param": "value"},
						},
					},
					"",
				).Return(&mcp.CallToolResult{
					Content: []interface{}{
						map[string]interface{}{
							"type": "text",
							"text": "Tool executed successfully",
						},
					},
				}, nil).Times(1)
			},
			toolCalls: []providers.ChatCompletionMessageToolCall{
				{
					ID:   "call_123",
					Type: providers.ChatCompletionToolTypeFunction,
					Function: providers.ChatCompletionMessageToolCallFunction{
						Name:      "test_tool",
						Arguments: `{"param": "value"}`,
					},
				},
			},
			expectError:     false,
			expectedResults: 1,
			expectedContent: "Tool executed successfully",
		},
		{
			name: "tool execution with MCP server",
			setupMocks: func(mockLogger *mocks.MockLogger, mockMCPClient *mocks.MockMCPClientInterface, mockProvider *mocks.MockIProvider) {
				mockLogger.EXPECT().Info("Agent: Executing tool call", "toolCall", "id=call_456 name=server_tool args=map[param:value] server=http://custom-server:8080").Times(1)

				mockMCPClient.EXPECT().ExecuteTool(
					gomock.Any(),
					mcp.Request{
						Method: "tools/call",
						Params: map[string]interface{}{
							"name":      "server_tool",
							"arguments": map[string]interface{}{"param": "value"},
						},
					},
					"http://custom-server:8080",
				).Return(&mcp.CallToolResult{
					Content: []interface{}{
						map[string]interface{}{
							"type": "text",
							"text": "Server tool executed",
						},
					},
				}, nil).Times(1)
			},
			toolCalls: []providers.ChatCompletionMessageToolCall{
				{
					ID:   "call_456",
					Type: providers.ChatCompletionToolTypeFunction,
					Function: providers.ChatCompletionMessageToolCallFunction{
						Name:      "server_tool",
						Arguments: `{"param": "value", "mcpServer": "http://custom-server:8080"}`,
					},
				},
			},
			expectError:     false,
			expectedResults: 1,
			expectedContent: "Server tool executed",
		},
		{
			name: "invalid JSON arguments",
			setupMocks: func(mockLogger *mocks.MockLogger, mockMCPClient *mocks.MockMCPClientInterface, mockProvider *mocks.MockIProvider) {
				mockLogger.EXPECT().Error("Agent: Failed to parse tool arguments", gomock.Any(), "args", "invalid json").Times(1)
			},
			toolCalls: []providers.ChatCompletionMessageToolCall{
				{
					ID:   "call_789",
					Type: providers.ChatCompletionToolTypeFunction,
					Function: providers.ChatCompletionMessageToolCallFunction{
						Name:      "bad_tool",
						Arguments: `invalid json`,
					},
				},
			},
			expectError:     false,
			expectedResults: 1,
			expectedContent: "Error: Failed to parse arguments",
		},
		{
			name: "MCP execution error",
			setupMocks: func(mockLogger *mocks.MockLogger, mockMCPClient *mocks.MockMCPClientInterface, mockProvider *mocks.MockIProvider) {
				mockLogger.EXPECT().Info("Agent: Executing tool call", "toolCall", "id=call_error name=failing_tool args=map[param:value] server=").Times(1)
				mockLogger.EXPECT().Error("Agent: Failed to execute tool call", gomock.Any(), "tool", "failing_tool").Times(1)

				mockMCPClient.EXPECT().ExecuteTool(gomock.Any(), gomock.Any(), "").Return(nil, fmt.Errorf("tool execution failed")).Times(1)
			},
			toolCalls: []providers.ChatCompletionMessageToolCall{
				{
					ID:   "call_error",
					Type: providers.ChatCompletionToolTypeFunction,
					Function: providers.ChatCompletionMessageToolCallFunction{
						Name:      "failing_tool",
						Arguments: `{"param": "value"}`,
					},
				},
			},
			expectError:     false,
			expectedResults: 1,
			expectedContent: "Error: tool execution failed",
		},
		{
			name: "multiple tool execution",
			setupMocks: func(mockLogger *mocks.MockLogger, mockMCPClient *mocks.MockMCPClientInterface, mockProvider *mocks.MockIProvider) {
				mockLogger.EXPECT().Info("Agent: Executing tool call", "toolCall", "id=call_multi1 name=first_tool args=map[param:value1] server=").Times(1)
				mockLogger.EXPECT().Info("Agent: Executing tool call", "toolCall", "id=call_multi2 name=second_tool args=map[action:execute] server=").Times(1)

				mockMCPClient.EXPECT().ExecuteTool(
					gomock.Any(),
					mcp.Request{
						Method: "tools/call",
						Params: map[string]interface{}{
							"name":      "first_tool",
							"arguments": map[string]interface{}{"param": "value1"},
						},
					},
					"",
				).Return(&mcp.CallToolResult{
					Content: []interface{}{
						map[string]interface{}{
							"type": "text",
							"text": "First tool executed successfully",
						},
					},
				}, nil).Times(1)

				mockMCPClient.EXPECT().ExecuteTool(
					gomock.Any(),
					mcp.Request{
						Method: "tools/call",
						Params: map[string]interface{}{
							"name":      "second_tool",
							"arguments": map[string]interface{}{"action": "execute"},
						},
					},
					"",
				).Return(&mcp.CallToolResult{
					Content: []interface{}{
						map[string]interface{}{
							"type": "text",
							"text": "Second tool executed successfully",
						},
					},
				}, nil).Times(1)
			},
			toolCalls: []providers.ChatCompletionMessageToolCall{
				{
					ID:   "call_multi1",
					Type: providers.ChatCompletionToolTypeFunction,
					Function: providers.ChatCompletionMessageToolCallFunction{
						Name:      "first_tool",
						Arguments: `{"param": "value1"}`,
					},
				},
				{
					ID:   "call_multi2",
					Type: providers.ChatCompletionToolTypeFunction,
					Function: providers.ChatCompletionMessageToolCallFunction{
						Name:      "second_tool",
						Arguments: `{"action": "execute"}`,
					},
				},
			},
			expectError:     false,
			expectedResults: 2,
			expectedContent: "First tool executed successfully",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockLogger := mocks.NewMockLogger(ctrl)
			mockMCPClient := mocks.NewMockMCPClientInterface(ctrl)
			mockProvider := mocks.NewMockIProvider(ctrl)

			tt.setupMocks(mockLogger, mockMCPClient, mockProvider)

			agentInstance := agent.NewAgent(mockLogger, mockMCPClient, mockProvider, "test-model")

			results, err := agentInstance.ExecuteTools(context.Background(), tt.toolCalls)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Len(t, results, tt.expectedResults)
				if tt.expectedResults > 0 {
					assert.Equal(t, providers.MessageRoleTool, results[0].Role)
					assert.Equal(t, tt.toolCalls[0].ID, *results[0].ToolCallId)
					assert.Contains(t, results[0].Content, tt.expectedContent)

					if tt.name == "multiple tool execution" && len(results) > 1 {
						assert.Equal(t, providers.MessageRoleTool, results[1].Role)
						assert.Equal(t, tt.toolCalls[1].ID, *results[1].ToolCallId)
						assert.Contains(t, results[1].Content, "Second tool executed successfully")
					}
				}
			}
		})
	}
}

func TestAgent_RunWithStream(t *testing.T) {
	tests := []struct {
		name              string
		setupMocks        func(*mocks.MockLogger, *mocks.MockMCPClientInterface, *mocks.MockIProvider)
		request           *providers.CreateChatCompletionRequest
		expectError       bool
		expectedResponses []string
		timeout           time.Duration
		setupContext      func() (context.Context, context.CancelFunc)
		validateResponse  func(*testing.T, []string, error)
	}{
		{
			name: "no tool calls streaming",
			setupMocks: func(mockLogger *mocks.MockLogger, mockMCPClient *mocks.MockMCPClientInterface, mockProvider *mocks.MockIProvider) {
				streamCh := make(chan []byte, 10)
				go func() {
					streamCh <- []byte(`data: {"id":"test","choices":[{"index":0,"delta":{"content":"Hello"},"finish_reason":null}]}`)
					time.Sleep(10 * time.Millisecond)
					streamCh <- []byte(`data: {"id":"test","choices":[{"index":0,"delta":{"content":" ther"},"finish_reason":null}]}`)
					time.Sleep(10 * time.Millisecond)
					streamCh <- []byte(`data: {"id":"test","choices":[{"index":0,"delta":{"content":"e!"},"finish_reason":null}]}`)
					time.Sleep(10 * time.Millisecond)
					streamCh <- []byte(`data: {"id":"test","choices":[{"index":0,"delta":{},"finish_reason":"stop"}],"usage":{"prompt_tokens":10,"completion_tokens":3,"total_tokens":13}}`)
					time.Sleep(10 * time.Millisecond)
					streamCh <- []byte(`data: [DONE]`)
					defer close(streamCh)
				}()

				mockLogger.EXPECT().Debug("Agent: Starting agent streaming with model", "model", "test-model").Times(1)
				mockLogger.EXPECT().Debug("Agent: Streaming iteration", "iteration", gomock.Any()).Times(1)
				mockLogger.EXPECT().Debug("Agent: Processing chunk", "chunk", gomock.Any()).AnyTimes()
				mockLogger.EXPECT().Debug("Agent: Stream completing due to stop finish reason", "finishReason", "stop").Times(1)
				mockLogger.EXPECT().Debug("Agent: Stream completed for iteration", "iteration", gomock.Any(), "hasToolCalls", false).Times(1)
				mockLogger.EXPECT().Debug("Agent: Final response body", "responseBodyBuilder", gomock.Any()).Times(1)
				mockLogger.EXPECT().Debug("Agent: No tool calls found, ending agent loop").Times(1)
				mockLogger.EXPECT().Debug("Agent: Sending agent completion signal").Times(1)

				mockProvider.EXPECT().StreamChatCompletions(gomock.Any(), gomock.Any()).Return(streamCh, nil).Times(1)
			},
			request: &providers.CreateChatCompletionRequest{
				Model: "test-model",
				Messages: []providers.Message{
					{Role: providers.MessageRoleUser, Content: "Hello"},
				},
			},
			expectError:       false,
			expectedResponses: []string{"Hello there!"},
			timeout:           5 * time.Second,
			setupContext: func() (context.Context, context.CancelFunc) {
				return context.Background(), nil
			},
			validateResponse: func(t *testing.T, responses []string, err error) {
				assert.NoError(t, err)
				assert.True(t, len(responses) > 0, "Should receive at least one response chunk")

				fullResponse := strings.Join(responses, "")
				t.Logf("Full response: %q", fullResponse)

				var combinedContent strings.Builder
				for _, response := range responses {
					if strings.HasPrefix(response, "data: ") {
						jsonStr := strings.TrimPrefix(response, "data: ")
						jsonStr = strings.TrimSpace(jsonStr)

						var streamResp map[string]interface{}
						if err := json.Unmarshal([]byte(jsonStr), &streamResp); err == nil {
							if choices, ok := streamResp["choices"].([]interface{}); ok && len(choices) > 0 {
								if choice, ok := choices[0].(map[string]interface{}); ok {
									if delta, ok := choice["delta"].(map[string]interface{}); ok {
										if content, ok := delta["content"].(string); ok {
											combinedContent.WriteString(content)
										}
									}
								}
							}
						}
					}
				}

				extractedContent := combinedContent.String()
				t.Logf("Extracted content: %q", extractedContent)

				for _, expectedContent := range []string{"Hello there!"} {
					assert.Contains(t, extractedContent, expectedContent, "Response should contain expected content")
				}
			},
		},
		{
			name: "provider error",
			setupMocks: func(mockLogger *mocks.MockLogger, mockMCPClient *mocks.MockMCPClientInterface, mockProvider *mocks.MockIProvider) {
				mockLogger.EXPECT().Debug("Agent: Starting agent streaming with model", "model", "test-model").Times(1)
				mockLogger.EXPECT().Debug("Agent: Streaming iteration", "iteration", gomock.Any()).Times(1)
				mockLogger.EXPECT().Error("Agent: Failed to start streaming", gomock.Any()).Times(1)
				mockLogger.EXPECT().Debug("Agent: Sending agent completion signal").Times(1)

				mockProvider.EXPECT().StreamChatCompletions(gomock.Any(), gomock.Any()).Return(nil, fmt.Errorf("provider streaming failed")).Times(1)
			},
			request: &providers.CreateChatCompletionRequest{
				Model: "test-model",
				Messages: []providers.Message{
					{Role: providers.MessageRoleUser, Content: "Hello"},
				},
			},
			expectError:       true,
			expectedResponses: nil,
			timeout:           1 * time.Second,
			setupContext: func() (context.Context, context.CancelFunc) {
				return context.Background(), nil
			},
			validateResponse: func(t *testing.T, responses []string, err error) {
				assert.Error(t, err)
			},
		},
		{
			name: "context cancellation",
			setupMocks: func(mockLogger *mocks.MockLogger, mockMCPClient *mocks.MockMCPClientInterface, mockProvider *mocks.MockIProvider) {
				streamCh := make(chan []byte)

				mockLogger.EXPECT().Debug("Agent: Starting agent streaming with model", "model", "test-model").Times(1)
				mockLogger.EXPECT().Debug("Agent: Streaming iteration", "iteration", gomock.Any()).Times(1)
				mockLogger.EXPECT().Debug("Context cancelled during streaming").Times(1)
				mockLogger.EXPECT().Debug("Agent: Sending agent completion signal").Times(1)

				mockProvider.EXPECT().StreamChatCompletions(gomock.Any(), gomock.Any()).Return(streamCh, nil).Times(1)
			},
			request: &providers.CreateChatCompletionRequest{
				Model: "test-model",
				Messages: []providers.Message{
					{Role: providers.MessageRoleUser, Content: "Hello"},
				},
			},
			expectError:       true,
			expectedResponses: nil,
			timeout:           1 * time.Second,
			setupContext: func() (context.Context, context.CancelFunc) {
				ctx, cancel := context.WithCancel(context.Background())
				go func() {
					time.Sleep(100 * time.Millisecond)
					cancel()
				}()
				return ctx, cancel
			},
			validateResponse: func(t *testing.T, responses []string, err error) {
				assert.Error(t, err)
			},
		},
		{
			name: "executing multiple mcp tools",
			setupMocks: func(mockLogger *mocks.MockLogger, mockMCPClient *mocks.MockMCPClientInterface, mockProvider *mocks.MockIProvider) {
				firstStreamCh := make(chan []byte, 15)
				go func() {
					time.Sleep(10 * time.Millisecond)

					firstStreamCh <- []byte(`data: {"id":"test","choices":[{"index":0,"delta":{"content":"I'll "},"finish_reason":null}]}`)
					time.Sleep(10 * time.Millisecond)
					firstStreamCh <- []byte(`data: {"id":"test","choices":[{"index":0,"delta":{"content":"use "},"finish_reason":null}]}`)
					time.Sleep(10 * time.Millisecond)
					firstStreamCh <- []byte(`data: {"id":"test","choices":[{"index":0,"delta":{"content":"both "},"finish_reason":null}]}`)
					time.Sleep(10 * time.Millisecond)
					firstStreamCh <- []byte(`data: {"id":"test","choices":[{"index":0,"delta":{"content":"tools"},"finish_reason":null}]}`)
					time.Sleep(10 * time.Millisecond)
					firstStreamCh <- []byte(`data: {"id":"test","choices":[{"index":0,"delta":{"content":" to "},"finish_reason":null}]}`)
					time.Sleep(10 * time.Millisecond)
					firstStreamCh <- []byte(`data: {"id":"test","choices":[{"index":0,"delta":{"content":"help "},"finish_reason":null}]}`)
					time.Sleep(10 * time.Millisecond)
					firstStreamCh <- []byte(`data: {"id":"test","choices":[{"index":0,"delta":{"content":"you."},"finish_reason":null}]}`)
					time.Sleep(10 * time.Millisecond)

					firstStreamCh <- []byte(`data: {"id":"test","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"id":"call_123","type":"function","function":{"name":"test_tool","arguments":"{\"param\":"}}]},"finish_reason":null}]}`)
					time.Sleep(10 * time.Millisecond)
					firstStreamCh <- []byte(`data: {"id":"test","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"arguments":"\"value\"}"}}]},"finish_reason":null}]}`)
					time.Sleep(10 * time.Millisecond)
					firstStreamCh <- []byte(`data: {"id":"test","choices":[{"index":0,"delta":{"tool_calls":[{"index":1,"id":"call_456","type":"function","function":{"name":"other_tool","arguments":"{\"action\":"}}]},"finish_reason":null}]}`)
					time.Sleep(10 * time.Millisecond)
					firstStreamCh <- []byte(`data: {"id":"test","choices":[{"index":0,"delta":{"tool_calls":[{"index":1,"function":{"arguments":"\"execute\"}"}}]},"finish_reason":null}]}`)
					time.Sleep(10 * time.Millisecond)
					firstStreamCh <- []byte(`data: {"id":"test","choices":[{"index":0,"delta":{},"finish_reason":"tool_calls"}],"usage":{"prompt_tokens":15,"completion_tokens":8,"total_tokens":23}}`)
					time.Sleep(10 * time.Millisecond)
					firstStreamCh <- []byte(`data: [DONE]`)
					close(firstStreamCh)
				}()

				secondStreamCh := make(chan []byte, 15)
				go func() {
					time.Sleep(50 * time.Millisecond)
					secondStreamCh <- []byte(`data: {"id":"test2","choices":[{"index":0,"delta":{"content":"Based"},"finish_reason":null}]}`)
					time.Sleep(10 * time.Millisecond)
					secondStreamCh <- []byte(`data: {"id":"test2","choices":[{"index":0,"delta":{"content":" on "},"finish_reason":null}]}`)
					time.Sleep(10 * time.Millisecond)
					secondStreamCh <- []byte(`data: {"id":"test2","choices":[{"index":0,"delta":{"content":"the "},"finish_reason":null}]}`)
					time.Sleep(10 * time.Millisecond)
					secondStreamCh <- []byte(`data: {"id":"test2","choices":[{"index":0,"delta":{"content":"tool "},"finish_reason":null}]}`)
					time.Sleep(10 * time.Millisecond)
					secondStreamCh <- []byte(`data: {"id":"test2","choices":[{"index":0,"delta":{"content":"resul"},"finish_reason":null}]}`)
					time.Sleep(10 * time.Millisecond)
					secondStreamCh <- []byte(`data: {"id":"test2","choices":[{"index":0,"delta":{"content":"ts, "},"finish_reason":null}]}`)
					time.Sleep(10 * time.Millisecond)
					secondStreamCh <- []byte(`data: {"id":"test2","choices":[{"index":0,"delta":{"content":"both "},"finish_reason":null}]}`)
					time.Sleep(10 * time.Millisecond)
					secondStreamCh <- []byte(`data: {"id":"test2","choices":[{"index":0,"delta":{"content":"tools"},"finish_reason":null}]}`)
					time.Sleep(10 * time.Millisecond)
					secondStreamCh <- []byte(`data: {"id":"test2","choices":[{"index":0,"delta":{"content":" exec"},"finish_reason":null}]}`)
					time.Sleep(10 * time.Millisecond)
					secondStreamCh <- []byte(`data: {"id":"test2","choices":[{"index":0,"delta":{"content":"uted "},"finish_reason":null}]}`)
					time.Sleep(10 * time.Millisecond)
					secondStreamCh <- []byte(`data: {"id":"test2","choices":[{"index":0,"delta":{"content":"succe"},"finish_reason":null}]}`)
					time.Sleep(10 * time.Millisecond)
					secondStreamCh <- []byte(`data: {"id":"test2","choices":[{"index":0,"delta":{"content":"ssful"},"finish_reason":null}]}`)
					time.Sleep(10 * time.Millisecond)
					secondStreamCh <- []byte(`data: {"id":"test2","choices":[{"index":0,"delta":{"content":"ly!"},"finish_reason":null}]}`)
					time.Sleep(10 * time.Millisecond)

					secondStreamCh <- []byte(`data: {"id":"test2","choices":[{"index":0,"delta":{},"finish_reason":"stop"}],"usage":{"prompt_tokens":25,"completion_tokens":12,"total_tokens":37}}`)
					time.Sleep(10 * time.Millisecond)
					secondStreamCh <- []byte(`data: [DONE]`)
					close(secondStreamCh)
				}()

				mockLogger.EXPECT().Debug("Agent: Starting agent streaming with model", "model", "test-model").Times(1)
				mockLogger.EXPECT().Debug("Agent: Streaming iteration", "iteration", 1).Times(1)
				mockLogger.EXPECT().Debug("Agent: Processing chunk", "chunk", gomock.Any()).AnyTimes()
				mockLogger.EXPECT().Debug("Agent: Found tool calls in delta", "count", gomock.Any()).AnyTimes()
				mockLogger.EXPECT().Debug("Agent: Valid tool call detected", "id", gomock.Any(), "functionName", gomock.Any()).AnyTimes()
				mockLogger.EXPECT().Debug("Agent: Stream completing due to tool calls finish reason", "finishReason", "tool_calls").Times(1)
				mockLogger.EXPECT().Debug("Agent: Stream completed for iteration", "iteration", 1, "hasToolCalls", true).Times(1)
				mockLogger.EXPECT().Debug("Agent: Final response body", "responseBody", gomock.Any()).AnyTimes()
				mockLogger.EXPECT().Debug("Agent: Parsed tool calls from stream", "count", 2).Times(1)
				mockLogger.EXPECT().Debug("Agent: Final parsed tool call", "toolCall", gomock.Any()).AnyTimes()
				mockLogger.EXPECT().Debug("Agent: Total parsed tool calls", "count", 2).Times(1)
				mockLogger.EXPECT().Debug("Agent: Executing tool calls", "count", 2).Times(1)
				mockLogger.EXPECT().Info("Agent: Executing tool call", "toolCall", "id=call_123 name=test_tool args=map[param:value] server=").Times(1)
				mockLogger.EXPECT().Info("Agent: Executing tool call", "toolCall", "id=call_456 name=other_tool args=map[action:execute] server=").Times(1)
				mockLogger.EXPECT().Debug("Agent: Tool execution complete, continuing to next iteration", "toolResults", 2, "totalMessages", gomock.Any()).Times(1)

				mockLogger.EXPECT().Debug("Agent: Streaming iteration", "iteration", 2).Times(1)
				mockLogger.EXPECT().Debug("Agent: Stream completing due to stop finish reason", "finishReason", "stop").Times(1)
				mockLogger.EXPECT().Debug("Agent: Stream completed for iteration", "iteration", 2, "hasToolCalls", false).Times(1)
				mockLogger.EXPECT().Debug("Agent: No tool calls found, ending agent loop").Times(1)
				mockLogger.EXPECT().Debug("Agent: Sending agent completion signal").Times(1)

				mockLogger.EXPECT().Debug(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
				mockLogger.EXPECT().Debug(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
				mockLogger.EXPECT().Debug(gomock.Any()).AnyTimes()

				mockMCPClient.EXPECT().ExecuteTool(
					gomock.Any(),
					mcp.Request{
						Method: "tools/call",
						Params: map[string]interface{}{
							"name":      "test_tool",
							"arguments": map[string]interface{}{"param": "value"},
						},
					},
					"",
				).Return(&mcp.CallToolResult{
					Content: []interface{}{
						map[string]interface{}{
							"type": "text",
							"text": "First tool executed successfully",
						},
					},
				}, nil).Times(1)

				mockMCPClient.EXPECT().ExecuteTool(
					gomock.Any(),
					mcp.Request{
						Method: "tools/call",
						Params: map[string]interface{}{
							"name":      "other_tool",
							"arguments": map[string]interface{}{"action": "execute"},
						},
					},
					"",
				).Return(&mcp.CallToolResult{
					Content: []interface{}{
						map[string]interface{}{
							"type": "text",
							"text": "Second tool executed successfully",
						},
					},
				}, nil).Times(1)

				mockProvider.EXPECT().StreamChatCompletions(gomock.Any(), gomock.Any()).Return(firstStreamCh, nil).Times(1)
				mockProvider.EXPECT().StreamChatCompletions(gomock.Any(), gomock.Any()).Return(secondStreamCh, nil).Times(1)
			},
			request: &providers.CreateChatCompletionRequest{
				Model: "test-model",
				Messages: []providers.Message{
					{Role: providers.MessageRoleUser, Content: "Use the test tool and the other tool"},
				},
			},
			expectError:       false,
			expectedResponses: []string{"I'll use both tools to help you.", "Based on the tool results, both tools executed successfully!"},
			timeout:           3 * time.Second,
			setupContext: func() (context.Context, context.CancelFunc) {
				return context.Background(), nil
			},
			validateResponse: func(t *testing.T, responses []string, err error) {
				assert.NoError(t, err)
				assert.True(t, len(responses) > 0, "Should receive at least one response chunk")

				fullResponse := strings.Join(responses, "")
				t.Logf("Full response: %q", fullResponse)
				t.Logf("Number of response chunks: %d", len(responses))

				var combinedContent strings.Builder
				for _, response := range responses {
					if strings.HasPrefix(response, "data: ") {
						jsonStr := strings.TrimPrefix(response, "data: ")
						jsonStr = strings.TrimSpace(jsonStr)

						var streamResp map[string]interface{}
						if err := json.Unmarshal([]byte(jsonStr), &streamResp); err == nil {
							if choices, ok := streamResp["choices"].([]interface{}); ok && len(choices) > 0 {
								if choice, ok := choices[0].(map[string]interface{}); ok {
									if delta, ok := choice["delta"].(map[string]interface{}); ok {
										if content, ok := delta["content"].(string); ok {
											combinedContent.WriteString(content)
										}
									}
								}
							}
						}
					}
				}

				extractedContent := combinedContent.String()
				t.Logf("Extracted content: %q", extractedContent)

				expectedContents := []string{
					"I'll use both tools to help you.",
					"Based on the tool results, both tools executed successfully!",
				}
				for _, expectedContent := range expectedContents {
					assert.Contains(t, extractedContent, expectedContent, "Response should contain expected content: %s", expectedContent)
				}

				assert.Contains(t, fullResponse, "test_tool", "Response should contain first tool call information")
				assert.Contains(t, fullResponse, "other_tool", "Response should contain second tool call information")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockLogger := mocks.NewMockLogger(ctrl)
			mockMCPClient := mocks.NewMockMCPClientInterface(ctrl)
			mockProvider := mocks.NewMockIProvider(ctrl)

			tt.setupMocks(mockLogger, mockMCPClient, mockProvider)

			agentInstance := agent.NewAgent(mockLogger, mockMCPClient, mockProvider, "test-model")

			middlewareStreamCh := make(chan []byte, 10)
			c, _ := gin.CreateTestContext(httptest.NewRecorder())

			ctx, cancel := tt.setupContext()
			if cancel != nil {
				defer cancel()
			}

			errCh := make(chan error, 1)
			go func() {
				err := agentInstance.RunWithStream(ctx, middlewareStreamCh, c, tt.request)
				errCh <- err
			}()

			var responses []string
			timeout := time.After(tt.timeout)
			var finalErr error

			collectingResponses := true
			for collectingResponses {
				select {
				case data, ok := <-middlewareStreamCh:
					if !ok {
						collectingResponses = false
						break
					}
					dataStr := string(data)
					if dataStr != "data: [DONE]\n\n" {
						responses = append(responses, dataStr)
					}
				case err := <-errCh:
					finalErr = err
					collectingResponses = false
				case <-timeout:
					if tt.expectError {
						finalErr = fmt.Errorf("test timed out")
					} else {
						t.Fatal("Test timed out waiting for stream completion")
					}
					collectingResponses = false
				}
			}

			tt.validateResponse(t, responses, finalErr)
		})
	}
}
