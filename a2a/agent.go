package a2a

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/inference-gateway/inference-gateway/config"
	"github.com/inference-gateway/inference-gateway/logger"
	"github.com/inference-gateway/inference-gateway/providers"
)

// MaxAgentIterations limits the number of agent loop iterations
const MaxAgentIterations = 10

// Supported A2A tool function names
const (
	ToolQueryAgentCard    = "query_a2a_agent_card"
	ToolSubmitTaskToAgent = "submit_task_to_agent"
)

// Agent defines the interface for running agent operations
//
//go:generate mockgen -source=agent.go -destination=../tests/mocks/a2a/agent.go -package=a2amocks
type Agent interface {
	Run(ctx context.Context, request *providers.CreateChatCompletionRequest, response *providers.CreateChatCompletionResponse) error
	RunWithStream(ctx context.Context, middlewareStreamCh chan []byte, c *gin.Context, body *providers.CreateChatCompletionRequest) error
	SetProvider(provider providers.IProvider)
	SetModel(model *string)
}

// Ensure agentImpl implements Agent interface at compile time
var _ Agent = (*agentImpl)(nil)

// agentImpl is the concrete implementation of the Agent interface
type agentImpl struct {
	logger    logger.Logger
	a2aClient A2AClientInterface
	provider  providers.IProvider
	model     *string
	a2aConfig *config.A2AConfig
}

// NewAgent creates a new Agent instance
func NewAgent(logger logger.Logger, a2aClient A2AClientInterface, a2aConfig *config.A2AConfig) Agent {
	return &agentImpl{
		a2aClient: a2aClient,
		logger:    logger,
		provider:  nil,
		model:     nil,
		a2aConfig: a2aConfig,
	}
}

