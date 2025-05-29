package tests

import (
	"context"
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
	}{
		{
			name: "no tool calls streaming",
			setupMocks: func(mockLogger *mocks.MockLogger, mockMCPClient *mocks.MockMCPClientInterface, mockProvider *mocks.MockIProvider) {
				streamCh := make(chan []byte, 3)
				streamCh <- []byte(`{"id":"test","choices":[{"index":0,"delta":{"content":"Hello"},"finish_reason":null}]}`)
				streamCh <- []byte(`{"id":"test","choices":[{"index":0,"delta":{"content":" there!"},"finish_reason":null}]}`)
				streamCh <- []byte(`{"id":"test","choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}`)
				close(streamCh)

				mockLogger.EXPECT().Debug("Agent: Starting agent streaming with model", "model", "test-model").Times(1)
				mockLogger.EXPECT().Debug("Agent: Streaming iteration", "iteration", gomock.Any()).Times(1)
				mockLogger.EXPECT().Debug("Agent: Processing chunk", "chunk", gomock.Any()).Times(3)
				mockLogger.EXPECT().Debug("Agent: Stream completing due to stop finish reason", "finishReason", "stop").Times(1)
				mockLogger.EXPECT().Debug("Agent: Stream completed for iteration", gomock.Any()).Times(1)
				mockLogger.EXPECT().Debug("Agent: Final response body", "responseBody", gomock.Any()).Times(1)
				mockLogger.EXPECT().Debug("Agent: No tool calls found, ending agent loop", gomock.Any()).Times(1)
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
			expectedResponses: []string{"Hello", " there!"},
			timeout:           2 * time.Second,
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

			var ctx context.Context
			var cancel context.CancelFunc

			if tt.name == "context cancellation" {
				ctx, cancel = context.WithCancel(context.Background())
				go func() {
					time.Sleep(100 * time.Millisecond)
					cancel()
				}()
			} else {
				ctx = context.Background()
			}

			errCh := make(chan error, 1)
			go func() {
				err := agentInstance.RunWithStream(ctx, middlewareStreamCh, c, tt.request)
				errCh <- err
			}()

			var responses []string
			timeout := time.After(tt.timeout)

			if !tt.expectError {
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
						collectingResponses = false
						assert.NoError(t, err)
					case <-timeout:
						t.Fatal("Test timed out waiting for stream completion")
					}
				}

				fullResponse := strings.Join(responses, "")
				t.Logf("Full response: %q", fullResponse)

				assert.True(t, len(responses) > 0, "Should receive at least one response chunk")
				assert.Contains(t, fullResponse, "Hello", "Response should contain at least the first part of the content")
			}

			if tt.expectError {
				collectingResponses := true
				errorReceived := false

				for collectingResponses && !errorReceived {
					select {
					case data, ok := <-middlewareStreamCh:
						if !ok {
							collectingResponses = false
							break
						}
						// Just drain the channel without collecting unused responses
						_ = string(data)
					case err := <-errCh:
						assert.Error(t, err)
						errorReceived = true
					case <-timeout:
						t.Fatal("Expected error but test timed out")
					}
				}
			}
		})
	}
}
