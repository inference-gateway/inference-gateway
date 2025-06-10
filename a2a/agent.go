package a2a

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/inference-gateway/inference-gateway/logger"
	"github.com/inference-gateway/inference-gateway/providers"
)

// MaxAgentIterations limits the number of agent loop iterations
const MaxAgentIterations = 10

// Agent defines the interface for running agent operations
//
//go:generate mockgen -source=agent.go -destination=../tests/mocks/a2a/agent.go -package=a2amocks
type Agent interface {
	Run(ctx context.Context, request *providers.CreateChatCompletionRequest, response *providers.CreateChatCompletionResponse) error
	RunWithStream(ctx context.Context, middlewareStreamCh chan []byte, c *gin.Context, body *providers.CreateChatCompletionRequest) error
	SetProvider(provider providers.IProvider)
	SetModel(model *string)
	ExecuteTools(ctx context.Context, toolCalls []providers.ChatCompletionMessageToolCall) ([]providers.Message, error)
}

// Ensure agentImpl implements Agent interface at compile time
var _ Agent = (*agentImpl)(nil)

// agentImpl is the concrete implementation of the Agent interface
type agentImpl struct {
	logger          logger.Logger
	a2aClient       A2AClientInterface
	provider        providers.IProvider
	model           *string
	discoveredTools []providers.ChatCompletionTool
	toolToAgentMap  map[string]string
}

// NewAgent creates a new Agent instance
func NewAgent(logger logger.Logger, a2aClient A2AClientInterface) Agent {
	return &agentImpl{
		a2aClient:      a2aClient,
		logger:         logger,
		provider:       nil,
		model:          nil,
		toolToAgentMap: make(map[string]string),
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
	if a.provider == nil {
		return errors.New("provider is not set for agent")
	}
	if a.model == nil {
		return errors.New("model is not set for agent")
	}

	currentRequest := *body
	maxIterations := MaxAgentIterations

	currentRequest.Model = *a.model
	a.logger.Debug("starting a2a agent streaming", "model", currentRequest.Model, "max_iterations", maxIterations)

	defer func() {
		a.logger.Debug("sending a2a agent completion signal")
		middlewareStreamCh <- []byte("data: [DONE]\n\n")
	}()

	for iteration := 0; iteration < maxIterations; iteration++ {
		a.logger.Debug("a2a agent streaming iteration", "iteration", iteration+1, "max_iterations", maxIterations)

		streamCh, err := a.provider.StreamChatCompletions(ctx, currentRequest)
		if err != nil {
			a.logger.Error("failed to start streaming", err, "iteration", iteration+1, "model", *a.model)
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
			a.logger.Debug("no tool calls found, ending a2a agent loop", "iteration", iteration+1)
			return nil
		}

		a.logger.Debug("executing a2a tool calls", "count", len(toolCalls), "iteration", iteration+1)
		toolResults, err := a.ExecuteTools(ctx, toolCalls)
		if err != nil {
			a.logger.Error("failed to execute a2a tool calls", err, "iteration", iteration+1, "tool_count", len(toolCalls))
			errorData := []byte(fmt.Sprintf("data: {\"error\": \"Failed to execute A2A tools: %s\"}\n\n", err.Error()))
			middlewareStreamCh <- errorData
			return err
		}

		currentRequest.Messages = append(currentRequest.Messages, assistantMessage)
		currentRequest.Messages = append(currentRequest.Messages, toolResults...)
		currentRequest.Model = *a.model

		a.logger.Debug("a2a tool execution complete, continuing to next iteration",
			"tool_results", len(toolResults), "total_messages", len(currentRequest.Messages), "iteration", iteration+1)
	}

	a.logger.Warn("a2a agent streaming reached maximum iterations", "max_iterations", maxIterations, "iterations_completed", maxIterations)
	return nil
}

func (a *agentImpl) Run(ctx context.Context, request *providers.CreateChatCompletionRequest, response *providers.CreateChatCompletionResponse) error {
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

		a.logger.Debug("a2a agent loop iteration", "iteration", iteration+1, "tool_calls", len(*currentResponse.Choices[0].Message.ToolCalls))

		a.logger.Debug("executing a2a tool calls", "count", len(*currentResponse.Choices[0].Message.ToolCalls))
		toolResults, err := a.ExecuteTools(ctx, *currentResponse.Choices[0].Message.ToolCalls)
		if err != nil {
			a.logger.Error("failed to execute a2a tool calls", err, "iteration", iteration+1)
			return err
		}

		currentRequest.Messages = append(currentRequest.Messages, currentResponse.Choices[0].Message)
		currentRequest.Messages = append(currentRequest.Messages, toolResults...)

		if len(a.discoveredTools) > 0 {
			originalTools := currentRequest.Tools
			if originalTools == nil {
				originalTools = &[]providers.ChatCompletionTool{}
			}

			allTools := make([]providers.ChatCompletionTool, len(*originalTools)+len(a.discoveredTools))
			copy(allTools, *originalTools)
			copy(allTools[len(*originalTools):], a.discoveredTools)

			currentRequest.Tools = &allTools
			a.logger.Debug("added discovered tools to request", "original_tools", len(*originalTools), "discovered_tools", len(a.discoveredTools), "total_tools", len(allTools))
		}

		currentRequest.Model = *a.model
		nextResponse, err := a.provider.ChatCompletions(ctx, currentRequest)
		if err != nil {
			a.logger.Error("failed to get response in a2a agent loop", err, "iteration", iteration+1, "model", a.model)
			return err
		}

		currentResponse = nextResponse
		iteration++
	}

	if iteration >= MaxAgentIterations {
		a.logger.Warn("a2a agent loop reached maximum iterations", "max_iterations", MaxAgentIterations, "iterations_completed", iteration)
	}

	a.logger.Debug("a2a agent loop completed", "iterations", iteration, "final_choices", len(currentResponse.Choices))

	*response = currentResponse

	return nil
}

