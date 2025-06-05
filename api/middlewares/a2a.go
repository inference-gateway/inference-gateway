package middlewares

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/inference-gateway/inference-gateway/a2a"
	config "github.com/inference-gateway/inference-gateway/config"
	"github.com/inference-gateway/inference-gateway/logger"
	"github.com/inference-gateway/inference-gateway/providers"
)

const (
	// A2AInternalHeader marks internal A2A requests to prevent middleware loops
	A2AInternalHeader = "X-A2A-Internal"
)

// a2aContextKey is a custom type for context keys to avoid collisions
type a2aContextKey string

const (
	// a2aInternalKey is the context key for marking internal A2A requests
	a2aInternalKey a2aContextKey = A2AInternalHeader
)

// A2AProviderModelResult contains the result of provider and model determination
type A2AProviderModelResult struct {
	Provider      providers.IProvider
	ProviderModel string
	ProviderID    *providers.Provider
}

// A2AMiddleware defines the interface for A2A middleware
type A2AMiddleware interface {
	Middleware() gin.HandlerFunc
}

// A2AMiddlewareImpl implements the A2A middleware
type A2AMiddlewareImpl struct {
	registry               providers.ProviderRegistry
	config                 config.Config
	a2aClient              a2a.A2AClientInterface
	a2aAgent               a2a.Agent
	logger                 logger.Logger
	inferenceGatewayClient providers.Client
}

// NoopA2AMiddlewareImpl is a no-operation implementation of A2AMiddleware
type NoopA2AMiddlewareImpl struct{}

// NewA2AMiddleware creates a new A2A middleware instance
func NewA2AMiddleware(registry providers.ProviderRegistry, a2aClient a2a.A2AClientInterface, a2aAgent a2a.Agent, log logger.Logger, inferenceGatewayClient providers.Client, cfg config.Config) (A2AMiddleware, error) {
	if !cfg.A2A.Enable {
		return &NoopA2AMiddlewareImpl{}, nil
	}

	return &A2AMiddlewareImpl{
		a2aClient:              a2aClient,
		a2aAgent:               a2aAgent,
		config:                 cfg,
		logger:                 log,
		registry:               registry,
		inferenceGatewayClient: inferenceGatewayClient,
	}, nil
}

// Middleware returns a no-op handler for the noop implementation
func (n *NoopA2AMiddlewareImpl) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()
	}
}

// Middleware returns the A2A middleware handler
func (m *A2AMiddlewareImpl) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.GetHeader(A2AInternalHeader) != "" {
			m.logger.Debug("internal a2a call, skipping middleware")
			c.Next()
			return
		}

		if c.Request.URL.Path != ChatCompletionsPath {
			c.Next()
			return
		}

		m.logger.Debug("a2a middleware invoked", "path", c.Request.URL.Path)
		var originalRequestBody providers.CreateChatCompletionRequest
		if err := c.ShouldBindJSON(&originalRequestBody); err != nil {
			m.logger.Error("failed to parse request body", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
			c.Abort()
			return
		}

		if !m.a2aClient.IsInitialized() {
			c.Next()
			return
		}

		agentQueryTool := m.createAgentQueryTool()
		m.addToolToRequest(&originalRequestBody, agentQueryTool)
		m.logger.Debug("added a2a agent query tool to request")

		agentSkillTools := m.createAgentSkillTools()
		for _, tool := range agentSkillTools {
			m.addToolToRequest(&originalRequestBody, tool)
		}
		m.logger.Debug("added a2a agent skill tools to request", "count", len(agentSkillTools))

		c.Set(string(a2aInternalKey), &originalRequestBody)

		result, err := m.getProviderAndModel(c, originalRequestBody.Model)
		if err != nil {
			if result == nil || result.ProviderID == nil {
				m.logger.Error("failed to determine provider", err, "model", originalRequestBody.Model)
				c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid model"})
				c.Abort()
				return
			}

			if result.Provider == nil {
				m.logger.Error("failed to get provider", err, "provider", *result.ProviderID)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Provider not available"})
				c.Abort()
				return
			}
		}

		bodyBytes, err := json.Marshal(&originalRequestBody)
		if err != nil {
			m.logger.Error("failed to marshal modified request", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
			c.Abort()
			return
		}

		c.Request.Body = io.NopCloser(bytes.NewReader(bodyBytes))
		c.Request.ContentLength = int64(len(bodyBytes))

		customWriter := &customResponseWriter{
			ResponseWriter: c.Writer,
			body:           &bytes.Buffer{},
			statusCode:     http.StatusOK,
			writeToClient:  false,
		}
		c.Writer = customWriter

		c.Next()

		var response providers.CreateChatCompletionResponse
		if err := json.Unmarshal(customWriter.body.Bytes(), &response); err != nil {
			m.logger.Error("failed to parse chat completion response", err)
			m.writeErrorResponse(c, customWriter, "Failed to parse response", http.StatusInternalServerError)
			return
		}

		if len(response.Choices) > 0 && response.Choices[0].Message.ToolCalls != nil {
			if err := m.handleA2AToolCalls(c, &response, &originalRequestBody, result); err != nil {
				m.logger.Error("failed to handle a2a tool calls", err)
				m.writeErrorResponse(c, customWriter, "Failed to execute A2A tools", http.StatusInternalServerError)
				return
			}
		}

		m.writeResponse(c, customWriter, response)
	}
}

