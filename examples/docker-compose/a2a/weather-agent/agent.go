package main

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"time"

	gin "github.com/gin-gonic/gin"
	uuid "github.com/google/uuid"
	sdk "github.com/inference-gateway/sdk"
	zap "go.uber.org/zap"

	a2a "github.com/inference-gateway/inference-gateway/a2a"
)

// JRPCErrorCode represents JSON-RPC error codes
type JRPCErrorCode int

const (
	ErrParseError     JRPCErrorCode = -32700
	ErrInvalidRequest JRPCErrorCode = -32600
	ErrMethodNotFound JRPCErrorCode = -32601
	ErrInvalidParams  JRPCErrorCode = -32602
	ErrInternalError  JRPCErrorCode = -32603
	ErrServerError    JRPCErrorCode = -32000
)

// QueuedTask represents a task in the processing queue
type QueuedTask struct {
	Task      *a2a.Task
	Messages  []sdk.Message
	RequestID interface{}
}

// PushNotificationConfig holds push notification configuration
type PushNotificationConfig struct {
	URL   string                 `json:"url"`
	Token string                 `json:"token,omitempty"`
	Auth  map[string]interface{} `json:"authentication,omitempty"`
}

// TaskPushNotificationConfig holds task-specific push notification config
type TaskPushNotificationConfig struct {
	TaskID                 string                  `json:"taskId"`
	PushNotificationConfig *PushNotificationConfig `json:"pushNotificationConfig"`
}

// A2AAgent implements the A2A agent interface
type A2AAgent struct {
	cfg                 Config
	logger              *zap.Logger
	client              sdk.Client
	weatherToolHandler  *WeatherToolHandler
	taskQueue           chan *QueuedTask
	allTasks            map[string]*a2a.Task
	allTasksMu          sync.RWMutex
	pushNotifications   map[string]*PushNotificationConfig
	pushNotificationsMu sync.RWMutex
	tools               []sdk.ChatCompletionTool
}

// NewA2AAgent creates a new A2A agent
func NewA2AAgent(cfg Config, logger *zap.Logger, client sdk.Client, weatherToolHandler *WeatherToolHandler) *A2AAgent {
	tools := []sdk.ChatCompletionTool{
		{
			Type: "function",
			Function: sdk.FunctionObject{
				Name:        "fetch_weather",
				Description: stringPtr("Fetch current weather information for a specified location"),
				Parameters: &sdk.FunctionParameters{
					"type": "object",
					"properties": map[string]interface{}{
						"location": map[string]interface{}{
							"type":        "string",
							"description": "The location to get weather for (e.g., 'New York', 'London', 'Tokyo')",
						},
					},
					"required": []string{"location"},
				},
			},
		},
	}

	return &A2AAgent{
		cfg:                cfg,
		logger:             logger,
		client:             client,
		weatherToolHandler: weatherToolHandler,
		taskQueue:          make(chan *QueuedTask, cfg.QueueConfig.MaxSize),
		allTasks:           make(map[string]*a2a.Task),
		pushNotifications:  make(map[string]*PushNotificationConfig),
		tools:              tools,
	}
}

// SetupRouter configures the HTTP router with A2A endpoints
func (agent *A2AAgent) SetupRouter(oidcAuthenticator OIDCAuthenticator) *gin.Engine {
	r := gin.Default()
	r.Use(oidcAuthenticator.Middleware())

	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "healthy"})
	})

	r.POST("/a2a", func(c *gin.Context) {
		agent.handleA2ARequest(c)
	})

	r.GET("/.well-known/agent.json", func(c *gin.Context) {
		agent.logger.Info("agent info requested")

		info := a2a.AgentCard{
			Name:        agent.cfg.AgentName,
			Description: agent.cfg.AgentDescription,
			URL:         agent.cfg.AgentURL,
			Version:     agent.cfg.AgentVersion,
			Capabilities: a2a.AgentCapabilities{
				Streaming:              &agent.cfg.CapabilitiesConfig.Streaming,
				PushNotifications:      &agent.cfg.CapabilitiesConfig.PushNotifications,
				StateTransitionHistory: &agent.cfg.CapabilitiesConfig.StateTransitionHistory,
			},
			DefaultInputModes:  []string{"text"},
			DefaultOutputModes: []string{"text"},
			Skills: []a2a.AgentSkill{
				{
					ID:          "weather",
					Name:        "weather",
					Description: "Get current weather information for any location",
					InputModes:  []string{"text"},
					OutputModes: []string{"text"},
				},
			},
		}
		c.JSON(http.StatusOK, info)
	})

	return r
}