func (a *agentImpl) SetProvider(provider providers.IProvider) {
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

func (a *agentImpl) RunWithStream(ctx context.Context, middlewareStreamCh chan []byte, c *gin.Context, body *providers.CreateChatCompletionRequest) error {
	if err := a.validateConfiguration(); err != nil {
		return err
	}

	currentRequest := *body
	currentRequest.Model = *a.model

	a.logger.Debug("starting a2a agent streaming", "model", currentRequest.Model, "max_iterations", MaxAgentIterations)

	defer func() {
		a.logger.Debug("sending a2a agent completion signal")
		middlewareStreamCh <- []byte("data: [DONE]\n\n")
	}()

	for iteration := 0; iteration < MaxAgentIterations; iteration++ {
		a.logger.Debug("a2a agent streaming iteration", "iteration", iteration+1, "max_iterations", MaxAgentIterations)

		streamCh, err := a.provider.StreamChatCompletions(ctx, currentRequest)
		if err != nil {
			a.logger.Error("failed to start streaming", err, "iteration", iteration+1, "model", *a.model)
			errorData := []byte(fmt.Sprintf("data: {\"error\": \"Failed to start streaming: %s\"}\n\n", err.Error()))
			middlewareStreamCh <- errorData
			return err
		}

		toolCalls, err := a.processStreamingResponse(streamCh, middlewareStreamCh, iteration)
		if err != nil {
			return err
		}

		if len(toolCalls) == 0 {
			a.logger.Debug("no tool calls found, ending a2a agent loop", "iteration", iteration+1)
			return nil
		}

		assistantMessage := providers.Message{
			Role:      providers.MessageRoleAssistant,
			Content:   "",
			ToolCalls: &toolCalls,
		}

		currentRequest.Messages = append(currentRequest.Messages, assistantMessage)

		for _, toolCall := range toolCalls {
			toolResult := a.processToolCall(ctx, &currentRequest, toolCall)
			currentRequest.Messages = append(currentRequest.Messages, toolResult)
		}
	}

	a.logger.Warn("a2a agent streaming reached maximum iterations", "max_iterations", MaxAgentIterations, "iterations_completed", MaxAgentIterations)
	return nil
}

func (a *agentImpl) Run(ctx context.Context, request *providers.CreateChatCompletionRequest, response *providers.CreateChatCompletionResponse) error {
	if err := a.validateConfiguration(); err != nil {
		return err
	}

	for iteration := 0; iteration < MaxAgentIterations; iteration++ {
		toolCalls := a.extractToolCalls(response)
		if len(toolCalls) == 0 {
			a.logger.Debug("no tool calls to handle, ending agent loop", "iteration", iteration)
			return nil
		}

		a.logger.Debug("processing tool calls", "count", len(toolCalls), "iteration", iteration+1)

		request.Messages = append(request.Messages, response.Choices[0].Message)

		allA2ATasksCompleted := true
		var toolResults []providers.Message

		for _, toolCall := range toolCalls {
			toolResult := a.processToolCall(ctx, request, toolCall)
			toolResults = append(toolResults, toolResult)
			request.Messages = append(request.Messages, toolResult)

			if toolCall.Function.Name != ToolSubmitTaskToAgent {
				allA2ATasksCompleted = false
			}
		}

		if allA2ATasksCompleted && len(toolResults) > 0 {
			a.logger.Debug("all a2a tasks completed, generating final response", "iteration", iteration+1)

			var combinedContent string
			for i, result := range toolResults {
				if i > 0 {
					combinedContent += "\n\n"
				}
				combinedContent += result.Content
			}

			*response = providers.CreateChatCompletionResponse{
				ID:      response.ID,
				Object:  response.Object,
				Created: response.Created,
				Model:   response.Model,
				Choices: []providers.ChatCompletionChoice{
					{
						Index: 0,
						Message: providers.Message{
							Role:    providers.MessageRoleAssistant,
							Content: combinedContent,
						},
						FinishReason: providers.FinishReasonStop,
					},
				},
			}
			return nil
		}

		request.Model = *a.model
		nextResponse, err := a.provider.ChatCompletions(ctx, *request)
		if err != nil {
			return fmt.Errorf("failed to get response after tool execution: %w", err)
		}

		*response = nextResponse
	}

	a.logger.Warn("agent reached maximum iterations", "max_iterations", MaxAgentIterations)
	return nil
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
	for i := 0; len(toolCallsMap) > 0 && i < len(toolCallsMap); i++ {
		if toolCall, exists := toolCallsMap[i]; exists {
			a.logger.Debug("final parsed a2a tool call", "tool_call", fmt.Sprintf("id=%s name=%s args=%s", toolCall.ID, toolCall.Function.Name, toolCall.Function.Arguments))
			toolCalls = append(toolCalls, *toolCall)
		}
	}

	a.logger.Debug("total parsed a2a tool calls", "count", len(toolCalls))
	return toolCalls, nil
}

// validateConfiguration checks if provider and model are set
func (a *agentImpl) validateConfiguration() error {
	if a.provider == nil {
		return errors.New("provider is not set for agent")
	}
	if a.model == nil {
		return errors.New("model is not set for agent")
	}
	return nil
}

// extractToolCalls extracts tool calls from the response
func (a *agentImpl) extractToolCalls(response *providers.CreateChatCompletionResponse) []providers.ChatCompletionMessageToolCall {
	if len(response.Choices) == 0 || response.Choices[0].Message.ToolCalls == nil {
		return nil
	}
	return *response.Choices[0].Message.ToolCalls
}

// processToolCall processes a single tool call and returns the result message
func (a *agentImpl) processToolCall(ctx context.Context, request *providers.CreateChatCompletionRequest, toolCall providers.ChatCompletionMessageToolCall) providers.Message {
	var result providers.Message
	var err error

	switch toolCall.Function.Name {
	case ToolQueryAgentCard:
		result, err = a.handleAgentQueryTool(ctx, toolCall)
	case ToolSubmitTaskToAgent:
		result, err = a.handleTaskSubmissionTool(ctx, request, toolCall)
	default:
		a.logger.Warn("unknown tool call", "function_name", toolCall.Function.Name)
		result = providers.Message{
			Role:       providers.MessageRoleTool,
			Content:    fmt.Sprintf("Unknown tool: %s", toolCall.Function.Name),
			ToolCallId: &toolCall.ID,
		}
		return result
	}

	if err != nil {
		a.logger.Error("failed to process tool call", err, "function_name", toolCall.Function.Name)
		result = providers.Message{
			Role:       providers.MessageRoleTool,
			Content:    fmt.Sprintf("Error processing %s: %s", toolCall.Function.Name, err.Error()),
			ToolCallId: &toolCall.ID,
		}
	}

	return result
}

// handleAgentQueryTool handles the query_a2a_agent_card tool call
func (a *agentImpl) handleAgentQueryTool(ctx context.Context, toolCall providers.ChatCompletionMessageToolCall) (providers.Message, error) {
	var args struct {
		AgentURL string `json:"agent_url"`
	}

	if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &args); err != nil {
		return providers.Message{}, fmt.Errorf("failed to parse tool arguments: %w", err)
	}

	a.logger.Debug("querying agent card", "agent_url", args.AgentURL)

	agentCard, err := a.a2aClient.GetAgentCard(ctx, args.AgentURL)
	if err != nil {
		return providers.Message{}, fmt.Errorf("failed to get agent card: %w", err)
	}

	var skillDescriptions []string
	for _, skill := range agentCard.Skills {
		description := fmt.Sprintf("- **%s** (ID: %s): %s", skill.Name, skill.ID, skill.Description)

		if len(skill.InputModes) > 0 {
			description += fmt.Sprintf("\n  - Input modes: %s", strings.Join(skill.InputModes, ", "))
		}
		if len(skill.OutputModes) > 0 {
			description += fmt.Sprintf("\n  - Output modes: %s", strings.Join(skill.OutputModes, ", "))
		}

		if len(skill.Examples) > 0 {
			description += fmt.Sprintf("\n  - Examples: %s", strings.Join(skill.Examples, "; "))
		}

		if len(skill.Tags) > 0 {
			description += fmt.Sprintf("\n  - Tags: %s", strings.Join(skill.Tags, ", "))
		}

		skillDescriptions = append(skillDescriptions, description)
	}

	resultContent := fmt.Sprintf("Agent '%s' (v%s) provides the following capabilities:\n\n%s\n\nTo use any of these skills, you can send a message to the agent at %s using the A2A protocol with the skill ID.",
		agentCard.Name, agentCard.Version, strings.Join(skillDescriptions, "\n\n"), args.AgentURL)

	return providers.Message{
		Role:       providers.MessageRoleTool,
		Content:    resultContent,
		ToolCallId: &toolCall.ID,
	}, nil
}

