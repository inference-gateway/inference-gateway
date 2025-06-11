package core

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"time"

	gin "github.com/gin-gonic/gin"
	uuid "github.com/google/uuid"
	sdk "github.com/inference-gateway/sdk"
	zap "go.uber.org/zap"

	a2a "github.com/inference-gateway/inference-gateway/a2a"
)

// TaskResultProcessor defines how to process tool call results for task completion
type TaskResultProcessor interface {
	// ProcessToolResult processes a tool call result and returns a completion message if the task should be completed
	// Returns nil if the task should continue processing
	ProcessToolResult(toolCallResult string) *a2a.Message
}

// AgentInfoProvider defines how to provide agent-specific information
type AgentInfoProvider interface {
	// GetAgentCard returns the agent's capabilities and metadata
	GetAgentCard(baseConfig Config) a2a.AgentCard
}

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
	toolsHandler        *ToolsHandler
	taskQueue           chan *QueuedTask
	allTasks            map[string]*a2a.Task
	allTasksMu          sync.RWMutex
	pushNotifications   map[string]*PushNotificationConfig
	tools               []sdk.ChatCompletionTool
	taskResultProcessor TaskResultProcessor
	agentInfoProvider   AgentInfoProvider
}

// NewA2AAgent creates a new A2A agent
func NewA2AAgent(cfg Config, logger *zap.Logger, client sdk.Client, toolsHandler *ToolsHandler) *A2AAgent {
	tools := toolsHandler.GetAllToolDefinitions()

	return &A2AAgent{
		cfg:               cfg,
		logger:            logger,
		client:            client,
		toolsHandler:      toolsHandler,
		taskQueue:         make(chan *QueuedTask, cfg.QueueConfig.MaxSize),
		allTasks:          make(map[string]*a2a.Task),
		pushNotifications: make(map[string]*PushNotificationConfig),
		tools:             tools,
	}
}

// SetTaskResultProcessor sets the task result processor for custom business logic
func (agent *A2AAgent) SetTaskResultProcessor(processor TaskResultProcessor) {
	agent.taskResultProcessor = processor
}

// SetAgentInfoProvider sets the agent info provider for custom agent metadata
func (agent *A2AAgent) SetAgentInfoProvider(provider AgentInfoProvider) {
	agent.agentInfoProvider = provider
}

// SetupRouter configures the HTTP router with A2A endpoints
func (agent *A2AAgent) SetupRouter(oidcAuthenticator OIDCAuthenticator) *gin.Engine {
	r := gin.Default()

	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "healthy"})
	})

	r.GET("/.well-known/agent.json", agent.handleAgentInfo)

	if agent.cfg.AuthConfig.Enable {
		r.POST("/a2a", oidcAuthenticator.Middleware(), agent.handleA2ARequest)
	} else {
		r.POST("/a2a", agent.handleA2ARequest)
	}

	return r
}

// handleAgentInfo returns agent capabilities and metadata
func (agent *A2AAgent) handleAgentInfo(c *gin.Context) {
	agent.logger.Info("agent info requested")

	var info a2a.AgentCard
	if agent.agentInfoProvider != nil {
		info = agent.agentInfoProvider.GetAgentCard(agent.cfg)
	} else {
		info = a2a.AgentCard{
			Name:        agent.cfg.AgentName,
			Description: agent.cfg.AgentDescription,
			URL:         agent.cfg.AgentURL,
			Version:     agent.cfg.AgentVersion,
			Capabilities: a2a.AgentCapabilities{
				Streaming:              &agent.cfg.CapabilitiesConfig.Streaming,
				PushNotifications:      &agent.cfg.CapabilitiesConfig.PushNotifications,
				StateTransitionHistory: &agent.cfg.CapabilitiesConfig.StateTransitionHistory,
			},
			DefaultInputModes:  []string{"text/plain"},
			DefaultOutputModes: []string{"text/plain"},
			Skills:             []a2a.AgentSkill{},
		}
	}
	c.JSON(http.StatusOK, info)
}

// handleA2ARequest processes A2A protocol requests
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
	default:
		agent.logger.Warn("unknown method requested", zap.String("method", req.Method))
		agent.sendError(c, req.ID, int(ErrMethodNotFound), "method not found")
	}
}

// sendError sends a JSON-RPC error response
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

// sendSuccess sends a JSON-RPC success response
func (agent *A2AAgent) sendSuccess(c *gin.Context, id interface{}, result interface{}) {
	resp := a2a.JSONRPCSuccessResponse{
		JSONRPC: "2.0",
		ID:      id,
		Result:  result,
	}
	c.JSON(http.StatusOK, resp)
	agent.logger.Info("sending success response", zap.Any("id", id))
}

