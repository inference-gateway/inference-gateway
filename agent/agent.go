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

		a.logger.Debug("agent loop iteration", "iteration", iteration+1, "tool_calls", len(*currentResponse.Choices[0].Message.ToolCalls))

		a.logger.Debug("executing tool calls", "count", len(*currentResponse.Choices[0].Message.ToolCalls))
		toolResults, err := a.ExecuteTools(ctx, *currentResponse.Choices[0].Message.ToolCalls)
		if err != nil {
			a.logger.Error("failed to execute tool calls", err, "iteration", iteration+1)
			return err
		}

		currentRequest.Messages = append(currentRequest.Messages, currentResponse.Choices[0].Message)
		currentRequest.Messages = append(currentRequest.Messages, toolResults...)

		currentRequest.Model = a.providerModel
		nextResponse, err := a.provider.ChatCompletions(ctx, currentRequest)
		if err != nil {
			a.logger.Error("failed to get response in agent loop", err, "iteration", iteration+1, "model", a.providerModel)
			return err
		}

		currentResponse = nextResponse
		iteration++
	}

	if iteration >= MaxAgentIterations {
		a.logger.Warn("agent loop reached maximum iterations", "max_iterations", MaxAgentIterations, "iterations_completed", iteration)
	}

	a.logger.Debug("agent loop completed", "iterations", iteration, "final_choices", len(currentResponse.Choices))

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
	a.logger.Debug("starting agent streaming", "model", currentRequest.Model, "max_iterations", maxIterations)

	defer func() {
		a.logger.Debug("sending agent completion signal")
		middlewareStreamCh <- []byte("data: [DONE]\n\n")
	}()

	for iteration := 0; iteration < maxIterations; iteration++ {
		a.logger.Debug("streaming iteration", "iteration", iteration+1, "max_iterations", maxIterations)

		streamCh, err := a.provider.StreamChatCompletions(ctx, currentRequest)
		if err != nil {
			a.logger.Error("failed to start streaming", err, "iteration", iteration+1, "model", a.providerModel)
			errorData := []byte(fmt.Sprintf("data: {\"error\": \"Failed to start streaming: %s\"}\n\n", err.Error()))
			middlewareStreamCh <- errorData
			return err
		}

		var responseBodyBuilder strings.Builder
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
					a.logger.Debug("stream channel closed", "iteration", iteration+1)
					streamComplete = true
					break
				}

				lineStr := string(line)
				trimmedLine := strings.TrimSpace(lineStr)

				if strings.Contains(trimmedLine, "[DONE]") {
					responseBodyBuilder.Write(line)
					continue
				}

				if !strings.HasPrefix(trimmedLine, "data: ") {
					continue
				}

				chunkData := strings.TrimPrefix(trimmedLine, "data: ")
				if chunkData == "" {
					continue
				}

				formattedData := []byte(fmt.Sprintf("data: %s\n\n", chunkData))
				middlewareStreamCh <- formattedData
				responseBodyBuilder.Write(formattedData)

				var resp providers.CreateChatCompletionStreamResponse
				if err := json.Unmarshal([]byte(chunkData), &resp); err != nil {
					a.logger.Debug("failed to unmarshal streaming chunk", err, "chunk_data", chunkData, "iteration", iteration+1)
					continue
				}

				if len(resp.Choices) == 0 {
					continue
				}

				choice := resp.Choices[0]

				if choice.Delta.Content != "" {
					assistantMessage.Content += choice.Delta.Content
				}

				if choice.Delta.ToolCalls != nil && len(*choice.Delta.ToolCalls) > 0 {
					a.logger.Debug("found tool calls in delta", "count", len(*choice.Delta.ToolCalls), "iteration", iteration+1)
					for _, toolCall := range *choice.Delta.ToolCalls {
						if toolCall.ID != nil || (toolCall.Function != nil && (toolCall.Function.Name != "" || toolCall.Function.Arguments != "")) {
							a.logger.Debug("valid tool call detected", "id", toolCall.ID, "function_name", toolCall.Function)
							hasToolCalls = true
							break
						}
					}
				}

				switch choice.FinishReason {
				case providers.FinishReasonToolCalls:
					a.logger.Debug("stream completing due to tool calls finish reason", "finish_reason", string(choice.FinishReason), "iteration", iteration+1)
					streamComplete = true
				case providers.FinishReasonStop:
					a.logger.Debug("stream completing due to stop finish reason", "finish_reason", string(choice.FinishReason), "iteration", iteration+1)
					streamComplete = true
				}

			case <-ctx.Done():
				a.logger.Debug("context cancelled during streaming", "iteration", iteration+1)
				return ctx.Err()
			}
		}

		a.logger.Debug("stream completed for iteration", "iteration", iteration+1, "has_tool_calls", hasToolCalls)

		var toolCalls []providers.ChatCompletionMessageToolCall
		if hasToolCalls {
			toolCalls, err = a.parseStreamingToolCalls(responseBodyBuilder.String())
			if err != nil {
				a.logger.Error("failed to parse streaming tool calls", err, "iteration", iteration+1)
			} else {
				a.logger.Debug("parsed tool calls from stream", "count", len(toolCalls), "iteration", iteration+1)
			}
		}

		if len(toolCalls) > 0 {
			assistantMessage.ToolCalls = &toolCalls
		}

		if len(toolCalls) == 0 {
			a.logger.Debug("no tool calls found, ending agent loop", "iteration", iteration+1)
			return nil
		}

		a.logger.Debug("executing tool calls", "count", len(toolCalls), "iteration", iteration+1)
		toolResults, err := a.ExecuteTools(ctx, toolCalls)
		if err != nil {
			a.logger.Error("failed to execute tool calls", err, "iteration", iteration+1, "tool_count", len(toolCalls))
			errorData := []byte(fmt.Sprintf("data: {\"error\": \"Failed to execute tools: %s\"}\n\n", err.Error()))
			middlewareStreamCh <- errorData
			return err
		}

		currentRequest.Messages = append(currentRequest.Messages, assistantMessage)
		currentRequest.Messages = append(currentRequest.Messages, toolResults...)
		currentRequest.Model = a.providerModel

		a.logger.Debug("tool execution complete, continuing to next iteration",
			"tool_results", len(toolResults), "total_messages", len(currentRequest.Messages), "iteration", iteration+1)
	}

	a.logger.Warn("agent streaming reached maximum iterations", "max_iterations", maxIterations, "iterations_completed", maxIterations)
	return nil
}