// StartTaskProcessor starts the background task processor
func (agent *A2AAgent) StartTaskProcessor(ctx context.Context) {
	agent.logger.Info("starting background task processor")

	for {
		select {
		case <-ctx.Done():
			agent.logger.Info("task processor shutting down")
			return
		case queuedTask := <-agent.taskQueue:
			agent.processTaskAsync(queuedTask)
		}
	}
}

// handleA2ARequest handles incoming A2A requests
func (agent *A2AAgent) handleA2ARequest(c *gin.Context) {
	var req a2a.JSONRPCRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		agent.logger.Error("failed to parse json request", zap.Error(err))
		agent.sendError(c, req.ID, int(ErrParseError), "parse error")
		return
	}

	if req.JSONRPC == "" {
		req.JSONRPC = "2.0"
	}

	if req.ID == nil {
		id := interface{}(uuid.New().String())
		req.ID = &id
	}

	agent.logger.Info("received a2a request",
		zap.String("method", req.Method),
		zap.Any("id", req.ID))

	switch req.Method {
	case "message/send":
		agent.handleMessageSend(c, req)
	case "message/stream":
		agent.handleMessageStream(c, req)
	case "tasks/get":
		agent.handleTaskGet(c, req)
	case "tasks/cancel":
		agent.handleTaskCancel(c, req)
	case "tasks/pushNotificationConfig/set":
		agent.handleSetPushNotificationConfig(c, req)
	case "tasks/pushNotificationConfig/get":
		agent.handleGetPushNotificationConfig(c, req)
	default:
		agent.logger.Warn("unknown method requested", zap.String("method", req.Method))
		agent.sendError(c, req.ID, int(ErrMethodNotFound), "method not found")
	}
}

// sendError sends an error response
func (agent *A2AAgent) sendError(c *gin.Context, id interface{}, code int, message string) {
	resp := a2a.JSONRPCErrorResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error: &a2a.JSONRPCError{
			Code:    code,
			Message: message,
		},
	}
	c.JSON(http.StatusOK, resp)
	agent.logger.Error("sending error response", zap.Int("code", code), zap.String("message", message))
}

// handleMessageSend handles message/send requests
func (agent *A2AAgent) handleMessageSend(c *gin.Context, req a2a.JSONRPCRequest) {
	params, messages, err := agent.parseMessageParams(req)
	if err != nil {
		agent.sendError(c, req.ID, int(ErrInvalidParams), "invalid request")
		return
	}

	contextID := params.Message.ContextID
	if contextID == nil {
		newContextID := uuid.New().String()
		contextID = &newContextID
	}

	task := agent.createTask(*contextID, a2a.TaskStateSubmitted, nil)
	agent.logger.Info("task created and queued", zap.String("task_id", task.ID))

	queuedTask := &QueuedTask{
		Task:      task,
		Messages:  messages,
		RequestID: req.ID,
	}

	select {
	case agent.taskQueue <- queuedTask:
		agent.logger.Info("task added to queue", zap.String("task_id", task.ID))
	default:
		agent.logger.Error("task queue is full", zap.String("task_id", task.ID))
		agent.failTaskWithCleanup(task, "Task queue is full, please try again later")
		agent.sendError(c, req.ID, int(ErrServerError), "task queue is full")
		return
	}

	c.JSON(http.StatusOK, a2a.JSONRPCSuccessResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  *task,
	})
}