// getProviderAndModel determines the provider and model from the request model string or query parameter
func (m *A2AMiddlewareImpl) getProviderAndModel(c *gin.Context, model string) (*A2AProviderModelResult, error) {
	if providerID := providers.Provider(c.Query("provider")); providerID != "" {
		provider, err := m.registry.BuildProvider(providerID, m.inferenceGatewayClient)
		if err != nil {
			return &A2AProviderModelResult{ProviderID: &providerID}, fmt.Errorf("failed to build provider: %w", err)
		}

		return &A2AProviderModelResult{
			Provider:      provider,
			ProviderModel: model,
			ProviderID:    &providerID,
		}, nil
	}

	providerPtr, providerModel := providers.DetermineProviderAndModelName(model)
	if providerPtr == nil {
		return &A2AProviderModelResult{ProviderID: nil}, fmt.Errorf("unable to determine provider for model: %s. Please specify a provider using the ?provider= query parameter or use the provider/model format", model)
	}

	provider, err := m.registry.BuildProvider(*providerPtr, m.inferenceGatewayClient)
	if err != nil {
		return &A2AProviderModelResult{ProviderID: providerPtr}, fmt.Errorf("failed to build provider: %w", err)
	}

	return &A2AProviderModelResult{
		Provider:      provider,
		ProviderModel: providerModel,
		ProviderID:    providerPtr,
	}, nil
}

// handleA2AToolCalls executes A2A tool calls and updates the response
func (m *A2AMiddlewareImpl) handleA2AToolCalls(c *gin.Context, response *providers.CreateChatCompletionResponse, originalRequest *providers.CreateChatCompletionRequest, result *A2AProviderModelResult) error {
	toolCalls := response.Choices[0].Message.ToolCalls

	for _, toolCall := range *toolCalls {
		// Handle the agent query tool call
		if toolCall.Function.Name == "query_a2a_agent_card" {
			agentCardResult, err := m.handleAgentCardQuery(c, toolCall)
			if err != nil {
				m.logger.Error("failed to query agent card", err, "tool", toolCall.Function.Name)
				continue
			}

			m.logger.Debug("agent card queried successfully", "tool", toolCall.Function.Name, "result", agentCardResult)
			continue
		}

		// Handle actual A2A task execution for agents
		agentURL, err := m.findAgentForSkill(toolCall.Function.Name)
		if err != nil {
			m.logger.Warn("tool not found in a2a agents", "tool", toolCall.Function.Name)
			continue
		}

		// Execute the A2A task
		taskResult, err := m.executeA2ATask(c, agentURL, toolCall)
		if err != nil {
			m.logger.Error("failed to execute a2a task", err, "tool", toolCall.Function.Name, "agent", agentURL)
			// Note: Tool calls don't have a Result field, this should be handled differently
			// For now, we'll log the error and continue
			continue
		}

		// Update the response with the task result by creating a new tool message
		// This follows the OpenAI pattern where tool results are returned as separate messages
		m.logger.Debug("a2a tool executed successfully", "tool", toolCall.Function.Name, "agent", agentURL, "result", taskResult)
	}

	return nil
}