// ExecuteTools executes tools with the provided context, tool name, and arguments
func (a *agentImpl) ExecuteTools(ctx context.Context, toolCalls []providers.ChatCompletionMessageToolCall) ([]providers.Message, error) {
	var results []providers.Message

	for _, toolCall := range toolCalls {
		var args map[string]interface{}
		if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &args); err != nil {
			a.logger.Error("failed to parse tool arguments", err, "args", toolCall.Function.Arguments, "tool_name", toolCall.Function.Name)
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

		a.logger.Info("executing tool call", "tool_call", fmt.Sprintf("id=%s name=%s args=%v server=%s", toolCall.ID, toolCall.Function.Name, args, server))
		result, err := a.mcpClient.ExecuteTool(ctx, mcpRequest, server)
		if err != nil {
			a.logger.Error("failed to execute tool call", err, "tool", toolCall.Function.Name, "server", server)
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
func (a *agentImpl) parseStreamingToolCalls(responseBodyBuilder string) ([]providers.ChatCompletionMessageToolCall, error) {
	toolCallsMap := make(map[int]*providers.ChatCompletionMessageToolCall)
	lines := strings.Split(responseBodyBuilder, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)

		var data string
		switch {
		case strings.HasPrefix(line, "data: "):
			data = strings.TrimPrefix(line, "data: ")
		case line != "" && line != "[DONE]":
			data = line
		default:
			continue
		}

		if data == "[DONE]" || data == "" {
			break
		}

		var chunk providers.CreateChatCompletionStreamResponse
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			a.logger.Debug("failed to parse streaming chunk", "data", data, "error", err)
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
									a.logger.Debug("parsed tool name from stream", "name", tc.Function.Name)
								}
								if tc.Function.Arguments != "" {
									toolCall.Function.Arguments += tc.Function.Arguments
									a.logger.Debug("parsed tool arguments from stream", "args", tc.Function.Arguments)
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
			a.logger.Debug("final parsed tool call", "tool_call", fmt.Sprintf("id=%s name=%s args=%s", toolCall.ID, toolCall.Function.Name, toolCall.Function.Arguments))
			toolCalls = append(toolCalls, *toolCall)
		}
	}

	a.logger.Debug("total parsed tool calls", "count", len(toolCalls))
	return toolCalls, nil
}