// handleMessageStream handles message/stream requests
func (agent *A2AAgent) handleMessageStream(c *gin.Context, req a2a.JSONRPCRequest) {
	params, messages, err := agent.parseMessageParams(req)
	if err != nil {
		agent.sendError(c, req.ID, int(ErrInvalidParams), "invalid request")
		return
	}

	contextID := params.Message.ContextID
	if contextID == nil {
		newContextID := uuid.New().String()
		contextID = &newContextID
	}

	task := agent.createTask(*contextID, a2a.TaskStateSubmitted, nil)
	agent.logger.Info("task created for streaming", zap.String("task_id", task.ID))

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")

	initialResponse := a2a.JSONRPCSuccessResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  *task,
	}

	initialData, err := json.Marshal(initialResponse)
	if err != nil {
		agent.logger.Error("failed to marshal initial response", zap.Error(err))
		agent.sendError(c, req.ID, int(ErrServerError), "failed to marshal response")
		return
	}

	c.SSEvent("data", string(initialData))
	c.Writer.Flush()

	go func() {
		agent.processTaskWithStreaming(task, messages, c, *contextID)
	}()

	ticker := time.NewTicker(agent.cfg.StreamingStatusUpdateInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			currentTask, exists := agent.getTask(task.ID)
			if !exists {
				return
			}

			statusUpdate := a2a.TaskStatusUpdateEvent{
				Kind:      "task_status_update",
				ContextID: *contextID,
				Status:    currentTask.Status,
				Final:     currentTask.Status.State == a2a.TaskStateCompleted || currentTask.Status.State == a2a.TaskStateFailed || currentTask.Status.State == a2a.TaskStateCanceled,
			}

			updateData, err := json.Marshal(statusUpdate)
			if err != nil {
				agent.logger.Error("failed to marshal status update", zap.Error(err))
				return
			}

			c.SSEvent("data", string(updateData))
			c.Writer.Flush()

			if statusUpdate.Final {
				c.SSEvent("", "[DONE]")
				c.Writer.Flush()
				return
			}
		case <-c.Request.Context().Done():
			agent.logger.Debug("client disconnected from stream")
			return
		}
	}
}

// parseMessageParams parses message parameters from JSON-RPC request
func (agent *A2AAgent) parseMessageParams(req a2a.JSONRPCRequest) (*a2a.MessageSendParams, []sdk.Message, error) {
	var params a2a.MessageSendParams
	paramsBytes, err := json.Marshal(req.Params)
	if err != nil {
		agent.logger.Error("failed to marshal params", zap.Error(err))
		return nil, nil, err
	}

	if err := json.Unmarshal(paramsBytes, &params); err != nil {
		agent.logger.Error("failed to parse message request", zap.Error(err))
		return nil, nil, err
	}

	if len(params.Message.Parts) <= 0 {
		return nil, nil, &InvalidRequestError{Message: "empty message parts"}
	}

	var messages []sdk.Message
	for _, part := range params.Message.Parts {
		partMap, ok := part.(map[string]interface{})
		if !ok {
			agent.logger.Error("failed to assert part to map")
			return nil, nil, &InvalidRequestError{Message: "invalid part format"}
		}

		textValue, exists := partMap["text"]
		if !exists {
			agent.logger.Error("part missing text field")
			return nil, nil, &InvalidRequestError{Message: "part missing text field"}
		}

		textString, ok := textValue.(string)
		if !ok {
			agent.logger.Error("text field is not a string")
			return nil, nil, &InvalidRequestError{Message: "text field is not a string"}
		}

		messages = append(messages, sdk.Message{
			Role:    sdk.MessageRole(params.Message.Role),
			Content: textString,
		})
	}

	return &params, messages, nil
}

// InvalidRequestError represents an invalid request error
type InvalidRequestError struct {
	Message string
}

func (e *InvalidRequestError) Error() string {
	return e.Message
}