// findAgentForSkill finds the agent URL that provides the specified skill
func (m *A2AMiddlewareImpl) findAgentForSkill(skillID string) (string, error) {
	for _, agentURL := range m.a2aClient.GetAgents() {
		skills, err := m.a2aClient.GetAgentSkills(agentURL)
		if err != nil {
			continue
		}

		for _, skill := range skills {
			if skill.ID == skillID {
				return agentURL, nil
			}
		}
	}

	return "", fmt.Errorf("skill %s not found in any agent", skillID)
}

// executeA2ATask executes an A2A task asynchronously and returns the result via streaming
func (m *A2AMiddlewareImpl) executeA2ATask(ctx *gin.Context, agentURL string, toolCall providers.ChatCompletionMessageToolCall) (string, error) {
	var req providers.CreateChatCompletionRequest
	if err := ctx.ShouldBindJSON(&req); err == nil && req.Stream != nil && *req.Stream {
		return m.executeA2ATaskAsync(ctx, agentURL, toolCall)
	}

	return m.executeA2ATaskSync(ctx, agentURL, toolCall)
}

// executeA2ATaskSync executes an A2A task synchronously (original behavior)
func (m *A2AMiddlewareImpl) executeA2ATaskSync(ctx *gin.Context, agentURL string, toolCall providers.ChatCompletionMessageToolCall) (string, error) {
	// Create the send message request using A2A's message/send JSON-RPC method
	sendRequest := &a2a.SendMessageRequest{
		ID:      generateRequestID(),
		JSONRPC: "2.0",
		Method:  "message/send",
		Params: a2a.MessageSendParams{
			Message: a2a.Message{
				Role:  "user",
				Parts: []a2a.Part{
					// Since Part is an empty struct, we need to handle this differently
					// The actual message will be processed by the A2A agent
				},
				Messageid: generateMessageID(),
			},
			Configuration: a2a.MessageSendConfiguration{
				Blocking: true,
			},
			Metadata: map[string]interface{}{
				"skill":     toolCall.Function.Name,
				"arguments": toolCall.Function.Arguments,
			},
		},
	}

	sendResponse, err := m.a2aClient.SendMessage(ctx, sendRequest, agentURL)
	if err != nil {
		return "", fmt.Errorf("failed to send message: %w", err)
	}

	_ = sendResponse
	return fmt.Sprintf("A2A task '%s' completed successfully", toolCall.Function.Name), nil
}

// executeA2ATaskAsync executes an A2A task asynchronously with polling and SSE streaming
func (m *A2AMiddlewareImpl) executeA2ATaskAsync(ctx *gin.Context, agentURL string, toolCall providers.ChatCompletionMessageToolCall) (string, error) {
	sendRequest := &a2a.SendMessageRequest{
		ID:      generateRequestID(),
		JSONRPC: "2.0",
		Method:  "message/send",
		Params: a2a.MessageSendParams{
			Message: a2a.Message{
				Role:  "user",
				Parts: []a2a.Part{
					// Since Part is an empty struct, we need to handle this differently
					// The actual message will be processed by the A2A agent
				},
				Messageid: generateMessageID(),
			},
			Configuration: a2a.MessageSendConfiguration{
				Blocking: false, // Enable async execution
			},
			Metadata: map[string]interface{}{
				"skill":     toolCall.Function.Name,
				"arguments": toolCall.Function.Arguments,
			},
		},
	}

	sendResponse, err := m.a2aClient.SendMessage(ctx, sendRequest, agentURL)
	if err != nil {
		return "", fmt.Errorf("failed to send message: %w", err)
	}

	taskID, err := m.extractTaskID(sendResponse)
	if err != nil {
		return "", fmt.Errorf("failed to extract task ID: %w", err)
	}

	if streamCh, exists := ctx.Get("middlewareStreamCh"); exists {
		if ch, ok := streamCh.(chan []byte); ok {
			go m.pollTaskCompletionAsync(ctx, agentURL, taskID, toolCall.Function.Name, ch)
			return fmt.Sprintf("A2A task '%s' submitted for async execution", toolCall.Function.Name), nil
		}
	}

	return m.pollTaskCompletionSync(ctx, agentURL, taskID, toolCall.Function.Name)
}

