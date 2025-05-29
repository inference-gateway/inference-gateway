package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/inference-gateway/inference-gateway/logger"
	"github.com/inference-gateway/inference-gateway/mcp"
	"github.com/inference-gateway/inference-gateway/providers"
)

// MaxAgentIterations limits the number of agent loop iterations
const MaxAgentIterations = 10

// Agent defines the interface for running agent operations
//
//go:generate mockgen -source=agent.go -destination=../tests/mocks/agent.go -package=mocks
type Agent interface {
	Run(ctx context.Context, request *providers.CreateChatCompletionRequest, response *providers.CreateChatCompletionResponse) error
	RunWithStream(ctx context.Context, middlewareStreamCh chan []byte, c *gin.Context, body *providers.CreateChatCompletionRequest) error
	ExecuteTools(ctx context.Context, toolCalls []providers.ChatCompletionMessageToolCall) ([]providers.Message, error)
}

// Ensure agentImpl implements Agent interface at compile time
var _ Agent = (*agentImpl)(nil)

// agentImpl is the concrete implementation of the Agent interface
type agentImpl struct {
	logger        logger.Logger
	mcpClient     mcp.MCPClientInterface
	provider      providers.IProvider
	providerModel string
}

// NewAgent creates a new Agent instance
func NewAgent(logger logger.Logger, mcpClient mcp.MCPClientInterface, provider providers.IProvider, providerModel string) Agent {
	return &agentImpl{
		mcpClient:     mcpClient,
		logger:        logger,
		provider:      provider,
		providerModel: providerModel,
	}
}

func (a *agentImpl) Run(ctx context.Context, request *providers.CreateChatCompletionRequest, response *providers.CreateChatCompletionResponse) error {
	currentRequest := *request
	currentResponse := *response
	iteration := 0

	for iteration < MaxAgentIterations {
		if len(currentResponse.Choices) == 0 || currentResponse.Choices[0].Message.ToolCalls == nil || len(*currentResponse.Choices[0].Message.ToolCalls) == 0 {
			break
		}

		a.logger.Debug("Agent: Agent loop iteration", "iteration", iteration+1, "toolCalls", len(*currentResponse.Choices[0].Message.ToolCalls))

		a.logger.Debug("Agent: Executing tool calls", "count", len(*currentResponse.Choices[0].Message.ToolCalls))
		toolResults, err := a.ExecuteTools(ctx, *currentResponse.Choices[0].Message.ToolCalls)
		if err != nil {
			a.logger.Error("Agent: Failed to execute tool calls", err)
			return err
		}

		currentRequest.Messages = append(currentRequest.Messages, currentResponse.Choices[0].Message)
		currentRequest.Messages = append(currentRequest.Messages, toolResults...)

		currentRequest.Model = a.providerModel
		nextResponse, err := a.provider.ChatCompletions(ctx, currentRequest)
		if err != nil {
			a.logger.Error("Agent: Failed to get response in agent loop", err)
			return err
		}

		currentResponse = nextResponse
		iteration++
	}

	if iteration >= MaxAgentIterations {
		a.logger.Error("Agent: Agent loop reached maximum iterations", fmt.Errorf("max iterations reached: %d", MaxAgentIterations))
	}

	a.logger.Debug("Agent: Agent loop completed", "iterations", iteration, "finalChoices", len(currentResponse.Choices))

	*response = currentResponse

	return nil
}

type ErrorResponse struct {
	Error string `json:"error"`
}

