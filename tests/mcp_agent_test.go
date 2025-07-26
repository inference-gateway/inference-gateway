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

	"github.com/inference-gateway/inference-gateway/mcp"
	"github.com/inference-gateway/inference-gateway/providers"
	"github.com/inference-gateway/inference-gateway/tests/mocks"
	mcpmocks "github.com/inference-gateway/inference-gateway/tests/mocks/mcp"
	providersmocks "github.com/inference-gateway/inference-gateway/tests/mocks/providers"
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
			mockMCPClient := mcpmocks.NewMockMCPClientInterface(ctrl)

			agentInstance := mcp.NewAgent(mockLogger, mockMCPClient)

			if tt.expectAgent {
				assert.NotNil(t, agentInstance)
				assert.Implements(t, (*mcp.Agent)(nil), agentInstance)
			} else {
				assert.Nil(t, agentInstance)
			}
		})
	}
}

func TestAgent_Run(t *testing.T) {
	tests := []struct {
		name           string
		setupMocks     func(*mocks.MockLogger, *mcpmocks.MockMCPClientInterface, *providersmocks.MockIProvider)
		request        *providers.CreateChatCompletionRequest
		response       *providers.CreateChatCompletionResponse
		expectError    bool
		expectedResult string
	}{
		{
			name: "no tool calls",
			setupMocks: func(mockLogger *mocks.MockLogger, mockMCPClient *mcpmocks.MockMCPClientInterface, mockProvider *providersmocks.MockIProvider) {
				mockProvider.EXPECT().GetName().Return("test-provider").Times(1)
				mockLogger.EXPECT().Debug("provider set for agent", "provider", "test-provider").Times(1)
				mockLogger.EXPECT().Debug("model set for agent", "model", "test-model").Times(1)
				mockLogger.EXPECT().Debug("agent loop completed", "iterations", 0, "final_choices", 1).Times(1)
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
			setupMocks: func(mockLogger *mocks.MockLogger, mockMCPClient *mcpmocks.MockMCPClientInterface, mockProvider *providersmocks.MockIProvider) {
				mockProvider.EXPECT().GetName().Return("test-provider").Times(1)
				mockLogger.EXPECT().Debug("provider set for agent", "provider", "test-provider").Times(1)
				mockLogger.EXPECT().Debug("model set for agent", "model", "test-model").Times(1)
				mockLogger.EXPECT().Debug("agent loop iteration", "iteration", 1, "tool_calls", 1).Times(1)
				mockLogger.EXPECT().Debug("executing tool calls", "count", 1).Times(1)
				mockLogger.EXPECT().Info("executing tool call", "tool_call", "id=call_123 name=mcp_test_tool mcp_name=test_tool args=map[param:value] server=http://test-server:8080/mcp").Times(1)
				mockLogger.EXPECT().Debug("agent loop completed", "iterations", 1, "final_choices", 1).Times(1)

				mockMCPClient.EXPECT().GetServerForTool("test_tool").Return("http://test-server:8080/mcp", nil).Times(1)
				mockMCPClient.EXPECT().ExecuteTool(
					gomock.Any(),
					mcp.Request{
						Method: "tools/call",
						Params: map[string]interface{}{
							"name":      "test_tool",
							"arguments": map[string]interface{}{"param": "value"},
						},
					},
					"http://test-server:8080/mcp",
				).Return(&mcp.CallToolResult{
					Content: []mcp.ContentBlock{
						mcp.TextContent{
							Type: "text",
							Text: "Tool executed successfully",
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
										Name:      "mcp_test_tool",
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
			setupMocks: func(mockLogger *mocks.MockLogger, mockMCPClient *mcpmocks.MockMCPClientInterface, mockProvider *providersmocks.MockIProvider) {
				mockProvider.EXPECT().GetName().Return("test-provider").Times(1)
				mockLogger.EXPECT().Debug("provider set for agent", "provider", "test-provider").Times(1)
				mockLogger.EXPECT().Debug("model set for agent", "model", "test-model").Times(1)
				mockLogger.EXPECT().Debug("agent loop iteration", "iteration", gomock.Any(), "tool_calls", 1).Times(10)
				mockLogger.EXPECT().Debug("executing tool calls", "count", 1).Times(10)
				mockLogger.EXPECT().Info("executing tool call", "tool_call", gomock.Any()).Times(10)
				mockLogger.EXPECT().Warn("agent loop reached maximum iterations", gomock.Any()).Times(1)
				mockLogger.EXPECT().Debug("agent loop completed", "iterations", 10, "final_choices", 1).Times(1)

				mockMCPClient.EXPECT().GetServerForTool(gomock.Any()).Return("http://test-server:8080/mcp", nil).Times(10)
				mockMCPClient.EXPECT().ExecuteTool(gomock.Any(), gomock.Any(), gomock.Any()).Return(&mcp.CallToolResult{
					Content: []mcp.ContentBlock{
						mcp.TextContent{Type: "text", Text: "Tool result"},
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
											Name:      "mcp_test_tool",
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
										Name:      "mcp_test_tool",
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
			mockMCPClient := mcpmocks.NewMockMCPClientInterface(ctrl)
			mockProvider := providersmocks.NewMockIProvider(ctrl)

			tt.setupMocks(mockLogger, mockMCPClient, mockProvider)

			agentInstance := mcp.NewAgent(mockLogger, mockMCPClient)
			agentInstance.SetProvider(mockProvider)
			agentInstance.SetModel(&tt.request.Model)

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
		setupMocks      func(*mocks.MockLogger, *mcpmocks.MockMCPClientInterface, *providersmocks.MockIProvider)
		toolCalls       []providers.ChatCompletionMessageToolCall
		expectError     bool
		expectedResults int
		expectedContent string
	}{
		{
			name: "successful tool execution",
			setupMocks: func(mockLogger *mocks.MockLogger, mockMCPClient *mcpmocks.MockMCPClientInterface, mockProvider *providersmocks.MockIProvider) {
				mockLogger.EXPECT().Info("executing tool call", "tool_call", "id=call_123 name=mcp_test_tool mcp_name=test_tool args=map[param:value] server=http://test-server:8080/mcp").Times(1)

				mockMCPClient.EXPECT().GetServerForTool("test_tool").Return("http://test-server:8080/mcp", nil).Times(1)
				mockMCPClient.EXPECT().ExecuteTool(
					gomock.Any(),
					mcp.Request{
						Method: "tools/call",
						Params: map[string]interface{}{
							"name":      "test_tool",
							"arguments": map[string]interface{}{"param": "value"},
						},
					},
					"http://test-server:8080/mcp",
				).Return(&mcp.CallToolResult{
					Content: []mcp.ContentBlock{
						mcp.TextContent{
							Type: "text",
							Text: "Tool executed successfully",
						},
					},
				}, nil).Times(1)
			},
			toolCalls: []providers.ChatCompletionMessageToolCall{
				{
					ID:   "call_123",
					Type: providers.ChatCompletionToolTypeFunction,
					Function: providers.ChatCompletionMessageToolCallFunction{
						Name:      "mcp_test_tool",
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
			setupMocks: func(mockLogger *mocks.MockLogger, mockMCPClient *mcpmocks.MockMCPClientInterface, mockProvider *providersmocks.MockIProvider) {
				mockMCPClient.EXPECT().GetServerForTool("server_tool").Return("http://custom-server:8080", nil).Times(1)

				mockLogger.EXPECT().Info("executing tool call", "tool_call", "id=call_456 name=mcp_server_tool mcp_name=server_tool args=map[param:value] server=http://custom-server:8080").Times(1)

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
					Content: []mcp.ContentBlock{
						mcp.TextContent{
							Type: "text",
							Text: "Server tool executed",
						},
					},
				}, nil).Times(1)
			},
			toolCalls: []providers.ChatCompletionMessageToolCall{
				{
					ID:   "call_456",
					Type: providers.ChatCompletionToolTypeFunction,
					Function: providers.ChatCompletionMessageToolCallFunction{
						Name:      "mcp_server_tool",
						Arguments: `{"param": "value"}`,
					},
				},
			},
			expectError:     false,
			expectedResults: 1,
			expectedContent: "Server tool executed",
		},
		{
			name: "invalid JSON arguments",
			setupMocks: func(mockLogger *mocks.MockLogger, mockMCPClient *mcpmocks.MockMCPClientInterface, mockProvider *providersmocks.MockIProvider) {
				mockLogger.EXPECT().Error("failed to parse tool arguments", gomock.Any(), "args", "invalid json", "tool_name", "mcp_bad_tool").Times(1)
			},
			toolCalls: []providers.ChatCompletionMessageToolCall{
				{
					ID:   "call_789",
					Type: providers.ChatCompletionToolTypeFunction,
					Function: providers.ChatCompletionMessageToolCallFunction{
						Name:      "mcp_bad_tool",
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
			setupMocks: func(mockLogger *mocks.MockLogger, mockMCPClient *mcpmocks.MockMCPClientInterface, mockProvider *providersmocks.MockIProvider) {
				mockLogger.EXPECT().Info("executing tool call", "tool_call", "id=call_error name=mcp_failing_tool mcp_name=failing_tool args=map[param:value] server=http://test-server:8080/mcp").Times(1)
				mockLogger.EXPECT().Error("failed to execute tool call", gomock.Any(), "tool", "mcp_failing_tool", "server", "http://test-server:8080/mcp").Times(1)

				mockMCPClient.EXPECT().GetServerForTool("failing_tool").Return("http://test-server:8080/mcp", nil).Times(1)
				mockMCPClient.EXPECT().ExecuteTool(gomock.Any(), gomock.Any(), "http://test-server:8080/mcp").Return(nil, fmt.Errorf("tool execution failed")).Times(1)
			},
			toolCalls: []providers.ChatCompletionMessageToolCall{
				{
					ID:   "call_error",
					Type: providers.ChatCompletionToolTypeFunction,
					Function: providers.ChatCompletionMessageToolCallFunction{
						Name:      "mcp_failing_tool",
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
			setupMocks: func(mockLogger *mocks.MockLogger, mockMCPClient *mcpmocks.MockMCPClientInterface, mockProvider *providersmocks.MockIProvider) {
				mockLogger.EXPECT().Info("executing tool call", "tool_call", "id=call_multi1 name=mcp_first_tool mcp_name=first_tool args=map[param:value1] server=http://test-server:8080/mcp").Times(1)
				mockLogger.EXPECT().Info("executing tool call", "tool_call", "id=call_multi2 name=mcp_second_tool mcp_name=second_tool args=map[action:execute] server=http://test-server:8080/mcp").Times(1)

				mockMCPClient.EXPECT().GetServerForTool("first_tool").Return("http://test-server:8080/mcp", nil).Times(1)
				mockMCPClient.EXPECT().GetServerForTool("second_tool").Return("http://test-server:8080/mcp", nil).Times(1)
				mockMCPClient.EXPECT().ExecuteTool(
					gomock.Any(),
					mcp.Request{
						Method: "tools/call",
						Params: map[string]interface{}{
							"name":      "first_tool",
							"arguments": map[string]interface{}{"param": "value1"},
						},
					},
					"http://test-server:8080/mcp",
				).Return(&mcp.CallToolResult{
					Content: []mcp.ContentBlock{
						mcp.TextContent{
							Type: "text",
							Text: "First tool executed successfully",
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
					"http://test-server:8080/mcp",
				).Return(&mcp.CallToolResult{
					Content: []mcp.ContentBlock{
						mcp.TextContent{
							Type: "text",
							Text: "Second tool executed successfully",
						},
					},
				}, nil).Times(1)
			},
			toolCalls: []providers.ChatCompletionMessageToolCall{
				{
					ID:   "call_multi1",
					Type: providers.ChatCompletionToolTypeFunction,
					Function: providers.ChatCompletionMessageToolCallFunction{
						Name:      "mcp_first_tool",
						Arguments: `{"param": "value1"}`,
					},
				},
				{
					ID:   "call_multi2",
					Type: providers.ChatCompletionToolTypeFunction,
					Function: providers.ChatCompletionMessageToolCallFunction{
						Name:      "mcp_second_tool",
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
			mockMCPClient := mcpmocks.NewMockMCPClientInterface(ctrl)
			mockProvider := providersmocks.NewMockIProvider(ctrl)

			tt.setupMocks(mockLogger, mockMCPClient, mockProvider)

			agentInstance := mcp.NewAgent(mockLogger, mockMCPClient)

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
		setupMocks        func(*mocks.MockLogger, *mcpmocks.MockMCPClientInterface, *providersmocks.MockIProvider)
		request           *providers.CreateChatCompletionRequest
		expectError       bool
		expectedResponses []string
		timeout           time.Duration
		setupContext      func() (context.Context, context.CancelFunc)
		validateResponse  func(*testing.T, []string, error)
	}{
		{
			name: "no tool calls streaming",
			setupMocks: func(mockLogger *mocks.MockLogger, mockMCPClient *mcpmocks.MockMCPClientInterface, mockProvider *providersmocks.MockIProvider) {
				mockProvider.EXPECT().GetName().Return("test-provider").Times(1)
				mockLogger.EXPECT().Debug("provider set for agent", "provider", "test-provider").Times(1)
				mockLogger.EXPECT().Debug("model set for agent", "model", "test-model").Times(1)
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

				mockLogger.EXPECT().Debug("starting agent streaming", "model", "test-model", "max_iterations", 10).Times(1)
				mockLogger.EXPECT().Debug("streaming iteration", "iteration", 1, "max_iterations", 10).Times(1)
				mockLogger.EXPECT().Debug("stream completing due to stop finish reason", "finish_reason", "stop", "iteration", 1).Times(1)
				mockLogger.EXPECT().Debug("stream completed for iteration", "iteration", 1, "has_tool_calls", false).Times(1)
				mockLogger.EXPECT().Debug("no tool calls found, ending agent loop", "iteration", 1).Times(1)
				mockLogger.EXPECT().Debug("sending agent completion signal").Times(1)
				// Allow any additional debug logs
				mockLogger.EXPECT().Debug(gomock.Any(), gomock.Any()).AnyTimes()
				mockLogger.EXPECT().Debug(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
				mockLogger.EXPECT().Debug(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
				mockLogger.EXPECT().Debug(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

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

				for _, expectedContent := range []string{"Hello there!"} {
					assert.Contains(t, extractedContent, expectedContent, "Response should contain expected content")
				}
			},
		},
		{
			name: "provider error",
			setupMocks: func(mockLogger *mocks.MockLogger, mockMCPClient *mcpmocks.MockMCPClientInterface, mockProvider *providersmocks.MockIProvider) {
				mockProvider.EXPECT().GetName().Return("test-provider").Times(1)
				mockLogger.EXPECT().Debug("provider set for agent", "provider", "test-provider").Times(1)
				mockLogger.EXPECT().Debug("model set for agent", "model", "test-model").Times(1)
				mockLogger.EXPECT().Debug("starting agent streaming", "model", "test-model", "max_iterations", 10).Times(1)
				mockLogger.EXPECT().Debug("streaming iteration", "iteration", 1, "max_iterations", 10).Times(1)
				mockLogger.EXPECT().Error("failed to start streaming", gomock.Any(), "iteration", 1, "model", "test-model").Times(1)
				mockLogger.EXPECT().Debug("sending agent completion signal").Times(1)

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
			setupMocks: func(mockLogger *mocks.MockLogger, mockMCPClient *mcpmocks.MockMCPClientInterface, mockProvider *providersmocks.MockIProvider) {
				mockProvider.EXPECT().GetName().Return("test-provider").Times(1)
				mockLogger.EXPECT().Debug("provider set for agent", "provider", "test-provider").Times(1)
				mockLogger.EXPECT().Debug("model set for agent", "model", "test-model").Times(1)
				streamCh := make(chan []byte)

				mockLogger.EXPECT().Debug("starting agent streaming", "model", "test-model", "max_iterations", 10).Times(1)
				mockLogger.EXPECT().Debug("streaming iteration", "iteration", 1, "max_iterations", 10).Times(1)
				mockLogger.EXPECT().Debug("context cancelled during streaming", "iteration", 1).Times(1)
				mockLogger.EXPECT().Debug("sending agent completion signal").Times(1)

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
			setupMocks: func(mockLogger *mocks.MockLogger, mockMCPClient *mcpmocks.MockMCPClientInterface, mockProvider *providersmocks.MockIProvider) {
				mockProvider.EXPECT().GetName().Return("test-provider").Times(1)
				mockLogger.EXPECT().Debug("provider set for agent", "provider", "test-provider").Times(1)
				mockLogger.EXPECT().Debug("model set for agent", "model", "test-model").Times(1)
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

					firstStreamCh <- []byte(`data: {"id":"test","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"id":"call_123","type":"function","function":{"name":"mcp_test_tool","arguments":"{\"param\":"}}]},"finish_reason":null}]}`)
					time.Sleep(10 * time.Millisecond)
					firstStreamCh <- []byte(`data: {"id":"test","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"arguments":"\"value\"}"}}]},"finish_reason":null}]}`)
					time.Sleep(10 * time.Millisecond)
					firstStreamCh <- []byte(`data: {"id":"test","choices":[{"index":0,"delta":{"tool_calls":[{"index":1,"id":"call_456","type":"function","function":{"name":"mcp_other_tool","arguments":"{\"action\":"}}]},"finish_reason":null}]}`)
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

				mockLogger.EXPECT().Debug("starting agent streaming", "model", "test-model", "max_iterations", 10).Times(1)
				mockLogger.EXPECT().Debug("streaming iteration", "iteration", 1, "max_iterations", 10).Times(1)
				mockLogger.EXPECT().Debug("found tool calls in delta", "count", gomock.Any(), "iteration", 1).AnyTimes()
				mockLogger.EXPECT().Debug("valid tool call detected", "id", gomock.Any(), "function_name", gomock.Any()).AnyTimes()
				mockLogger.EXPECT().Debug("stream completing due to tool calls finish reason", "finish_reason", "tool_calls", "iteration", 1).Times(1)
				mockLogger.EXPECT().Debug("stream completed for iteration", "iteration", 1, "has_tool_calls", true).Times(1)
				mockLogger.EXPECT().Debug("parsed tool calls from stream", "count", 2, "iteration", 1).Times(1)
				mockLogger.EXPECT().Debug("final parsed tool call", "tool_call", gomock.Any()).AnyTimes()
				mockLogger.EXPECT().Debug("total parsed tool calls", "count", 2).Times(1)
				mockLogger.EXPECT().Debug("executing tool calls", "count", 2, "iteration", 1).Times(1)
				mockLogger.EXPECT().Info("executing tool call", "tool_call", "id=call_123 name=mcp_test_tool mcp_name=test_tool args=map[param:value] server=http://test-server:8080/mcp").Times(1)
				mockLogger.EXPECT().Info("executing tool call", "tool_call", "id=call_456 name=mcp_other_tool mcp_name=other_tool args=map[action:execute] server=http://test-server:8080/mcp").Times(1)
				mockLogger.EXPECT().Debug("tool execution complete, continuing to next iteration", "tool_results", 2, "total_messages", gomock.Any(), "iteration", 1).Times(1)

				mockLogger.EXPECT().Debug("streaming iteration", "iteration", 2, "max_iterations", 10).Times(1)
				mockLogger.EXPECT().Debug("processing chunk", "chunk", gomock.Any(), "iteration", 2).AnyTimes()
				mockLogger.EXPECT().Debug("stream completing due to stop finish reason", "finish_reason", "stop", "iteration", 2).Times(1)
				mockLogger.EXPECT().Debug("stream completed for iteration", "iteration", 2, "has_tool_calls", false).Times(1)
				mockLogger.EXPECT().Debug("no tool calls found, ending agent loop", "iteration", 2).Times(1)
				mockLogger.EXPECT().Debug("sending agent completion signal").Times(1)

				mockLogger.EXPECT().Debug(gomock.Any()).AnyTimes()
				mockLogger.EXPECT().Debug(gomock.Any(), gomock.Any()).AnyTimes()
				mockLogger.EXPECT().Debug(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
				mockLogger.EXPECT().Debug(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
				mockLogger.EXPECT().Debug(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
				mockLogger.EXPECT().Debug(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

				mockMCPClient.EXPECT().GetServerForTool("test_tool").Return("http://test-server:8080/mcp", nil).Times(1)
				mockMCPClient.EXPECT().GetServerForTool("other_tool").Return("http://test-server:8080/mcp", nil).Times(1)
				mockMCPClient.EXPECT().ExecuteTool(
					gomock.Any(),
					mcp.Request{
						Method: "tools/call",
						Params: map[string]interface{}{
							"name":      "test_tool",
							"arguments": map[string]interface{}{"param": "value"},
						},
					},
					"http://test-server:8080/mcp",
				).Return(&mcp.CallToolResult{
					Content: []mcp.ContentBlock{
						mcp.TextContent{
							Type: "text",
							Text: "First tool executed successfully",
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
					"http://test-server:8080/mcp",
				).Return(&mcp.CallToolResult{
					Content: []mcp.ContentBlock{
						mcp.TextContent{
							Type: "text",
							Text: "Second tool executed successfully",
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
			mockMCPClient := mcpmocks.NewMockMCPClientInterface(ctrl)
			mockProvider := providersmocks.NewMockIProvider(ctrl)

			tt.setupMocks(mockLogger, mockMCPClient, mockProvider)

			agentInstance := mcp.NewAgent(mockLogger, mockMCPClient)
			agentInstance.SetProvider(mockProvider)
			agentInstance.SetModel(&tt.request.Model)

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