// extractTaskID extracts the task ID from the A2A send message response
func (m *A2AMiddlewareImpl) extractTaskID(response *a2a.SendMessageSuccessResponse) (string, error) {
	if response == nil {
		return "", fmt.Errorf("response is nil")
	}

	if response.Result == nil {
		return "", fmt.Errorf("response result is nil")
	}

	resultBytes, err := json.Marshal(response.Result)
	if err != nil {
		return "", fmt.Errorf("failed to marshal response result: %w", err)
	}

	var task a2a.Task
	if err := json.Unmarshal(resultBytes, &task); err == nil && task.ID != "" {
		return task.ID, nil
	}

	var message a2a.Message
	if err := json.Unmarshal(resultBytes, &message); err == nil {
		if message.Taskid != "" {
			return message.Taskid, nil
		}
		if message.Messageid != "" {
			return message.Messageid, nil
		}
	}

	var genericResult map[string]interface{}
	if err := json.Unmarshal(resultBytes, &genericResult); err == nil {
		for _, idField := range []string{"id", "taskId", "messageId"} {
			if id, exists := genericResult[idField]; exists {
				if idStr, ok := id.(string); ok && idStr != "" {
					return idStr, nil
				}
			}
		}
	}

	return "", fmt.Errorf("unable to extract task ID from response result")
}

// pollTaskCompletionAsync polls for task completion asynchronously and streams results via SSE
func (m *A2AMiddlewareImpl) pollTaskCompletionAsync(ctx *gin.Context, agentURL, taskID, skillName string, streamCh chan []byte) {
	defer func() {
		if r := recover(); r != nil {
			m.logger.Error("panic in async task polling", fmt.Errorf("panic: %v", r), "taskID", taskID, "skill", skillName)
		}
	}()

	m.logger.Debug("starting async polling for A2A task", "taskID", taskID, "skill", skillName, "agentURL", agentURL)

	pollCtx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	m.sendSSEMessage(streamCh, map[string]interface{}{
		"type":    "a2a_task_started",
		"taskId":  taskID,
		"skill":   skillName,
		"agent":   agentURL,
		"message": fmt.Sprintf("A2A task '%s' started", skillName),
	})

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	maxAttempts := 150
	attempts := 0

	for {
		select {
		case <-pollCtx.Done():
			m.logger.Error("polling timeout for A2A task", pollCtx.Err(), "taskID", taskID, "skill", skillName)
			m.sendSSEMessage(streamCh, map[string]interface{}{
				"type":    "a2a_task_timeout",
				"taskId":  taskID,
				"skill":   skillName,
				"message": fmt.Sprintf("A2A task '%s' timed out", skillName),
			})
			return

		case <-ticker.C:
			attempts++
			if attempts > maxAttempts {
				m.logger.Error("max polling attempts reached for A2A task", nil, "taskID", taskID, "skill", skillName, "attempts", attempts)
				m.sendSSEMessage(streamCh, map[string]interface{}{
					"type":    "a2a_task_failed",
					"taskId":  taskID,
					"skill":   skillName,
					"message": fmt.Sprintf("A2A task '%s' exceeded maximum polling attempts", skillName),
				})
				return
			}

			taskStatus, completed, err := m.pollTaskStatus(pollCtx, agentURL, taskID)
			if err != nil {
				m.logger.Error("failed to poll task status", err, "taskID", taskID, "skill", skillName, "attempt", attempts)
				continue
			}

			m.sendSSEMessage(streamCh, map[string]interface{}{
				"type":    "a2a_task_progress",
				"taskId":  taskID,
				"skill":   skillName,
				"status":  taskStatus,
				"attempt": attempts,
				"message": fmt.Sprintf("A2A task '%s' status: %s", skillName, taskStatus),
			})

			if completed {
				result, err := m.getTaskResult(pollCtx, agentURL, taskID)
				if err != nil {
					m.logger.Error("failed to get task result", err, "taskID", taskID, "skill", skillName)
					m.sendSSEMessage(streamCh, map[string]interface{}{
						"type":    "a2a_task_failed",
						"taskId":  taskID,
						"skill":   skillName,
						"message": fmt.Sprintf("A2A task '%s' failed to get result", skillName),
					})
					return
				}

				m.sendSSEMessage(streamCh, map[string]interface{}{
					"type":    "a2a_task_completed",
					"taskId":  taskID,
					"skill":   skillName,
					"result":  result,
					"message": fmt.Sprintf("A2A task '%s' completed successfully", skillName),
				})

				m.logger.Debug("A2A task completed successfully", "taskID", taskID, "skill", skillName, "attempts", attempts)
				return
			}
		}
	}
}