// ExecuteTools executes A2A tools with the provided context and tool calls
func (a *agentImpl) ExecuteTools(ctx context.Context, toolCalls []providers.ChatCompletionMessageToolCall) ([]providers.Message, error) {
	var results []providers.Message

	for _, toolCall := range toolCalls {
		if toolCall.Function.Name == "query_a2a_agent_card" {
			result, err := a.handleAgentQueryTool(ctx, toolCall)
			if err != nil {
				a.logger.Error("failed to handle agent query tool", err, "tool_call", toolCall.ID)
				results = append(results, providers.Message{
					Role:       providers.MessageRoleTool,
					Content:    fmt.Sprintf("Error: %v", err),
					ToolCallId: &toolCall.ID,
				})
			} else {
				results = append(results, result)
			}
			continue
		}

		var args map[string]interface{}
		if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &args); err != nil {
			a.logger.Error("failed to parse a2a tool arguments", err, "args", toolCall.Function.Arguments, "tool_name", toolCall.Function.Name)
			results = append(results, providers.Message{
				Role:       providers.MessageRoleTool,
				Content:    fmt.Sprintf("Error: Failed to parse arguments: %v", err),
				ToolCallId: &toolCall.ID,
			})
			continue
		}

		var agentURL string
		if agentURLVal, ok := args["agentURL"].(string); ok && agentURLVal != "" {
			agentURL = agentURLVal
		} else if mappedAgent, ok := a.toolToAgentMap[toolCall.Function.Name]; ok {
			agentURL = mappedAgent
			a.logger.Debug("using mapped agent for tool", "tool_name", toolCall.Function.Name, "agent_url", agentURL)
		} else {
			agents := a.a2aClient.GetAgents()
			if len(agents) > 0 {
				agentURL = agents[0]
				a.logger.Debug("falling back to first available agent", "tool_name", toolCall.Function.Name, "agent_url", agentURL)
			} else {
				a.logger.Error("no a2a agents available for tool execution", nil, "tool", toolCall.Function.Name)
				results = append(results, providers.Message{
					Role:       providers.MessageRoleTool,
					Content:    "Error: No A2A agents available",
					ToolCallId: &toolCall.ID,
				})
				continue
			}
		}

		delete(args, "agentURL")

		var requestText string
		if req, ok := args["request"].(string); ok {
			requestText = req
		}

		requestID := fmt.Sprintf("req_%s", toolCall.ID)
		messageID := fmt.Sprintf("msg_%s", toolCall.ID)

		sendRequest := &SendMessageRequest{
			ID:      requestID,
			JSONRPC: "2.0",
			Method:  "message/send",
			Params: MessageSendParams{
				Message: Message{
					Role:      "user",
					Parts:     []Part{},
					MessageID: messageID,
				},
				Configuration: &MessageSendConfiguration{
					Blocking: boolPtr(true),
				},
				Metadata: map[string]interface{}{
					"skill":     toolCall.Function.Name,
					"arguments": args,
				},
			},
		}

		if requestText != "" {
			sendRequest.Params.Message.Parts = append(sendRequest.Params.Message.Parts, TextPart{
				Kind: "text",
				Text: requestText,
			})
		}

		a.logger.Info("executing a2a tool call", "tool_call", fmt.Sprintf("id=%s name=%s args=%v agent=%s", toolCall.ID, toolCall.Function.Name, args, agentURL))

		if requestText != "" {
			result, err := a.sendMessageWithTextPart(ctx, sendRequest, agentURL, requestText)
			if err != nil {
				a.logger.Error("failed to execute a2a tool call", err, "tool", toolCall.Function.Name, "agent", agentURL)
				results = append(results, providers.Message{
					Role:       providers.MessageRoleTool,
					Content:    fmt.Sprintf("Error: %v", err),
					ToolCallId: &toolCall.ID,
				})
				continue
			}

			var resultStr string
			if result != nil && result.Result != nil {
				if output, ok := result.Result.(string); ok {
					resultStr = output
				} else {
					outputJSON, _ := json.Marshal(result.Result)
					resultStr = string(outputJSON)
				}
			} else {
				resultStr = "No output received from agent"
			}

			results = append(results, providers.Message{
				Role:       providers.MessageRoleTool,
				Content:    resultStr,
				ToolCallId: &toolCall.ID,
			})
		} else {
			result, err := a.a2aClient.SendMessage(ctx, sendRequest, agentURL)
			if err != nil {
				a.logger.Error("failed to execute a2a tool call", err, "tool", toolCall.Function.Name, "agent", agentURL)
				results = append(results, providers.Message{
					Role:       providers.MessageRoleTool,
					Content:    fmt.Sprintf("Error: %v", err),
					ToolCallId: &toolCall.ID,
				})
				continue
			}

			var resultStr string
			if result != nil && result.Result != nil {
				if output, ok := result.Result.(string); ok {
					resultStr = output
				} else {
					outputJSON, _ := json.Marshal(result.Result)
					resultStr = string(outputJSON)
				}
			} else {
				resultStr = "No output received from agent"
			}

			results = append(results, providers.Message{
				Role:       providers.MessageRoleTool,
				Content:    resultStr,
				ToolCallId: &toolCall.ID,
			})
		}
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
	for i := 0; len(toolCallsMap) > 0 && i < len(toolCallsMap); i++ {
		if toolCall, exists := toolCallsMap[i]; exists {
			a.logger.Debug("final parsed a2a tool call", "tool_call", fmt.Sprintf("id=%s name=%s args=%s", toolCall.ID, toolCall.Function.Name, toolCall.Function.Arguments))
			toolCalls = append(toolCalls, *toolCall)
		}
	}

	a.logger.Debug("total parsed a2a tool calls", "count", len(toolCalls))
	return toolCalls, nil
}