// handleTaskGet handles tasks/get requests
func (agent *A2AAgent) handleTaskGet(c *gin.Context, req a2a.JSONRPCRequest) {
	var params a2a.TaskQueryParams
	paramsBytes, err := json.Marshal(req.Params)
	if err != nil {
		agent.logger.Error("failed to marshal params", zap.Error(err))
		agent.sendError(c, req.ID, int(ErrInvalidParams), "invalid params")
		return
	}

	if err := json.Unmarshal(paramsBytes, &params); err != nil {
		agent.logger.Error("failed to parse tasks/get request", zap.Error(err))
		agent.sendError(c, req.ID, int(ErrInvalidParams), "invalid request")
		return
	}

	agent.logger.Info("retrieving task", zap.String("task_id", params.ID))

	task, exists := agent.getTask(params.ID)
	if !exists {
		agent.logger.Error("task not found", zap.String("task_id", params.ID))
		agent.sendError(c, req.ID, int(ErrInvalidParams), "task not found")
		return
	}

	agent.logger.Info("task retrieved successfully", zap.String("task_id", params.ID), zap.String("status", string(task.Status.State)))

	c.JSON(http.StatusOK, a2a.JSONRPCSuccessResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  *task,
	})
}

// handleTaskCancel handles tasks/cancel requests
func (agent *A2AAgent) handleTaskCancel(c *gin.Context, req a2a.JSONRPCRequest) {
	var params a2a.TaskQueryParams
	paramsBytes, err := json.Marshal(req.Params)
	if err != nil {
		agent.logger.Error("failed to marshal params", zap.Error(err))
		agent.sendError(c, req.ID, int(ErrInvalidParams), "invalid params")
		return
	}

	if err := json.Unmarshal(paramsBytes, &params); err != nil {
		agent.logger.Error("failed to parse tasks/cancel request", zap.Error(err))
		agent.sendError(c, req.ID, int(ErrInvalidParams), "invalid request")
		return
	}

	agent.logger.Info("attempting to cancel task", zap.String("task_id", params.ID))

	task, exists := agent.getTask(params.ID)
	if !exists {
		agent.logger.Error("task not found", zap.String("task_id", params.ID))
		agent.sendError(c, req.ID, int(ErrInvalidParams), "task not found")
		return
	}

	if task.Status.State == a2a.TaskStateCompleted {
		agent.logger.Info("task already completed, cannot cancel", zap.String("task_id", params.ID))
		agent.sendError(c, req.ID, int(ErrInvalidParams), "task already completed")
		return
	}

	task.Status.State = a2a.TaskStateCanceled
	agent.storeTask(task)
	agent.sendPushNotification(task.ID, task)
	agent.logger.Info("task canceled", zap.String("task_id", params.ID))

	go func() {
		time.Sleep(agent.cfg.QueueConfig.CleanupInterval)
		agent.removeTask(task.ID)
		agent.logger.Debug("cleaned up canceled task", zap.String("task_id", task.ID), zap.Duration("cleanup_interval", agent.cfg.QueueConfig.CleanupInterval))
	}()

	c.JSON(http.StatusOK, a2a.JSONRPCSuccessResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  *task,
	})
}

// handleSetPushNotificationConfig handles push notification config setting
func (agent *A2AAgent) handleSetPushNotificationConfig(c *gin.Context, req a2a.JSONRPCRequest) {
	var params TaskPushNotificationConfig
	paramsBytes, err := json.Marshal(req.Params)
	if err != nil {
		agent.logger.Error("failed to marshal params", zap.Error(err))
		agent.sendError(c, req.ID, int(ErrInvalidParams), "invalid params")
		return
	}

	if err := json.Unmarshal(paramsBytes, &params); err != nil {
		agent.logger.Error("failed to parse push notification config request", zap.Error(err))
		agent.sendError(c, req.ID, int(ErrInvalidParams), "invalid request")
		return
	}

	agent.logger.Info("setting push notification config", zap.String("task_id", params.TaskID))

	_, exists := agent.getTask(params.TaskID)
	if !exists {
		agent.logger.Error("task not found for push notification config", zap.String("task_id", params.TaskID))
		agent.sendError(c, req.ID, int(ErrInvalidParams), "task not found")
		return
	}

	agent.pushNotificationsMu.Lock()
	agent.pushNotifications[params.TaskID] = params.PushNotificationConfig
	agent.pushNotificationsMu.Unlock()

	agent.logger.Info("push notification config set successfully", zap.String("task_id", params.TaskID), zap.String("url", params.PushNotificationConfig.URL))

	c.JSON(http.StatusOK, a2a.JSONRPCSuccessResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  params,
	})
}