// pollTaskCompletionSync polls for task completion synchronously (fallback for non-streaming)
func (m *A2AMiddlewareImpl) pollTaskCompletionSync(ctx *gin.Context, agentURL, taskID, skillName string) (string, error) {
	m.logger.Debug("starting sync polling for A2A task", "taskID", taskID, "skill", skillName, "agentURL", agentURL)

	pollCtx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	maxAttempts := 120
	attempts := 0

	for {
		select {
		case <-pollCtx.Done():
			return "", fmt.Errorf("polling timeout for A2A task '%s'", skillName)

		case <-ticker.C:
			attempts++
			if attempts > maxAttempts {
				return "", fmt.Errorf("max polling attempts reached for A2A task '%s'", skillName)
			}

			_, completed, err := m.pollTaskStatus(pollCtx, agentURL, taskID)
			if err != nil {
				m.logger.Error("failed to poll task status", err, "taskID", taskID, "skill", skillName, "attempt", attempts)
				continue
			}

			if completed {
				result, err := m.getTaskResult(pollCtx, agentURL, taskID)
				if err != nil {
					return "", fmt.Errorf("failed to get task result for '%s': %w", skillName, err)
				}

				m.logger.Debug("A2A task completed successfully", "taskID", taskID, "skill", skillName, "attempts", attempts)
				return result, nil
			}
		}
	}
}

// pollTaskStatus polls the task status using A2A's tasks/get method
func (m *A2AMiddlewareImpl) pollTaskStatus(ctx context.Context, agentURL, taskID string) (string, bool, error) {
	getTaskRequest := &a2a.GetTaskRequest{
		ID:      generateRequestID(),
		JSONRPC: "2.0",
		Method:  "tasks/get",
		Params: a2a.TaskQueryParams{
			ID: taskID,
		},
	}

	response, err := m.a2aClient.GetTask(ctx, getTaskRequest, agentURL)
	if err != nil {
		return "", false, fmt.Errorf("failed to get task: %w", err)
	}

	if response == nil || response.Result.Status.State == "" {
		return "unknown", false, nil
	}

	taskState := string(response.Result.Status.State)

	switch a2a.TaskState(response.Result.Status.State) {
	case a2a.TaskStateCompleted:
		return taskState, true, nil
	case a2a.TaskStateFailed, a2a.TaskStateCanceled, a2a.TaskStateRejected:
		return taskState, true, fmt.Errorf("task failed with state: %s", taskState)
	default:
		return taskState, false, nil
	}
}

// getTaskResult retrieves the final result from a completed task
func (m *A2AMiddlewareImpl) getTaskResult(ctx context.Context, agentURL, taskID string) (string, error) {
	getTaskRequest := &a2a.GetTaskRequest{
		ID:      generateRequestID(),
		JSONRPC: "2.0",
		Method:  "tasks/get",
		Params: a2a.TaskQueryParams{
			ID: taskID,
		},
	}

	response, err := m.a2aClient.GetTask(ctx, getTaskRequest, agentURL)
	if err != nil {
		return "", fmt.Errorf("failed to get task result: %w", err)
	}

	if response == nil {
		return "", fmt.Errorf("empty response from task")
	}

	if len(response.Result.History) > 0 {
		for i := len(response.Result.History) - 1; i >= 0; i-- {
			message := response.Result.History[i]
			if message.Role == "assistant" || message.Role == "agent" {
				content := m.extractMessageContentFromJSON(message)
				if content != "" {
					return content, nil
				}
			}
		}
	}

	if response.Result.Status.Message.Role == "assistant" || response.Result.Status.Message.Role == "agent" {
		content := m.extractMessageContentFromJSON(response.Result.Status.Message)
		if content != "" {
			return content, nil
		}
	}

	return fmt.Sprintf("A2A task completed with status: %s", response.Result.Status.State), nil
}