// RunWithStream executes the agent with the provided streaming response channel
func (a *agentImpl) RunWithStream(ctx context.Context, middlewareStreamCh chan []byte, c *gin.Context, body *providers.CreateChatCompletionRequest) error {
	currentRequest := *body
	maxIterations := 10

	currentRequest.Model = a.providerModel
	a.logger.Debug("Agent: Starting agent streaming with model", "model", currentRequest.Model)

	for iteration := 0; iteration < maxIterations; iteration++ {
		a.logger.Debug("Agent: Streaming iteration", "iteration", iteration+1)

		streamCh, err := a.provider.StreamChatCompletions(ctx, currentRequest)
		if err != nil {
			a.logger.Error("Agent: Failed to start streaming", err)
			errorData := []byte(fmt.Sprintf("data: {\"error\": \"Failed to start streaming: %s\"}\n\n", err.Error()))
			middlewareStreamCh <- errorData
			return err
		}

		var responseBody strings.Builder
		assistantMessage := providers.Message{
			Role:      providers.MessageRoleAssistant,
			Content:   "",
			ToolCalls: nil,
		}

		streamComplete := false
		hasToolCalls := false

		for !streamComplete {
			select {
			case line, ok := <-streamCh:
				if !ok {
					streamComplete = true
					break
				}

				chunkData := []byte("data: " + string(line) + "\n\n")
				middlewareStreamCh <- chunkData

				var resp providers.CreateChatCompletionStreamResponse
				if err := json.Unmarshal(line, &resp); err != nil {
					a.logger.Debug("Agent: Failed to unmarshal streaming chunk", err)
					continue
				}

				responseBody.WriteString("data: " + string(line) + "\n")

				if len(resp.Choices) == 0 {
					continue
				}

				choice := resp.Choices[0]

				if choice.Delta.Content != "" {
					assistantMessage.Content += choice.Delta.Content
				}

				if choice.Delta.ToolCalls != nil && len(*choice.Delta.ToolCalls) > 0 {
					for _, toolCall := range *choice.Delta.ToolCalls {
						if toolCall.ID != nil || (toolCall.Function != nil && (toolCall.Function.Name != "" || toolCall.Function.Arguments != "")) {
							hasToolCalls = true
							break
						}
					}
				}

				// Check if stream is complete
				if choice.FinishReason == providers.FinishReasonStop ||
					choice.FinishReason == providers.FinishReasonToolCalls {
					streamComplete = true
				}

			case <-ctx.Done():
				a.logger.Debug("Context cancelled during streaming")
				middlewareStreamCh <- []byte("data: [DONE]\n\n")
				return ctx.Err()
			}
		}

		// Parse tool calls if present
		var toolCalls []providers.ChatCompletionMessageToolCall
		if hasToolCalls {
			toolCalls, err = a.parseStreamingToolCalls(responseBody.String())
			if err != nil {
				a.logger.Error("Agent: Failed to parse streaming tool calls", err)
			} else {
				a.logger.Debug("Agent: Parsed tool calls from stream", "count", len(toolCalls))
			}
		}

		// Build complete assistant message
		if len(toolCalls) > 0 {
			assistantMessage.ToolCalls = &toolCalls
		}

		// If no tool calls, end the agent loop
		if len(toolCalls) == 0 {
			a.logger.Debug("Agent: No tool calls found, ending agent loop")
			middlewareStreamCh <- []byte("data: [DONE]\n\n")
			return nil
		}

		// Execute tool calls
		a.logger.Debug("Agent: Executing tool calls", "count", len(toolCalls))
		toolResults, err := a.ExecuteTools(ctx, toolCalls)
		if err != nil {
			a.logger.Error("Agent: Failed to execute tool calls", err)
			errorData := []byte(fmt.Sprintf("data: {\"error\": \"Failed to execute tools: %s\"}\n\n", err.Error()))
			middlewareStreamCh <- errorData
			return err
		}

		// Update messages for next iteration
		currentRequest.Messages = append(currentRequest.Messages, assistantMessage)
		currentRequest.Messages = append(currentRequest.Messages, toolResults...)
		currentRequest.Model = a.providerModel

		a.logger.Debug("Agent: Tool execution complete, continuing to next iteration",
			"toolResults", len(toolResults), "totalMessages", len(currentRequest.Messages))
	}

	// Max iterations reached
	a.logger.Error("Agent: Agent streaming reached maximum iterations", fmt.Errorf("max iterations reached: %d", maxIterations))
	middlewareStreamCh <- []byte("data: [DONE]\n\n")
	return nil
}