// handleMessageSend processes message/send requests
func (agent *A2AAgent) handleMessageSend(c *gin.Context, req a2a.JSONRPCRequest) {
	var params a2a.MessageSendParams
	paramsBytes, err := json.Marshal(req.Params)
	if err != nil {
		agent.logger.Error("failed to marshal params", zap.Error(err))
		agent.sendError(c, req.ID, int(ErrInvalidParams), "invalid params")
		return
	}
	if err := json.Unmarshal(paramsBytes, &params); err != nil {
		agent.logger.Error("failed to parse message/send request", zap.Error(err))
		agent.sendError(c, req.ID, int(ErrInvalidParams), "invalid request")
		return
	}

	if len(params.Message.Parts) == 0 {
		agent.sendError(c, req.ID, int(ErrInvalidRequest), "empty message parts")
		return
	}

	// Convert message parts to SDK messages
	messages, err := agent.convertPartsToMessages(params.Message.Parts, params.Message.Role)
	if err != nil {
		agent.logger.Error("failed to convert message parts", zap.Error(err))
		agent.sendError(c, req.ID, int(ErrInvalidParams), err.Error())
		return
	}

	contextID := params.Message.ContextID
	if contextID == nil {
		newContextID := uuid.New().String()
		contextID = &newContextID
	}

	task := agent.createTask(*contextID, a2a.TaskStateSubmitted, nil)

	queuedTask := &QueuedTask{
		Task:      task,
		Messages:  messages,
		RequestID: req.ID,
	}

	select {
	case agent.taskQueue <- queuedTask:
		agent.logger.Info("task queued for processing", zap.String("task_id", task.ID))
	default:
		agent.logger.Error("task queue is full")
		agent.updateTask(task.ID, a2a.TaskStateFailed, &a2a.Message{
			Kind:      "message",
			MessageID: uuid.New().String(),
			Role:      "assistant",
			Parts: []a2a.Part{
				map[string]interface{}{
					"kind": "text",
					"text": "Task queue is full. Please try again later.",
				},
			},
		})
	}

	agent.sendSuccess(c, req.ID, *task)
}

// convertPartsToMessages converts A2A message parts to SDK messages
func (agent *A2AAgent) convertPartsToMessages(parts []a2a.Part, role string) ([]sdk.Message, error) {
	var messages []sdk.Message
	for _, part := range parts {
		partMap, ok := part.(map[string]interface{})
		if !ok {
			return nil, NewInvalidPartFormatError()
		}

		textValue, exists := partMap["text"]
		if !exists {
			return nil, NewMissingTextFieldError()
		}

		textString, ok := textValue.(string)
		if !ok {
			return nil, NewInvalidTextFieldError()
		}

		messages = append(messages, sdk.Message{
			Role:    sdk.MessageRole(role),
			Content: textString,
		})
	}
	return messages, nil
}

// StartTaskProcessor starts the background task processing goroutine
func (agent *A2AAgent) StartTaskProcessor(ctx context.Context) {
	agent.logger.Info("starting task processor")

	go agent.startTaskCleanup(ctx)

	for {
		select {
		case <-ctx.Done():
			agent.logger.Info("task processor shutting down")
			return
		case queuedTask := <-agent.taskQueue:
			agent.processTask(ctx, queuedTask)
		}
	}
}

// processTask processes a single queued task
func (agent *A2AAgent) processTask(ctx context.Context, queuedTask *QueuedTask) {
	task := queuedTask.Task
	messages := queuedTask.Messages

	agent.logger.Info("processing task", zap.String("task_id", task.ID))
	agent.updateTask(task.ID, a2a.TaskStateWorking, nil)

	var iteration int
	for iteration < agent.cfg.MaxChatCompletionIterations {
		response, err := agent.client.WithTools(&agent.tools).WithHeader("X-A2A-Internal", "true").GenerateContent(ctx, sdk.Provider(agent.cfg.LLMProvider), agent.cfg.LLMModel, messages)
		if err != nil {
			agent.logger.Error("failed to create chat completion", zap.Error(err))
			agent.updateTask(task.ID, a2a.TaskStateFailed, &a2a.Message{
				Kind:      "message",
				MessageID: uuid.New().String(),
				Role:      "assistant",
				Parts: []a2a.Part{
					map[string]interface{}{
						"kind": "text",
						"text": "failed to process request: " + err.Error(),
					},
				},
			})
			return
		}

		if len(response.Choices) == 0 {
			agent.logger.Error("no choices returned from chat completion")
			agent.updateTask(task.ID, a2a.TaskStateFailed, &a2a.Message{
				Kind:      "message",
				MessageID: uuid.New().String(),
				Role:      "assistant",
				Parts: []a2a.Part{
					map[string]interface{}{
						"kind": "text",
						"text": "no response generated",
					},
				},
			})
			return
		}

		toolCalls := response.Choices[0].Message.ToolCalls

		if toolCalls == nil || len(*toolCalls) == 0 {
			agent.logger.Debug("no tool calls found in response, using text content")
			resultMessage := &a2a.Message{
				Kind:      "message",
				MessageID: uuid.New().String(),
				Role:      "assistant",
				Parts: []a2a.Part{
					map[string]interface{}{
						"kind": "text",
						"text": response.Choices[0].Message.Content,
					},
				},
			}
			agent.updateTask(task.ID, a2a.TaskStateCompleted, resultMessage)
			return
		}

		assistantMessage := sdk.Message{
			Role:      sdk.Assistant,
			Content:   response.Choices[0].Message.Content,
			ToolCalls: toolCalls,
		}
		messages = append(messages, assistantMessage)

		var allToolResults []string
		for _, toolCall := range *toolCalls {
			if agent.toolsHandler.IsToolSupported(toolCall.Function.Name) {
				result, err := agent.toolsHandler.HandleToolCall(toolCall)
				if err != nil {
					agent.logger.Error("failed to handle tool call",
						zap.String("tool", toolCall.Function.Name),
						zap.Error(err))
					continue
				}
				allToolResults = append(allToolResults, result)

				toolResponse := sdk.Message{
					Role:       sdk.Tool,
					Content:    result,
					ToolCallId: &toolCall.Id,
				}
				messages = append(messages, toolResponse)
			} else {
				agent.logger.Debug("ignoring unsupported tool call",
					zap.String("tool", toolCall.Function.Name))
			}
		}

		// Check if any tool result should complete the task using the processor
		if agent.taskResultProcessor != nil {
			for _, toolResult := range allToolResults {
				if completionMessage := agent.taskResultProcessor.ProcessToolResult(toolResult); completionMessage != nil {
					agent.updateTask(task.ID, a2a.TaskStateCompleted, completionMessage)
					return
				}
			}
		}

		iteration++
	}

	agent.updateTask(task.ID, a2a.TaskStateFailed, &a2a.Message{
		Kind:      "message",
		MessageID: uuid.New().String(),
		Role:      "assistant",
		Parts: []a2a.Part{
			map[string]interface{}{
				"kind": "text",
				"text": "maximum processing iterations reached",
			},
		},
	})
}