// extractMessageContentFromJSON extracts text content from A2A message by working with raw JSON
func (m *A2AMiddlewareImpl) extractMessageContentFromJSON(message a2a.Message) string {
	messageBytes, err := json.Marshal(message)
	if err != nil {
		m.logger.Debug("failed to marshal message for content extraction", "error", err)
		return ""
	}

	var messageMap map[string]interface{}
	if err := json.Unmarshal(messageBytes, &messageMap); err != nil {
		m.logger.Debug("failed to unmarshal message for content extraction", "error", err)
		return ""
	}

	parts, ok := messageMap["parts"].([]interface{})
	if !ok || len(parts) == 0 {
		return ""
	}

	var contentParts []string
	for _, part := range parts {
		if partMap, ok := part.(map[string]interface{}); ok {
			if kind, exists := partMap["kind"]; exists {
				switch kind {
				case "text":
					if text, hasText := partMap["text"].(string); hasText && text != "" {
						contentParts = append(contentParts, text)
					}
				case "data":
					if data, hasData := partMap["data"]; hasData {
						if dataStr, ok := data.(string); ok {
							contentParts = append(contentParts, dataStr)
						} else {
							if dataBytes, err := json.Marshal(data); err == nil {
								contentParts = append(contentParts, string(dataBytes))
							}
						}
					}
				case "file":
					contentParts = append(contentParts, "[File content]")
				}
			}
		}
	}

	if len(contentParts) > 0 {
		return strings.Join(contentParts, " ")
	}

	return ""
}

// sendSSEMessage sends a Server-Sent Event message through the streaming channel
func (m *A2AMiddlewareImpl) sendSSEMessage(streamCh chan []byte, data map[string]interface{}) {
	jsonData, err := json.Marshal(data)
	if err != nil {
		m.logger.Error("failed to marshal SSE message", err, "data", data)
		return
	}

	sseMessage := fmt.Sprintf("data: %s\n\n", string(jsonData))

	select {
	case streamCh <- []byte(sseMessage):
	default:
		m.logger.Debug("failed to send SSE message: channel unavailable", "data", data)
	}
}

// createAgentQueryTool creates a tool that allows LLM to query agent cards
func (m *A2AMiddlewareImpl) createAgentQueryTool() providers.ChatCompletionTool {
	return providers.ChatCompletionTool{
		Type: providers.ChatCompletionToolTypeFunction,
		Function: providers.FunctionObject{
			Name:        "query_a2a_agent_card",
			Description: &[]string{"Query an A2A agent's card to understand its capabilities and determine if it's suitable for a task"}[0],
			Parameters: &providers.FunctionParameters{
				"type": "object",
				"properties": map[string]interface{}{
					"agent_url": map[string]interface{}{
						"type":        "string",
						"description": "The URL of the A2A agent to query",
					},
				},
				"required": []string{"agent_url"},
			},
		},
	}
}

// createAgentSkillTools creates chat completion tools from all available A2A agent skills
func (m *A2AMiddlewareImpl) createAgentSkillTools() []providers.ChatCompletionTool {
	var tools []providers.ChatCompletionTool

	agents := m.a2aClient.GetAgents()

	for _, agentURL := range agents {
		skills, err := m.a2aClient.GetAgentSkills(agentURL)
		if err != nil {
			m.logger.Warn("failed to get agent skills", "agent", agentURL, "error", err)
			continue
		}

		for _, skill := range skills {
			tool := providers.ChatCompletionTool{
				Type: providers.ChatCompletionToolTypeFunction,
				Function: providers.FunctionObject{
					Name:        skill.ID,
					Description: &skill.Description,
					Parameters: &providers.FunctionParameters{
						"type": "object",
						"properties": map[string]interface{}{
							"arguments": map[string]interface{}{
								"type":        "object",
								"description": "Arguments for the " + skill.Name + " skill",
							},
						},
						"required": []string{},
					},
				},
			}
			tools = append(tools, tool)
		}
	}

	return tools
}

