package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	logger "github.com/inference-gateway/inference-gateway/logger"
	core "github.com/inference-gateway/inference-gateway/providers/core"
	types "github.com/inference-gateway/inference-gateway/providers/types"
	otelapi "go.opentelemetry.io/otel"
	attribute "go.opentelemetry.io/otel/attribute"
	codes "go.opentelemetry.io/otel/codes"
	semconv "go.opentelemetry.io/otel/semconv/v1.41.0"
	trace "go.opentelemetry.io/otel/trace"
)

// MaxAgentIterations limits the number of agent loop iterations
const MaxAgentIterations = 10

// Agent defines the interface for running agent operations
//
//go:generate mockgen -source=agent.go -destination=../../tests/mocks/mcp/agent.go -package=mcpmocks -typed
type Agent interface {
	Run(ctx context.Context, request *types.CreateChatCompletionRequest, response *types.CreateChatCompletionResponse) error
	RunWithStream(ctx context.Context, middlewareStreamCh chan []byte, body *types.CreateChatCompletionRequest) error
	ExecuteTools(ctx context.Context, toolCalls []types.ChatCompletionMessageToolCall) ([]types.Message, error)
	SetProvider(provider core.IProvider)
	SetModel(model *string)
}

// Ensure agentImpl implements Agent interface at compile time
var _ Agent = (*agentImpl)(nil)

// agentImpl is the concrete implementation of the Agent interface
type agentImpl struct {
	logger    logger.Logger
	mcpClient MCPClientInterface
	provider  core.IProvider
	model     *string
}

// NewAgent creates a new Agent instance
func NewAgent(logger logger.Logger, mcpClient MCPClientInterface) Agent {
	return &agentImpl{
		mcpClient: mcpClient,
		logger:    logger,
		provider:  nil,
		model:     nil,
	}
}

func (a *agentImpl) SetProvider(provider core.IProvider) {
	if provider == nil {
		a.logger.Error("attempted to set nil provider", errors.New("provider is nil"))
		return
	}
	a.provider = provider
	a.logger.Debug("provider set for agent", "provider", provider.GetName())
}

func (a *agentImpl) SetModel(model *string) {
	if model == nil {
		a.logger.Error("attempted to set nil model", errors.New("model is nil"))
		return
	}
	a.model = model
	a.logger.Debug("model set for agent", "model", *model)
}