// handleGetPushNotificationConfig handles push notification config retrieval
func (agent *A2AAgent) handleGetPushNotificationConfig(c *gin.Context, req a2a.JSONRPCRequest) {
	var params struct {
		TaskID string `json:"taskId"`
	}
	paramsBytes, err := json.Marshal(req.Params)
	if err != nil {
		agent.logger.Error("failed to marshal params", zap.Error(err))
		agent.sendError(c, req.ID, int(ErrInvalidParams), "invalid params")
		return
	}

	if err := json.Unmarshal(paramsBytes, &params); err != nil {
		agent.logger.Error("failed to parse get push notification config request", zap.Error(err))
		agent.sendError(c, req.ID, int(ErrInvalidParams), "invalid request")
		return
	}

	agent.logger.Info("getting push notification config", zap.String("task_id", params.TaskID))

	_, exists := agent.getTask(params.TaskID)
	if !exists {
		agent.logger.Error("task not found for push notification config", zap.String("task_id", params.TaskID))
		agent.sendError(c, req.ID, int(ErrInvalidParams), "task not found")
		return
	}

	agent.pushNotificationsMu.RLock()
	config, exists := agent.pushNotifications[params.TaskID]
	agent.pushNotificationsMu.RUnlock()

	if !exists {
		agent.logger.Info("no push notification config found", zap.String("task_id", params.TaskID))
		agent.sendError(c, req.ID, int(ErrInvalidParams), "no push notification config found")
		return
	}

	result := TaskPushNotificationConfig{
		TaskID:                 params.TaskID,
		PushNotificationConfig: config,
	}

	c.JSON(http.StatusOK, a2a.JSONRPCSuccessResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  result,
	})
}

// processTaskAsync processes a task asynchronously
func (agent *A2AAgent) processTaskAsync(queuedTask *QueuedTask) {
	agent.logger.Info("processing task from queue", zap.String("task_id", queuedTask.Task.ID))

	queuedTask.Task.Status.State = a2a.TaskStateWorking
	agent.storeTask(queuedTask.Task)
	agent.sendPushNotification(queuedTask.Task.ID, queuedTask.Task)

	defer func() {
		if r := recover(); r != nil {
			agent.logger.Error("panic in task processing", zap.Any("panic", r), zap.String("task_id", queuedTask.Task.ID))
			agent.failTaskWithCleanup(queuedTask.Task, "Task processing failed due to internal error")
		}
	}()

	agent.processTaskLogic(queuedTask.Task, queuedTask.Messages)
}

// processTaskWithStreaming processes a task with streaming
func (agent *A2AAgent) processTaskWithStreaming(task *a2a.Task, messages []sdk.Message, c *gin.Context, contextID string) {
	agent.logger.Info("processing task with streaming", zap.String("task_id", task.ID))

	task.Status.State = a2a.TaskStateWorking
	agent.storeTask(task)
	agent.sendPushNotification(task.ID, task)

	defer func() {
		if r := recover(); r != nil {
			agent.logger.Error("panic in streaming task processing", zap.Any("panic", r), zap.String("task_id", task.ID))
			agent.failTaskWithCleanup(task, "Task processing failed due to internal error")
		}
	}()

	agent.processTaskLogic(task, messages)
}