// handleTaskSubmissionTool handles the submit_task_to_agent tool call
func (a *agentImpl) handleTaskSubmissionTool(ctx context.Context, request *providers.CreateChatCompletionRequest, toolCall providers.ChatCompletionMessageToolCall) (providers.Message, error) {
	var args struct {
		AgentURL          string `json:"agent_url"`
		TaskDescription   string `json:"task_description"`
		AdditionalContext string `json:"additional_context,omitempty"`
	}

	if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &args); err != nil {
		return providers.Message{}, fmt.Errorf("failed to parse tool arguments: %w", err)
	}

	a.logger.Debug("submitting task to a2a agent", "agent_url", args.AgentURL, "task", args.TaskDescription)

	taskMessage := args.TaskDescription
	if args.AdditionalContext != "" {
		taskMessage += "\n\nAdditional context: " + args.AdditionalContext
	}

	capabilities := a.a2aClient.GetAgentCapabilities()
	agentCapability, hasCapability := capabilities[args.AgentURL]
	supportsStreaming := hasCapability && agentCapability.Streaming != nil && *agentCapability.Streaming
	requestIsStreaming := request.Stream != nil && *request.Stream

	if supportsStreaming && requestIsStreaming {
		a.logger.Debug("using streaming communication with a2a agent", "agent_url", args.AgentURL, "reason", "both agent supports streaming and request is streaming")
		return a.handleStreamingTaskSubmission(ctx, request, toolCall, args.AgentURL, taskMessage)
	}

	switch {
	case supportsStreaming && !requestIsStreaming:
		a.logger.Debug("using non-streaming communication with a2a agent", "agent_url", args.AgentURL, "reason", "agent supports streaming but request is non-streaming")
	case !supportsStreaming:
		a.logger.Debug("using non-streaming communication with a2a agent", "agent_url", args.AgentURL, "reason", "agent does not support streaming")
	default:
		a.logger.Debug("using non-streaming communication with a2a agent", "agent_url", args.AgentURL)
	}
	return a.handleNonStreamingTaskSubmission(ctx, request, toolCall, args.AgentURL, taskMessage)
}