func (a *agentImpl) Run(ctx context.Context, request *types.CreateChatCompletionRequest, response *types.CreateChatCompletionResponse) error {
	if a.provider == nil {
		return errors.New("provider is not set for agent")
	}
	if a.model == nil {
		return errors.New("model is not set for agent")
	}

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

		currentRequest.Model = *a.model
		nextResponse, err := a.provider.ChatCompletions(ctx, currentRequest)
		if err != nil {
			a.logger.Error("failed to get response in agent loop", err, "iteration", iteration+1, "model", a.model)
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

func send(ctx context.Context, ch chan<- []byte, b []byte) bool {
	select {
	case ch <- b:
		return true
	case <-ctx.Done():
		return false
	}
}

// RunWithStream executes the agent with the provided streaming response channel
func (a *agentImpl) RunWithStream(ctx context.Context, middlewareStreamCh chan []byte, body *types.CreateChatCompletionRequest) error {
	if a.provider == nil {
		return errors.New("provider is not set for agent")
	}
	if a.model == nil {
		return errors.New("model is not set for agent")
	}

	currentRequest := *body

	currentRequest.Model = *a.model
	a.logger.Debug("starting agent streaming", "model", currentRequest.Model, "max_iterations", MaxAgentIterations)

	defer func() {
		a.logger.Debug("sending agent completion signal")
		send(ctx, middlewareStreamCh, []byte("data: [DONE]\n\n"))
	}()

	for iteration := range MaxAgentIterations {
		a.logger.Debug("streaming iteration", "iteration", iteration+1, "max_iterations", MaxAgentIterations)

		streamCh, err := a.provider.StreamChatCompletions(ctx, currentRequest)
		if err != nil {
			a.logger.Error("failed to start streaming", err, "iteration", iteration+1, "model", *a.model)
			errorData := []byte(fmt.Sprintf("data: {\"error\": \"Failed to start streaming: %s\"}\n\n", err.Error()))
			send(ctx, middlewareStreamCh, errorData)
			return err
		}

		var responseBodyBuilder strings.Builder
		assistantMessage := types.Message{
			Role:      types.Assistant,
			ToolCalls: nil,
		}
		if err := assistantMessage.Content.FromMessageContent0(""); err != nil {
			a.logger.Error("failed to initialize assistant message content", err)
			return err
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
				if !send(ctx, middlewareStreamCh, formattedData) {
					a.logger.Debug("context cancelled while sending stream chunk", "iteration", iteration+1)
					return ctx.Err()
				}
				responseBodyBuilder.Write(formattedData)

				var resp types.CreateChatCompletionStreamResponse
				if err := json.Unmarshal([]byte(chunkData), &resp); err != nil {
					a.logger.Debug("failed to unmarshal streaming chunk", err, "chunk_data", chunkData, "iteration", iteration+1)
					continue
				}

				if len(resp.Choices) == 0 {
					continue
				}

				choice := resp.Choices[0]

				if choice.Delta.Content != "" {
					if currentContent, err := assistantMessage.Content.AsMessageContent0(); err == nil {
						newContent := currentContent + choice.Delta.Content
						if err := assistantMessage.Content.FromMessageContent0(newContent); err != nil {
							a.logger.Debug("failed to update message content", err)
						}
					} else {
						if err := assistantMessage.Content.FromMessageContent0(choice.Delta.Content); err != nil {
							a.logger.Debug("failed to set message content", err)
						}
					}
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
				case types.ToolCalls:
					a.logger.Debug("stream completing due to tool calls finish reason", "finish_reason", string(choice.FinishReason), "iteration", iteration+1)
					streamComplete = true
				case types.Stop:
					a.logger.Debug("stream completing due to stop finish reason", "finish_reason", string(choice.FinishReason), "iteration", iteration+1)
					streamComplete = true
				}

			case <-ctx.Done():
				a.logger.Debug("context cancelled during streaming", "iteration", iteration+1)
				return ctx.Err()
			}
		}

		a.logger.Debug("stream completed for iteration", "iteration", iteration+1, "has_tool_calls", hasToolCalls)

		var toolCalls []types.ChatCompletionMessageToolCall
		if hasToolCalls {
			toolCalls = types.AccumulateStreamingToolCalls(responseBodyBuilder.String())
			a.logger.Debug("parsed tool calls from stream", "count", len(toolCalls), "iteration", iteration+1)
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
			send(ctx, middlewareStreamCh, errorData)
			return err
		}

		currentRequest.Messages = append(currentRequest.Messages, assistantMessage)
		currentRequest.Messages = append(currentRequest.Messages, toolResults...)
		currentRequest.Model = *a.model

		a.logger.Debug("tool execution complete, continuing to next iteration",
			"tool_results", len(toolResults), "total_messages", len(currentRequest.Messages), "iteration", iteration+1)
	}

	a.logger.Warn("agent streaming reached maximum iterations", "max_iterations", MaxAgentIterations, "iterations_completed", MaxAgentIterations)
	return nil
}

// ExecuteTools executes tools with the provided context, tool name, and arguments
func (a *agentImpl) ExecuteTools(ctx context.Context, toolCalls []types.ChatCompletionMessageToolCall) ([]types.Message, error) {
	var results []types.Message

	for _, toolCall := range toolCalls {
		var args map[string]any
		if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &args); err != nil {
			a.logger.Error("failed to parse tool arguments", err, "args", toolCall.Function.Arguments, "tool_name", toolCall.Function.Name)
			msg := types.Message{
				Role:       types.Tool,
				ToolCallID: &toolCall.ID,
			}
			if contentErr := msg.Content.FromMessageContent0(fmt.Sprintf("Error: Failed to parse arguments: %v", err)); contentErr != nil {
				a.logger.Error("failed to set error content", contentErr)
			}
			results = append(results, msg)
			continue
		}

		var server string
		toolName := strings.TrimPrefix(toolCall.Function.Name, "mcp_")
		toolCtx, span := otelapi.Tracer("github.com/inference-gateway/inference-gateway/internal/mcp").
			Start(ctx, "execute_tool "+toolName, trace.WithAttributes(semconv.GenAIToolName(toolName)))
		server, err := a.mcpClient.GetServerForTool(toolName)
		if err != nil {
			span.SetStatus(codes.Error, err.Error())
			span.End()
			a.logger.Error("failed to find server for tool", err, "tool", toolCall.Function.Name, "tool_name", toolName)
			msg := types.Message{
				Role:       types.Tool,
				ToolCallID: &toolCall.ID,
			}
			if contentErr := msg.Content.FromMessageContent0(fmt.Sprintf("Error: %v", err)); contentErr != nil {
				a.logger.Error("failed to set error content", contentErr)
			}
			results = append(results, msg)
			continue
		}
		span.SetAttributes(attribute.String("mcp.server.url", server))

		mcpRequest := Request{
			Method: "tools/call",
			Params: map[string]any{
				"name":      toolName,
				"arguments": args,
			},
		}

		a.logger.Info("executing tool call", "tool_call", fmt.Sprintf("id=%s name=%s mcp_name=%s args=%v server=%s", toolCall.ID, toolCall.Function.Name, toolName, args, server))
		result, err := a.mcpClient.ExecuteTool(toolCtx, mcpRequest, server)
		if err != nil {
			span.SetStatus(codes.Error, err.Error())
			span.End()
			a.logger.Error("failed to execute tool call", err, "tool", toolCall.Function.Name, "server", server)
			msg := types.Message{
				Role:       types.Tool,
				ToolCallID: &toolCall.ID,
			}
			if contentErr := msg.Content.FromMessageContent0(fmt.Sprintf("Error: %v", err)); contentErr != nil {
				a.logger.Error("failed to set error content", contentErr)
			}
			results = append(results, msg)
			continue
		}
		span.End()

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

		msg := types.Message{
			Role:       types.Tool,
			ToolCallID: &toolCall.ID,
		}
		if err := msg.Content.FromMessageContent0(resultStr); err != nil {
			a.logger.Error("failed to set tool result content", err)
			return nil, err
		}
		results = append(results, msg)
	}

	return results, nil
}