// handleMessageStream processes message/stream requests
func (agent *A2AAgent) handleMessageStream(c *gin.Context, req a2a.JSONRPCRequest) {
	agent.logger.Info("streaming not implemented yet")
	agent.sendError(c, req.ID, int(ErrServerError), "streaming not implemented")
}

// handleTaskGet processes tasks/get requests
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
	agent.sendSuccess(c, req.ID, *task)
}

// handleTaskCancel processes tasks/cancel requests
func (agent *A2AAgent) handleTaskCancel(c *gin.Context, req a2a.JSONRPCRequest) {
	agent.logger.Info("tasks/cancel not implemented yet")
	agent.sendError(c, req.ID, int(ErrServerError), "tasks/cancel not implemented")
}

// createTask creates a new task and stores it
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

	agent.allTasksMu.Lock()
	agent.allTasks[taskID] = task
	agent.allTasksMu.Unlock()

	return task
}

// updateTask updates an existing task
func (agent *A2AAgent) updateTask(taskID string, state a2a.TaskState, message *a2a.Message) {
	agent.allTasksMu.Lock()
	defer agent.allTasksMu.Unlock()

	if task, exists := agent.allTasks[taskID]; exists {
		task.Status.State = state
		task.Status.Message = message
	}
}

// getTask retrieves a task by ID
func (agent *A2AAgent) getTask(taskID string) (*a2a.Task, bool) {
	agent.allTasksMu.RLock()
	task, ok := agent.allTasks[taskID]
	agent.allTasksMu.RUnlock()
	return task, ok
}

// startTaskCleanup starts the background task cleanup process
func (agent *A2AAgent) startTaskCleanup(ctx context.Context) {
	ticker := time.NewTicker(agent.cfg.QueueConfig.CleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			agent.logger.Info("task cleanup shutting down")
			return
		case <-ticker.C:
			agent.cleanupCompletedTasks()
		}
	}
}

// cleanupCompletedTasks removes old completed tasks from memory
func (agent *A2AAgent) cleanupCompletedTasks() {
	agent.allTasksMu.Lock()
	defer agent.allTasksMu.Unlock()

	var toRemove []string

	for taskID, task := range agent.allTasks {
		if task.Status.State == a2a.TaskStateCompleted || task.Status.State == a2a.TaskStateFailed {
			toRemove = append(toRemove, taskID)
		}
	}

	for _, taskID := range toRemove {
		delete(agent.allTasks, taskID)
	}

	if len(toRemove) > 0 {
		agent.logger.Debug("cleaned up completed tasks",
			zap.Int("count", len(toRemove)),
			zap.Duration("retention_period", agent.cfg.QueueConfig.CleanupInterval))
	}
}

// Error types for better error handling
func NewInvalidPartFormatError() error {
	return &InvalidPartFormatError{}
}

func NewMissingTextFieldError() error {
	return &MissingTextFieldError{}
}

func NewInvalidTextFieldError() error {
	return &InvalidTextFieldError{}
}

type InvalidPartFormatError struct{}

func (e *InvalidPartFormatError) Error() string { return "invalid part format" }

type MissingTextFieldError struct{}

func (e *MissingTextFieldError) Error() string { return "part missing text field" }

type InvalidTextFieldError struct{}

func (e *InvalidTextFieldError) Error() string { return "text field is not a string" }