// processTaskLogic contains the core task processing logic
func (agent *A2AAgent) processTaskLogic(task *a2a.Task, messages []sdk.Message) {
	response, err := agent.client.WithTools(&agent.tools).WithHeader("X-A2A-Internal", "true").GenerateContent(context.Background(), sdk.Provider(agent.cfg.LLMProvider), agent.cfg.LLMModel, messages)
	if err != nil {
		agent.logger.Error("failed to create chat completion", zap.Error(err), zap.String("task_id", task.ID))
		agent.failTaskWithCleanup(task, "Failed to process weather request")
		return
	}

	if len(response.Choices) == 0 {
		agent.logger.Error("no choices returned from chat completion", zap.String("task_id", task.ID))
		agent.failTaskWithCleanup(task, "No response from weather service")
		return
	}

	var weatherResult string
	var iteration int

	for iteration < agent.cfg.MaxChatCompletionIterations {
		response, err = agent.client.WithTools(&agent.tools).WithHeader("X-A2A-Internal", "true").GenerateContent(context.Background(), sdk.Provider(agent.cfg.LLMProvider), agent.cfg.LLMModel, messages)
		if err != nil {
			agent.logger.Error("failed to create chat completion in iteration", zap.Error(err), zap.String("task_id", task.ID), zap.Int("iteration", iteration))
			agent.failTaskWithCleanup(task, "Failed to process weather request during iteration")
			return
		}

		if len(response.Choices) == 0 {
			agent.logger.Error("no choices returned from chat completion in iteration", zap.String("task_id", task.ID), zap.Int("iteration", iteration))
			agent.failTaskWithCleanup(task, "No response from weather service during processing")
			return
		}

		toolCalls := response.Choices[0].Message.ToolCalls

		if toolCalls == nil || len(*toolCalls) == 0 {
			agent.logger.Debug("no tool calls found in response, using text content", zap.String("task_id", task.ID))
			break
		}

		assistantMessage := sdk.Message{
			Role:      sdk.Assistant,
			Content:   response.Choices[0].Message.Content,
			ToolCalls: toolCalls,
		}
		messages = append(messages, assistantMessage)

		for _, toolCall := range *toolCalls {
			if toolCall.Function.Name != "fetch_weather" {
				agent.logger.Debug("ignoring tool call", zap.String("name", toolCall.Function.Name), zap.String("task_id", task.ID))
				continue
			}

			result, err := agent.weatherToolHandler.HandleFetchWeather(toolCall.Function.Arguments)
			if err != nil {
				agent.logger.Error("failed to handle fetch_weather tool call", zap.Error(err), zap.String("task_id", task.ID))
				agent.failTaskWithCleanup(task, "Invalid parameters for weather request")
				return
			}

			weatherResult = result

			toolResponse := sdk.Message{
				Role:       sdk.Tool,
				Content:    weatherResult,
				ToolCallId: &toolCall.Id,
			}

			messages = append(messages, toolResponse)
		}

		iteration++
	}

	if len(response.Choices) == 0 {
		agent.logger.Error("no choices returned from chat completion after iterations", zap.String("task_id", task.ID))
		agent.failTaskWithCleanup(task, "No final response from weather service")
		return
	}

	finalMessage := response.Choices[0].Message.Content

	agent.logger.Debug("weather result generated", zap.String("result", weatherResult), zap.String("task_id", task.ID))
	agent.logger.Debug("final response", zap.String("response", finalMessage), zap.String("task_id", task.ID))

	resultMessage := &a2a.Message{
		Kind:      "message",
		MessageID: uuid.New().String(),
		Role:      "assistant",
		Parts: []a2a.Part{
			a2a.TextPart{
				Kind: "text",
				Text: finalMessage,
			},
		},
	}

	task.Status.State = a2a.TaskStateCompleted
	task.Status.Message = resultMessage

	agent.storeTask(task)
	agent.sendPushNotification(task.ID, task)

	agent.logger.Info("task processing completed", zap.String("task_id", task.ID))

	go func() {
		time.Sleep(agent.cfg.QueueConfig.CleanupInterval)
		agent.removeTask(task.ID)
		agent.logger.Debug("cleaned up completed task", zap.String("task_id", task.ID), zap.Duration("cleanup_interval", agent.cfg.QueueConfig.CleanupInterval))
	}()
}