// ExecuteTool executes a tool with the provided context, tool name, and arguments
func (a *agentImpl) ExecuteTools(ctx context.Context, toolCalls []providers.ChatCompletionMessageToolCall) ([]providers.Message, error) {
	var results []providers.Message

	for _, toolCall := range toolCalls {
		var args map[string]interface{}
		if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &args); err != nil {
			a.logger.Error("Agent: Failed to parse tool arguments", err, "args", toolCall.Function.Arguments)
			results = append(results, providers.Message{
				Role:       providers.MessageRoleTool,
				Content:    fmt.Sprintf("Error: Failed to parse arguments: %v", err),
				ToolCallId: &toolCall.ID,
			})
			continue
		}

		var server string
		if mcpServer, ok := args["mcpServer"].(string); ok && mcpServer != "" {
			server = mcpServer
		}

		delete(args, "mcpServer")

		mcpRequest := mcp.Request{
			Method: "tools/call",
			Params: map[string]interface{}{
				"name":      toolCall.Function.Name,
				"arguments": args,
			},
		}

		// Execute the tool call using the MCP client
		a.logger.Debug("Agent: Executing tool call", "toolCall", fmt.Sprintf("id=%s name=%s args=%v server=%s", toolCall.ID, toolCall.Function.Name, args, server))
		result, err := a.mcpClient.ExecuteTool(ctx, mcpRequest, server)
		if err != nil {
			a.logger.Error("Agent: Failed to execute tool call", err, "tool", toolCall.Function.Name)
			results = append(results, providers.Message{
				Role:       providers.MessageRoleTool,
				Content:    fmt.Sprintf("Error: %v", err),
				ToolCallId: &toolCall.ID,
			})
			continue
		}

		var resultStr string
		if result == nil {
			resultStr = "null"
		} else {
			resultBytes, err := json.Marshal(result)
			if err != nil {
				resultStr = fmt.Sprintf("Error marshaling result: %v", err)
			} else {
				resultStr = string(resultBytes)
			}
		}

		results = append(results, providers.Message{
			Role:       providers.MessageRoleTool,
			Content:    resultStr,
			ToolCallId: &toolCall.ID,
		})
	}

	return results, nil
}

// parseStreamingToolCalls parses streaming response to extract tool calls
func (a *agentImpl) parseStreamingToolCalls(responseBody string) ([]providers.ChatCompletionMessageToolCall, error) {
	toolCallsMap := make(map[int]*providers.ChatCompletionMessageToolCall)
	lines := strings.Split(responseBody, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "data: ") {
			continue
		}

		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			break
		}

		var chunk providers.CreateChatCompletionStreamResponse
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue
		}

		if len(chunk.Choices) == 0 || chunk.Choices[0].Delta.ToolCalls == nil {
			continue
		}

		for _, toolCallChunk := range *chunk.Choices[0].Delta.ToolCalls {
			index := toolCallChunk.Index

			if _, exists := toolCallsMap[index]; !exists {
				toolCallsMap[index] = &providers.ChatCompletionMessageToolCall{
					ID:   "",
					Type: providers.ChatCompletionToolTypeFunction,
					Function: providers.ChatCompletionMessageToolCallFunction{
						Name:      "",
						Arguments: "",
					},
				}
			}

			toolCall := toolCallsMap[index]

			if toolCallChunk.ID != nil {
				toolCall.ID = *toolCallChunk.ID
			}

			if toolCallChunk.Type != nil {
				toolCall.Type = providers.ChatCompletionToolType(*toolCallChunk.Type)
			}

			if toolCallChunk.Function != nil {
				type TempToolCallFunction struct {
					Name      string `json:"name,omitempty"`
					Arguments string `json:"arguments,omitempty"`
				}
				type TempToolCall struct {
					Index    int                  `json:"index"`
					Function TempToolCallFunction `json:"function"`
				}
				type TempChoice struct {
					Delta struct {
						ToolCalls []TempToolCall `json:"tool_calls"`
					} `json:"delta"`
				}
				type TempResponse struct {
					Choices []TempChoice `json:"choices"`
				}

				var tempResp TempResponse
				if err := json.Unmarshal([]byte(data), &tempResp); err == nil {
					if len(tempResp.Choices) > 0 {
						for _, tc := range tempResp.Choices[0].Delta.ToolCalls {
							if tc.Index == index {
								if tc.Function.Name != "" {
									toolCall.Function.Name = tc.Function.Name
									a.logger.Debug("Parsed tool name from stream", "name", tc.Function.Name)
								}
								if tc.Function.Arguments != "" {
									toolCall.Function.Arguments += tc.Function.Arguments
									a.logger.Debug("Parsed tool arguments from stream", "args", tc.Function.Arguments)
								}
							}
						}
					}
				}
			}
		}
	}

	var toolCalls []providers.ChatCompletionMessageToolCall
	for i := 0; i < len(toolCallsMap); i++ {
		if toolCall, exists := toolCallsMap[i]; exists {
			a.logger.Debug("Agent: Final parsed tool call", "toolCall", fmt.Sprintf("id=%s name=%s args=%s", toolCall.ID, toolCall.Function.Name, toolCall.Function.Arguments))
			toolCalls = append(toolCalls, *toolCall)
		}
	}

	a.logger.Debug("Agent: Total parsed tool calls", "count", len(toolCalls))
	return toolCalls, nil
}