// handleAgentQueryTool handles the special query_a2a_agent_card tool call
func (a *agentImpl) handleAgentQueryTool(ctx context.Context, toolCall providers.ChatCompletionMessageToolCall) (providers.Message, error) {
	var args map[string]interface{}
	if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &args); err != nil {
		return providers.Message{}, fmt.Errorf("failed to parse arguments: %w", err)
	}

	agentURL, ok := args["agent_url"].(string)
	if !ok || agentURL == "" {
		return providers.Message{}, fmt.Errorf("missing or invalid agent_url parameter")
	}

	agentCard, err := a.a2aClient.GetAgentCard(ctx, agentURL)
	if err != nil {
		return providers.Message{}, fmt.Errorf("failed to query agent card: %w", err)
	}

	for _, skill := range agentCard.Skills {
		skillTool := a.convertSkillToTool(skill, agentURL)

		exists := false
		for _, existingTool := range a.discoveredTools {
			if existingTool.Function.Name == skillTool.Function.Name {
				exists = true
				break
			}
		}

		if !exists {
			a.discoveredTools = append(a.discoveredTools, skillTool)
			a.toolToAgentMap[skill.ID] = agentURL
			a.logger.Debug("added discovered skill as tool", "skill_name", skill.Name, "skill_id", skill.ID, "agent_url", agentURL)
		}
	}

	a.logger.Info("discovered and added agent skills as tools", "agent_url", agentURL, "agent_name", agentCard.Name, "skills_count", len(agentCard.Skills), "total_discovered_tools", len(a.discoveredTools))

	agentCardBytes, err := json.Marshal(agentCard)
	if err != nil {
		return providers.Message{}, fmt.Errorf("failed to format agent card: %w", err)
	}

	a.logger.Info("successfully queried agent card", "agent_url", agentURL, "agent_name", agentCard.Name)

	return providers.Message{
		Role:       providers.MessageRoleTool,
		Content:    string(agentCardBytes),
		ToolCallId: &toolCall.ID,
	}, nil
}