// sendPushNotification sends a push notification for a task
func (agent *A2AAgent) sendPushNotification(taskID string, task *a2a.Task) {
	agent.pushNotificationsMu.RLock()
	config, exists := agent.pushNotifications[taskID]
	agent.pushNotificationsMu.RUnlock()

	if !exists {
		agent.logger.Debug("no push notification config for task", zap.String("task_id", taskID))
		return
	}

	payload := map[string]interface{}{
		"taskId": taskID,
		"status": task.Status,
		"task":   task,
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		agent.logger.Error("failed to marshal push notification payload", zap.Error(err), zap.String("task_id", taskID))
		return
	}

	req, err := http.NewRequest("POST", config.URL, strings.NewReader(string(payloadBytes)))
	if err != nil {
		agent.logger.Error("failed to create push notification request", zap.Error(err), zap.String("task_id", taskID))
		return
	}

	req.Header.Set("Content-Type", "application/json")

	// Add authentication if provided
	if config.Token != "" {
		req.Header.Set("Authorization", "Bearer "+config.Token)
	}

	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	resp, err := client.Do(req)
	if err != nil {
		agent.logger.Error("failed to send push notification", zap.Error(err), zap.String("task_id", taskID), zap.String("url", config.URL))
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		agent.logger.Info("push notification sent successfully", zap.String("task_id", taskID), zap.String("url", config.URL), zap.Int("status", resp.StatusCode))
	} else {
		agent.logger.Warn("push notification failed", zap.String("task_id", taskID), zap.String("url", config.URL), zap.Int("status", resp.StatusCode))
	}
}

// Task storage and management methods
func (agent *A2AAgent) createTask(contextID string, state a2a.TaskState, message *a2a.Message) *a2a.Task {
	taskID := uuid.New().String()

	task := &a2a.Task{
		ID:        taskID,
		ContextID: contextID,
		Kind:      "task",
		Status: a2a.TaskStatus{
			State:   state,
			Message: message,
		},
	}

	agent.storeTask(task)
	return task
}

func (agent *A2AAgent) storeTask(task *a2a.Task) {
	agent.allTasksMu.Lock()
	defer agent.allTasksMu.Unlock()
	agent.allTasks[task.ID] = task
}

func (agent *A2AAgent) getTask(taskID string) (*a2a.Task, bool) {
	agent.allTasksMu.RLock()
	defer agent.allTasksMu.RUnlock()
	task, ok := agent.allTasks[taskID]
	return task, ok
}

func (agent *A2AAgent) removeTask(taskID string) {
	agent.allTasksMu.Lock()
	delete(agent.allTasks, taskID)
	agent.allTasksMu.Unlock()

	agent.pushNotificationsMu.Lock()
	delete(agent.pushNotifications, taskID)
	agent.pushNotificationsMu.Unlock()
}

func (agent *A2AAgent) failTaskWithCleanup(task *a2a.Task, errorMessage string) {
	failureMessage := &a2a.Message{
		Kind:      "message",
		MessageID: uuid.New().String(),
		Role:      "assistant",
		Parts: []a2a.Part{
			a2a.TextPart{
				Kind: "text",
				Text: errorMessage,
			},
		},
	}
	task.Status.State = a2a.TaskStateFailed
	task.Status.Message = failureMessage
	agent.storeTask(task)
	agent.sendPushNotification(task.ID, task)

	go func() {
		time.Sleep(agent.cfg.QueueConfig.CleanupInterval)
		agent.removeTask(task.ID)
		agent.logger.Debug("cleaned up failed task", zap.String("task_id", task.ID), zap.Duration("cleanup_interval", agent.cfg.QueueConfig.CleanupInterval))
	}()
}