// extractTaskResponse extracts the text response from a completed task
func (a *agentImpl) extractTaskResponse(task *Task, toolCallID string) (providers.Message, error) {
	if task.Status.Message == nil {
		return providers.Message{}, errors.New("task completion returned no message")
	}

	responseContent := "Task completed successfully"
	message := task.Status.Message

	if len(message.Parts) == 0 {
		a.logger.Debug("no message parts found")
		return providers.Message{}, errors.New("task completion returned no message parts")
	}

	a.logger.Debug("processing message parts", "parts_count", len(message.Parts))
	for i, part := range message.Parts {
		a.logger.Debug("processing part", "part_index", i, "part_type", fmt.Sprintf("%T", part))

		var textContent string

		if textPart, ok := part.(TextPart); ok {
			a.logger.Debug("found text part struct", "text_length", len(textPart.Text), "text_content", textPart.Text)
			if textPart.Text == "" {
				continue
			}
			textContent = textPart.Text
		} else if partMap, ok := part.(map[string]interface{}); ok {
			keys := make([]string, 0, len(partMap))
			for k := range partMap {
				keys = append(keys, k)
			}
			a.logger.Debug("found part map", "keys", keys)

			kind, kindExists := partMap["kind"]
			if !kindExists || kind != "text" {
				continue
			}

			text, textExists := partMap["text"].(string)
			if !textExists || text == "" {
				continue
			}

			a.logger.Debug("extracted text from part map", "text_length", len(text), "text_content", text)
			textContent = text
		} else {
			a.logger.Debug("part is not a recognized text type", "actual_type", fmt.Sprintf("%T", part))
			continue
		}

		responseContent = textContent
		a.logger.Debug("using text content as response", "content", responseContent)
		break
	}

	a.logger.Debug("final task response", "response_content", responseContent, "task_status", task.Status.State)

	return providers.Message{
		Role:       providers.MessageRoleTool,
		Content:    responseContent,
		ToolCallId: &toolCallID,
	}, nil
}

// pollTaskUntilCompletion polls the task until it's completed and returns the final task
func (a *agentImpl) pollTaskUntilCompletion(ctx context.Context, taskID, agentURL string) (*Task, error) {
	ticker := time.NewTicker(a.a2aConfig.PollingInterval)
	defer ticker.Stop()

	maxAttempts := a.a2aConfig.MaxPollAttempts
	attempts := 0

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
			attempts++
			if attempts > maxAttempts {
				return nil, fmt.Errorf("task polling timeout after %d attempts", maxAttempts)
			}

			getTaskRequest := &GetTaskRequest{
				JSONRPC: "2.0",
				Method:  "tasks/get",
				Params: TaskQueryParams{
					ID: taskID,
				},
			}

			response, err := a.a2aClient.GetTask(ctx, getTaskRequest, agentURL)
			if err != nil {
				a.logger.Debug("failed to get task status", "task_id", taskID, "agent_url", agentURL, "attempt", attempts, "error", err.Error())
				continue
			}

			task := response.Result

			a.logger.Debug("task status check",
				"task_id", taskID,
				"status", task.Status.State,
				"agent_url", agentURL,
				"attempt", attempts)

			switch task.Status.State {
			case TaskStateCompleted:
				a.logger.Info("task completed successfully via polling", "task_id", taskID, "agent_url", agentURL, "attempts", attempts)
				return &task, nil
			case TaskStateFailed:
				a.logger.Error("task failed", fmt.Errorf("task failed"), "task_id", taskID, "agent_url", agentURL, "attempts", attempts)
				return &task, fmt.Errorf("task failed: %s", taskID)
			case TaskStateCanceled:
				a.logger.Info("task was canceled", "task_id", taskID, "agent_url", agentURL, "attempts", attempts)
				return &task, fmt.Errorf("task canceled: %s", taskID)
			case TaskStateRejected:
				a.logger.Error("task was rejected", fmt.Errorf("task rejected"), "task_id", taskID, "agent_url", agentURL, "attempts", attempts)
				return &task, fmt.Errorf("task rejected: %s", taskID)
			case TaskStateSubmitted, TaskStateWorking, TaskStateInputRequired, TaskStateAuthRequired:
				a.logger.Debug("task still in progress", "task_id", taskID, "status", task.Status.State, "agent_url", agentURL)
				continue
			default:
				a.logger.Debug("unknown task state, continuing polling", "task_id", taskID, "state", task.Status.State, "agent_url", agentURL)
				continue
			}
		}
	}
}