// convertSkillToTool converts an AgentSkill to a ChatCompletionTool
func (a *agentImpl) convertSkillToTool(skill AgentSkill, agentURL string) providers.ChatCompletionTool {
	description := skill.Description
	if len(skill.Examples) > 0 {
		description += "\n\nExamples:\n"
		for _, example := range skill.Examples {
			description += "- " + example + "\n"
		}
	}

	parameters := &providers.FunctionParameters{
		"type": "object",
		"properties": map[string]interface{}{
			"request": map[string]interface{}{
				"type":        "string",
				"description": "The request or instruction for the " + skill.Name + " skill",
			},
		},
		"required": []string{"request"},
	}

	if props, ok := (*parameters)["properties"].(map[string]interface{}); ok {
		props["agentURL"] = map[string]interface{}{
			"type":        "string",
			"description": "The A2A agent URL to use for this skill (auto-filled)",
			"default":     agentURL,
		}
	}

	return providers.ChatCompletionTool{
		Type: providers.ChatCompletionToolTypeFunction,
		Function: providers.FunctionObject{
			Name:        skill.ID,
			Description: &description,
			Parameters:  parameters,
		},
	}
}

// sendMessageWithTextPart sends an A2A message with a text part, working around the empty Part struct
func (a *agentImpl) sendMessageWithTextPart(ctx context.Context, baseRequest *SendMessageRequest, agentURL string, text string) (*SendMessageSuccessResponse, error) {
	customRequest := map[string]interface{}{
		"id":      baseRequest.ID,
		"jsonrpc": baseRequest.JSONRPC,
		"method":  baseRequest.Method,
		"params": map[string]interface{}{
			"message": map[string]interface{}{
				"role":      baseRequest.Params.Message.Role,
				"messageId": baseRequest.Params.Message.MessageID,
				"parts": []map[string]interface{}{
					{
						"kind": "text",
						"text": text,
					},
				},
			},
			"configuration": map[string]interface{}{
				"blocking": baseRequest.Params.Configuration.Blocking,
			},
			"metadata": baseRequest.Params.Metadata,
		},
	}

	requestBody, err := json.Marshal(customRequest)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal custom request: %w", err)
	}

	return a.makeCustomA2ARequest(ctx, requestBody, agentURL)
}

// makeCustomA2ARequest makes a custom A2A request with pre-marshaled JSON
func (a *agentImpl) makeCustomA2ARequest(ctx context.Context, requestBody []byte, agentURL string) (*SendMessageSuccessResponse, error) {
	rpcURL, err := url.JoinPath(agentURL, "a2a")
	if err != nil {
		return nil, fmt.Errorf("failed to build JSON-RPC URL: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", rpcURL, bytes.NewReader(requestBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	var response SendMessageSuccessResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &response, nil
}

func boolPtr(b bool) *bool {
	return &b
}