// addToolToRequest adds a single tool to the request
func (m *A2AMiddlewareImpl) addToolToRequest(request *providers.CreateChatCompletionRequest, tool providers.ChatCompletionTool) {
	if request.Tools == nil {
		request.Tools = &[]providers.ChatCompletionTool{tool}
	} else {
		tools := append(*request.Tools, tool)
		request.Tools = &tools
	}
}

// writeErrorResponse writes an error response to the client
func (m *A2AMiddlewareImpl) writeErrorResponse(c *gin.Context, customWriter *customResponseWriter, message string, statusCode int) {
	errorResponse := ErrorResponse{Error: message}
	customWriter.statusCode = statusCode
	m.writeResponse(c, customWriter, errorResponse)
}

// writeResponse writes the response to the client
func (m *A2AMiddlewareImpl) writeResponse(c *gin.Context, customWriter *customResponseWriter, response interface{}) {
	customWriter.writeToClient = true
	customWriter.WriteHeader(customWriter.statusCode)

	responseBytes, err := json.Marshal(response)
	if err != nil {
		m.logger.Error("failed to marshal final response", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		return
	}

	if _, err := customWriter.Write(responseBytes); err != nil {
		m.logger.Error("failed to write response", err)
	}
}

// generateRequestID generates a unique request ID
func generateRequestID() interface{} {
	return fmt.Sprintf("req_%d", time.Now().UnixNano())
}

// generateMessageID generates a unique message ID
func generateMessageID() string {
	return uuid.New().String()
}

// handleAgentCardQuery handles the query_a2a_agent_card tool call
func (m *A2AMiddlewareImpl) handleAgentCardQuery(c *gin.Context, toolCall providers.ChatCompletionMessageToolCall) (string, error) {
	var args map[string]interface{}
	if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &args); err != nil {
		return "", fmt.Errorf("failed to parse tool call arguments: %w", err)
	}

	agentURL, ok := args["agent_url"].(string)
	if !ok || agentURL == "" {
		agents := m.a2aClient.GetAgents()
		if len(agents) == 0 {
			return "No A2A agents are currently available.", nil
		}

		result := "Available A2A agents:\n"
		for _, url := range agents {
			result += fmt.Sprintf("- %s\n", url)
		}
		result += "\nUse the agent URL to query specific agent capabilities."
		return result, nil
	}

	agentCard, err := m.a2aClient.GetAgentCard(c, agentURL)
	if err != nil {
		return "", fmt.Errorf("failed to get agent card for %s: %w", agentURL, err)
	}

	result := fmt.Sprintf("Agent Card for %s:\n", agentURL)
	result += fmt.Sprintf("Name: %s\n", agentCard.Name)
	result += fmt.Sprintf("Description: %s\n", agentCard.Description)
	result += fmt.Sprintf("Version: %s\n", agentCard.Version)

	if len(agentCard.Skills) > 0 {
		result += "\nAvailable Skills:\n"
		for _, skill := range agentCard.Skills {
			result += fmt.Sprintf("- %s: %s\n", skill.ID, skill.Description)
			if len(skill.Inputmodes) > 0 || len(skill.Outputmodes) > 0 {
				result += fmt.Sprintf("  Input modes: %s, Output modes: %s\n",
					strings.Join(skill.Inputmodes, ", "),
					strings.Join(skill.Outputmodes, ", "))
			}
		}
	}

	capabilities := m.a2aClient.GetAgentCapabilities()[agentURL]
	result += "\nCapabilities:\n"
	result += fmt.Sprintf("- Push notifications: %v\n", capabilities.Pushnotifications)
	result += fmt.Sprintf("- State transition history: %v\n", capabilities.Statetransitionhistory)
	result += fmt.Sprintf("- Streaming: %v\n", capabilities.Streaming)

	return result, nil
}