// processStreamingResponse processes a streaming response and returns extracted tool calls
func (a *agentImpl) processStreamingResponse(streamCh <-chan []byte, middlewareStreamCh chan []byte, iteration int) ([]providers.ChatCompletionMessageToolCall, error) {
	var responseBodyBuilder strings.Builder
	streamComplete := false
	hasToolCalls := false

	for !streamComplete {
		line, ok := <-streamCh
		if !ok {
			a.logger.Debug("stream channel closed", "iteration", iteration+1)
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
	}

	a.logger.Debug("stream completed for iteration", "iteration", iteration+1, "has_tool_calls", hasToolCalls)

	var toolCalls []providers.ChatCompletionMessageToolCall
	if hasToolCalls {
		var err error
		toolCalls, err = a.parseStreamingToolCalls(responseBodyBuilder.String())
		if err != nil {
			a.logger.Error("failed to parse streaming tool calls", err, "iteration", iteration+1)
			return nil, err
		}
		a.logger.Debug("parsed tool calls from stream", "count", len(toolCalls), "iteration", iteration+1)
	}

	return toolCalls, nil
}

// handleStreamingTaskSubmission handles task submission using streaming A2A communication
func (a *agentImpl) handleStreamingTaskSubmission(ctx context.Context, request *providers.CreateChatCompletionRequest, toolCall providers.ChatCompletionMessageToolCall, agentURL, taskMessage string) (providers.Message, error) {
	streamingRequest := &SendStreamingMessageRequest{
		ID:      "stream-task-" + fmt.Sprintf("%d", len(request.Messages)),
		JSONRPC: "2.0",
		Method:  "message/stream",
		Params: MessageSendParams{
			Message: Message{
				Kind:      "message",
				MessageID: fmt.Sprintf("stream-msg-%d", len(request.Messages)),
				Role:      "user",
				Parts: []Part{
					TextPart{
						Kind: "text",
						Text: taskMessage,
					},
				},
				Metadata: map[string]interface{}{
					"tool_call": map[string]interface{}{
						"id":        toolCall.ID,
						"function":  toolCall.Function.Name,
						"arguments": toolCall.Function.Arguments,
					},
				},
			},
		},
	}

	streamCh, err := a.a2aClient.SendStreamingMessage(ctx, streamingRequest, agentURL)
	if err != nil {
		a.logger.Debug("streaming failed, falling back to non-streaming", "agent_url", agentURL, "error", err.Error())
		return a.handleNonStreamingTaskSubmission(ctx, request, toolCall, agentURL, taskMessage)
	}

	a.logger.Debug("processing streaming response from a2a agent", "agent_url", agentURL)

	var responseContent strings.Builder
	for {
		select {
		case line, ok := <-streamCh:
			if !ok {
				a.logger.Debug("streaming response completed", "agent_url", agentURL)
				goto ProcessComplete
			}

			lineStr := string(line)
			a.logger.Debug("received streaming chunk from a2a agent", "agent_url", agentURL, "chunk", lineStr)

			if strings.HasPrefix(lineStr, "data: ") {
				dataStr := strings.TrimPrefix(lineStr, "data: ")
				dataStr = strings.TrimSpace(dataStr)

				if dataStr == "" || dataStr == "[DONE]" {
					continue
				}

				var sseEvent map[string]interface{}
				if err := json.Unmarshal([]byte(dataStr), &sseEvent); err != nil {
					a.logger.Debug("failed to parse SSE event", "data", dataStr, "error", err.Error())
					continue
				}

				if result, exists := sseEvent["result"].(map[string]interface{}); exists {
					if kind, exists := result["kind"].(string); exists {
						switch kind {
						case "artifact-update":
							if artifact, exists := result["artifact"].(map[string]interface{}); exists {
								if parts, exists := artifact["parts"].([]interface{}); exists {
									for _, part := range parts {
										if partMap, ok := part.(map[string]interface{}); ok {
											if partType, exists := partMap["type"].(string); exists && partType == "text" {
												if text, exists := partMap["text"].(string); exists {
													responseContent.WriteString(text)
												}
											}
										}
									}
								}
							}
						case "status-update":
							if status, exists := result["status"].(map[string]interface{}); exists {
								if state, exists := status["state"].(string); exists {
									a.logger.Debug("task status update", "state", state, "agent_url", agentURL)
									if state == "completed" {
										if final, exists := result["final"].(bool); exists && final {
											goto ProcessComplete
										}
									}
								}
							}
						}
					}
				}
			}

		case <-ctx.Done():
			a.logger.Debug("context cancelled during streaming", "agent_url", agentURL)
			return providers.Message{}, ctx.Err()
		}
	}

ProcessComplete:
	finalResponse := responseContent.String()
	if finalResponse == "" {
		finalResponse = "Task completed successfully via streaming"
	}

	a.logger.Info("streaming task completed", "agent_url", agentURL, "response_length", len(finalResponse))

	return providers.Message{
		Role:       providers.MessageRoleTool,
		Content:    finalResponse,
		ToolCallId: &toolCall.ID,
	}, nil
}

// handleNonStreamingTaskSubmission handles task submission using traditional blocking A2A communication
func (a *agentImpl) handleNonStreamingTaskSubmission(ctx context.Context, request *providers.CreateChatCompletionRequest, toolCall providers.ChatCompletionMessageToolCall, agentURL, taskMessage string) (providers.Message, error) {
	taskRequest := &SendMessageRequest{
		ID:      "task-" + fmt.Sprintf("%d", len(request.Messages)),
		JSONRPC: "2.0",
		Method:  "message/send",
		Params: MessageSendParams{
			Message: Message{
				Kind:      "message",
				MessageID: fmt.Sprintf("msg-%d", len(request.Messages)),
				Role:      "user",
				Parts: []Part{
					TextPart{
						Kind: "text",
						Text: taskMessage,
					},
				},
				Metadata: map[string]interface{}{
					"tool_call": map[string]interface{}{
						"id":        toolCall.ID,
						"function":  toolCall.Function.Name,
						"arguments": toolCall.Function.Arguments,
					},
				},
			},
		},
	}

	response, err := a.a2aClient.SendMessage(ctx, taskRequest, agentURL)
	if err != nil {
		return providers.Message{}, fmt.Errorf("failed to submit task to a2a agent: %w", err)
	}

	a.logger.Debug("task submitted successfully", "task_id", response.Result, "agent_url", agentURL)

	resultBytes, err := json.Marshal(response.Result)
	if err != nil {
		return providers.Message{}, fmt.Errorf("failed to marshal response result: %w", err)
	}

	var task Task
	if err := json.Unmarshal(resultBytes, &task); err != nil {
		return providers.Message{}, fmt.Errorf("failed to unmarshal task from response: %w", err)
	}

	taskID := task.ID
	a.logger.Debug("received task ID from agent", "task_id", taskID, "agent_url", agentURL)

	if task.Status.State == TaskStateCompleted {
		a.logger.Info("task completed immediately", "task_id", taskID, "agent_url", agentURL)
		return a.extractTaskResponse(&task, toolCall.ID)
	}

	a.logger.Debug("task not completed immediately, starting polling", "task_id", taskID, "status", task.Status.State, "agent_url", agentURL)

	completedTask, err := a.pollTaskUntilCompletion(ctx, taskID, agentURL)
	if err != nil {
		return providers.Message{}, fmt.Errorf("failed to poll task completion: %w", err)
	}

	a.logger.Info("task completed via polling", "task_id", taskID, "agent_url", agentURL)
	return a.extractTaskResponse(completedTask, toolCall.ID)
}
