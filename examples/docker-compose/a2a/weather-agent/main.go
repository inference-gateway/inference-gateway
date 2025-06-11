package main

import (
	"context"
	"encoding/json"
	"log"
	"math/rand"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/sethvargo/go-envconfig"
	"go.uber.org/zap"

	sdk "github.com/inference-gateway/sdk"

	a2a "github.com/inference-gateway/inference-gateway/a2a"
)

var logger *zap.Logger

type Config struct {
	Debug               bool   `env:"DEBUG,default=false"`
	Port                string `env:"PORT,default=8080"`
	InferenceGatewayURL string `env:"INFERENCE_GATEWAY_URL,required"`
	LLMProvider         string `env:"LLM_PROVIDER,default=deepseek"`
	LLMModel            string `env:"LLM_MODEL,default=deepseek-chat"`
	MaxIterations       int    `env:"MAX_ITERATIONS,default=10"`
}

type JRPCErrorCode int

const (
	ErrParseError     JRPCErrorCode = -32700
	ErrInvalidRequest JRPCErrorCode = -32600
	ErrMethodNotFound JRPCErrorCode = -32601
	ErrInvalidParams  JRPCErrorCode = -32602
	ErrInternalError  JRPCErrorCode = -32603
	ErrServerError    JRPCErrorCode = -32000
)

var tools = []sdk.ChatCompletionTool{
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

type WeatherData struct {
	Location    string  `json:"location"`
	Temperature float64 `json:"temperature"`
	Humidity    int     `json:"humidity"`
	Condition   string  `json:"condition"`
	WindSpeed   float64 `json:"wind_speed"`
	Pressure    float64 `json:"pressure"`
	Timestamp   string  `json:"timestamp"`
}

type FetchWeatherParams struct {
	Location string `json:"location"`
}

type QueuedTask struct {
	Task      *a2a.Task
	Messages  []sdk.Message
	RequestID interface{}
}

var cfg Config

func setupRouter(logger *zap.Logger, client sdk.Client) *gin.Engine {
	r := gin.Default()

	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "healthy"})
	})

	r.POST("/a2a", func(c *gin.Context) {
		handleA2ARequest(c, logger, client)
	})

	r.GET("/.well-known/agent.json", func(c *gin.Context) {
		logger.Info("agent info requested")
		streaming := true
		pushNotifications := false
		stateTransitionHistory := false

		info := a2a.AgentCard{
			Name:        "weather-agent",
			Description: "A weather information agent that provides current weather data using AI tools",
			URL:         "http://weather-agent:8080",
			Version:     "1.0.0",
			Capabilities: a2a.AgentCapabilities{
				Streaming:              &streaming,
				PushNotifications:      &pushNotifications,
				StateTransitionHistory: &stateTransitionHistory,
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

func main() {
	ctx := context.Background()

	if err := envconfig.Process(ctx, &cfg); err != nil {
		log.Fatal("failed to process configuration:", err)
	}

	var err error
	if cfg.Debug {
		logger, err = zap.NewDevelopment()
	} else {
		logger, err = zap.NewProduction()
	}
	if err != nil {
		log.Fatal("failed to initialize logger:", err)
	}
	defer logger.Sync()

	client := sdk.NewClient(&sdk.ClientOptions{
		BaseURL: cfg.InferenceGatewayURL,
	})

	logger.Info("starting weather agent",
		zap.String("version", "1.0.0"),
		zap.String("port", cfg.Port),
		zap.String("inference_gateway_url", cfg.InferenceGatewayURL),
		zap.String("llm_provider", cfg.LLMProvider),
		zap.String("llm_model", cfg.LLMModel),
		zap.Bool("debug_mode", cfg.Debug))

	go startTaskProcessor(ctx, logger, client)

	router := setupRouter(logger, client)

	logger.Info("weather-agent starting on port 8080...")
	if err := router.Run(":8080"); err != nil {
		logger.Fatal("failed to start server", zap.Error(err))
	}
}

func handleA2ARequest(c *gin.Context, logger *zap.Logger, client sdk.Client) {
	var req a2a.JSONRPCRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		logger.Error("failed to parse json request", zap.Error(err))
		sendError(c, req.ID, int(ErrParseError), "parse error", logger)
		return
	}

	if req.JSONRPC == "" {
		req.JSONRPC = "2.0"
	}

	if req.ID == nil {
		id := interface{}(uuid.New().String())
		req.ID = &id
	}

	logger.Info("received a2a request",
		zap.String("method", req.Method),
		zap.Any("id", req.ID))

	switch req.Method {
	case "message/send":
		handleMessageSend(c, req, logger, client)
	case "message/stream":
		handleMessageStream(c, req, logger, client)
	case "task/get":
		handleTaskGet(c, req, logger, client)
	case "task/cancel":
		handleTaskCancel(c, req, logger, client)
	default:
		logger.Warn("unknown method requested", zap.String("method", req.Method))
		sendError(c, req.ID, int(ErrMethodNotFound), "method not found", logger)
	}
}

func sendError(c *gin.Context, id interface{}, code int, message string, logger *zap.Logger) {
	resp := a2a.JSONRPCErrorResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error: &a2a.JSONRPCError{
			Code:    code,
			Message: message,
		},
	}
	c.JSON(http.StatusOK, resp)
	logger.Error("sending error response", zap.Int("code", code), zap.String("message", message))
}

func handleMessageSend(c *gin.Context, req a2a.JSONRPCRequest, logger *zap.Logger, client sdk.Client) {
	var params a2a.MessageSendParams
	paramsBytes, err := json.Marshal(req.Params)
	if err != nil {
		logger.Error("failed to marshal params", zap.Error(err))
		sendError(c, req.ID, int(ErrInvalidParams), "invalid params", logger)
		return
	}
	if err := json.Unmarshal(paramsBytes, &params); err != nil {
		logger.Error("failed to parse message/send request", zap.Error(err))
		sendError(c, req.ID, int(ErrInvalidParams), "invalid request", logger)
		return
	}

	if len(params.Message.Parts) <= 0 {
		sendError(c, req.ID, -32600, "invalid request", logger)
		return
	}

	defer func() {
		logger.Debug("message/send request processed")
	}()

	contextID := params.Message.ContextID
	if contextID == nil {
		newContextID := uuid.New().String()
		contextID = &newContextID
	}

	task := createTask(*contextID, a2a.TaskStateSubmitted, nil)

	logger.Info("task created and queued", zap.String("task_id", task.ID))

	var messages []sdk.Message
	for _, part := range params.Message.Parts {
		partMap, ok := part.(map[string]interface{})
		if !ok {
			logger.Error("failed to assert part to map")
			sendError(c, req.ID, int(ErrInvalidParams), "invalid part format", logger)
			return
		}

		textValue, exists := partMap["text"]
		if !exists {
			logger.Error("part missing text field")
			sendError(c, req.ID, int(ErrInvalidParams), "part missing text field", logger)
			return
		}

		textString, ok := textValue.(string)
		if !ok {
			logger.Error("text field is not a string")
			sendError(c, req.ID, int(ErrInvalidParams), "text field is not a string", logger)
			return
		}

		messages = append(messages, sdk.Message{
			Role:    sdk.MessageRole(params.Message.Role),
			Content: textString,
		})
	}

	queuedTask := &QueuedTask{
		Task:      task,
		Messages:  messages,
		RequestID: req.ID,
	}

	select {
	case taskQueue <- queuedTask:
		logger.Info("task added to queue", zap.String("task_id", task.ID))
	default:
		logger.Error("task queue is full", zap.String("task_id", task.ID))
		task.Status.State = a2a.TaskStateFailed
		task.Status.Message = &a2a.Message{
			Kind:      "message",
			MessageID: uuid.New().String(),
			Role:      "assistant",
			Parts: []a2a.Part{
				map[string]interface{}{
					"kind": "text",
					"text": "Task queue is full, please try again later",
				},
			},
		}
		storeTask(task) // Persist the failed state
		sendError(c, req.ID, int(ErrServerError), "task queue is full", logger)
		return
	}

	c.JSON(http.StatusOK, a2a.JSONRPCSuccessResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  *task,
	})
}

func handleMessageStream(c *gin.Context, req a2a.JSONRPCRequest, logger *zap.Logger, client sdk.Client) {
	logger.Info("streaming not implemented yet")
	sendError(c, req.ID, int(ErrServerError), "streaming not implemented", logger)
}

func handleTaskGet(c *gin.Context, req a2a.JSONRPCRequest, logger *zap.Logger, client sdk.Client) {
	var params a2a.TaskQueryParams
	paramsBytes, err := json.Marshal(req.Params)
	if err != nil {
		logger.Error("failed to marshal params", zap.Error(err))
		sendError(c, req.ID, int(ErrInvalidParams), "invalid params", logger)
		return
	}

	if err := json.Unmarshal(paramsBytes, &params); err != nil {
		logger.Error("failed to parse task/get request", zap.Error(err))
		sendError(c, req.ID, int(ErrInvalidParams), "invalid request", logger)
		return
	}

	logger.Info("retrieving task", zap.String("task_id", params.ID))

	task, exists := getTask(params.ID)
	if !exists {
		logger.Error("task not found", zap.String("task_id", params.ID))
		sendError(c, req.ID, int(ErrInvalidParams), "task not found", logger)
		return
	}

	logger.Info("task retrieved successfully", zap.String("task_id", params.ID), zap.String("status", string(task.Status.State)))

	c.JSON(http.StatusOK, a2a.JSONRPCSuccessResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  *task,
	})
}

func handleTaskCancel(c *gin.Context, req a2a.JSONRPCRequest, logger *zap.Logger, client sdk.Client) {
	var params a2a.TaskQueryParams
	paramsBytes, err := json.Marshal(req.Params)
	if err != nil {
		logger.Error("failed to marshal params", zap.Error(err))
		sendError(c, req.ID, int(ErrInvalidParams), "invalid params", logger)
		return
	}

	if err := json.Unmarshal(paramsBytes, &params); err != nil {
		logger.Error("failed to parse task/cancel request", zap.Error(err))
		sendError(c, req.ID, int(ErrInvalidParams), "invalid request", logger)
		return
	}

	logger.Info("attempting to cancel task", zap.String("task_id", params.ID))

	task, exists := getTask(params.ID)
	if !exists {
		logger.Error("task not found", zap.String("task_id", params.ID))
		sendError(c, req.ID, int(ErrInvalidParams), "task not found", logger)
		return
	}

	if task.Status.State == a2a.TaskStateCompleted {
		logger.Info("task already completed, cannot cancel", zap.String("task_id", params.ID))
		sendError(c, req.ID, int(ErrInvalidParams), "task already completed", logger)
		return
	}

	task.Status.State = a2a.TaskStateCanceled
	storeTask(task) // Persist the canceled state
	logger.Info("task canceled", zap.String("task_id", params.ID))

	go func() {
		time.Sleep(10 * time.Second)
		removeTask(task.ID)
		logger.Debug("cleaned up canceled task after 10 seconds", zap.String("task_id", task.ID))
	}()

	c.JSON(http.StatusOK, a2a.JSONRPCSuccessResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  *task,
	})
}

func stringPtr(s string) *string {
	return &s
}

// fetchWeather simulates fetching weather data for the given location
func fetchWeather(location string) *WeatherData {
	logger.Debug("generating weather data for location", zap.String("location", location))

	conditions := []string{"sunny", "partly cloudy", "cloudy", "rainy", "stormy", "snowy", "foggy"}
	condition := conditions[rand.Intn(len(conditions))]

	var temp float64
	switch condition {
	case "sunny":
		temp = 20 + rand.Float64()*15 // 20-35°C
	case "partly cloudy", "cloudy":
		temp = 15 + rand.Float64()*10 // 15-25°C
	case "rainy", "stormy":
		temp = 10 + rand.Float64()*10 // 10-20°C
	case "snowy":
		temp = -5 + rand.Float64()*10 // -5-5°C
	case "foggy":
		temp = 5 + rand.Float64()*15 // 5-20°C
	default:
		temp = 15 + rand.Float64()*10
	}

	weather := &WeatherData{
		Location:    location,
		Temperature: float64(int(temp*10)) / 10, // Round to 1 decimal
		Humidity:    30 + rand.Intn(51),         // 30-80%
		Condition:   condition,
		WindSpeed:   float64(rand.Intn(31)),  // 0-30 km/h
		Pressure:    980 + rand.Float64()*50, // 980-1030 hPa
		Timestamp:   time.Now().Format("2006-01-02T15:04:05Z"),
	}

	logger.Debug("generated weather data",
		zap.String("location", weather.Location),
		zap.Float64("temperature", weather.Temperature),
		zap.String("condition", weather.Condition),
		zap.Int("humidity", weather.Humidity),
		zap.Float64("wind_speed", weather.WindSpeed),
		zap.Float64("pressure", weather.Pressure))

	return weather
}

var (
	taskQueue = make(chan *QueuedTask, 100)
	// In-memory store for all tasks (submitted, working, completed, failed, canceled)
	allTasks   = make(map[string]*a2a.Task)
	allTasksMu sync.RWMutex
)

func startTaskProcessor(ctx context.Context, logger *zap.Logger, client sdk.Client) {
	logger.Info("starting background task processor")

	for {
		select {
		case <-ctx.Done():
			logger.Info("task processor shutting down")
			return
		case queuedTask := <-taskQueue:
			processTaskAsync(queuedTask, logger, client)
		}
	}
}

func processTaskAsync(queuedTask *QueuedTask, logger *zap.Logger, client sdk.Client) {
	logger.Info("processing task from queue", zap.String("task_id", queuedTask.Task.ID))

	queuedTask.Task.Status.State = a2a.TaskStateWorking
	storeTask(queuedTask.Task)

	defer func() {
		if r := recover(); r != nil {
			logger.Error("panic in task processing", zap.Any("panic", r), zap.String("task_id", queuedTask.Task.ID))
			failTaskWithCleanup(queuedTask.Task, "Task processing failed due to internal error", logger)
		}
	}()

	response, err := client.WithTools(&tools).WithHeader("X-A2A-Internal", "true").GenerateContent(context.Background(), sdk.Provider(cfg.LLMProvider), cfg.LLMModel, queuedTask.Messages)
	if err != nil {
		logger.Error("failed to create chat completion", zap.Error(err), zap.String("task_id", queuedTask.Task.ID))
		failTaskWithCleanup(queuedTask.Task, "Failed to process weather request", logger)
		return
	}

	if len(response.Choices) == 0 {
		logger.Error("no choices returned from chat completion", zap.String("task_id", queuedTask.Task.ID))
		failTaskWithCleanup(queuedTask.Task, "No response from weather service", logger)
		return
	}

	messages := queuedTask.Messages
	var weatherResult string
	var iteration int

	for iteration < cfg.MaxIterations {
		response, err = client.WithTools(&tools).WithHeader("X-A2A-Internal", "true").GenerateContent(context.Background(), sdk.Provider(cfg.LLMProvider), cfg.LLMModel, messages)
		if err != nil {
			logger.Error("failed to create chat completion in iteration", zap.Error(err), zap.String("task_id", queuedTask.Task.ID), zap.Int("iteration", iteration))
			failureMessage := &a2a.Message{
				Kind:      "message",
				MessageID: uuid.New().String(),
				Role:      "assistant",
				Parts: []a2a.Part{
					map[string]interface{}{
						"kind": "text",
						"text": "Failed to process weather request during iteration",
					},
				},
			}
			queuedTask.Task.Status.State = a2a.TaskStateFailed
			queuedTask.Task.Status.Message = failureMessage
			return
		}

		if len(response.Choices) == 0 {
			logger.Error("no choices returned from chat completion in iteration", zap.String("task_id", queuedTask.Task.ID), zap.Int("iteration", iteration))
			failureMessage := &a2a.Message{
				Kind:      "message",
				MessageID: uuid.New().String(),
				Role:      "assistant",
				Parts: []a2a.Part{
					map[string]interface{}{
						"kind": "text",
						"text": "No response from weather service during processing",
					},
				},
			}
			queuedTask.Task.Status.State = a2a.TaskStateFailed
			queuedTask.Task.Status.Message = failureMessage
			return
		}

		toolCalls := response.Choices[0].Message.ToolCalls

		if toolCalls == nil || len(*toolCalls) == 0 {
			logger.Debug("no tool calls found in response, using text content", zap.String("task_id", queuedTask.Task.ID))
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
				logger.Debug("ignoring tool call", zap.String("name", toolCall.Function.Name), zap.String("task_id", queuedTask.Task.ID))
				continue
			}

			var weatherParams FetchWeatherParams
			if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &weatherParams); err != nil {
				logger.Error("failed to unmarshal fetch_weather parameters", zap.Error(err), zap.String("task_id", queuedTask.Task.ID))
				failureMessage := &a2a.Message{
					Kind:      "message",
					MessageID: uuid.New().String(),
					Role:      "assistant",
					Parts: []a2a.Part{
						map[string]interface{}{
							"kind": "text",
							"text": "Invalid parameters for weather request",
						},
					},
				}
				queuedTask.Task.Status.State = a2a.TaskStateFailed
				queuedTask.Task.Status.Message = failureMessage
				return
			}

			weatherData := fetchWeather(weatherParams.Location)
			weatherJSON, _ := json.Marshal(weatherData)
			weatherResult = string(weatherJSON)

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
		logger.Error("no choices returned from chat completion after iterations", zap.String("task_id", queuedTask.Task.ID))
		failureMessage := &a2a.Message{
			Kind:      "message",
			MessageID: uuid.New().String(),
			Role:      "assistant",
			Parts: []a2a.Part{
				map[string]interface{}{
					"kind": "text",
					"text": "No final response from weather service",
				},
			},
		}
		queuedTask.Task.Status.State = a2a.TaskStateFailed
		queuedTask.Task.Status.Message = failureMessage
		return
	}

	finalMessage := response.Choices[0].Message.Content

	logger.Debug("weather result generated", zap.String("result", weatherResult), zap.String("task_id", queuedTask.Task.ID))
	logger.Debug("final response", zap.String("response", finalMessage), zap.String("task_id", queuedTask.Task.ID))

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

	queuedTask.Task.Status.State = a2a.TaskStateCompleted
	queuedTask.Task.Status.Message = resultMessage

	storeTask(queuedTask.Task)

	logger.Info("task processing completed", zap.String("task_id", queuedTask.Task.ID))

	go func() {
		time.Sleep(10 * time.Second)
		removeTask(queuedTask.Task.ID)
		logger.Debug("cleaned up completed task after 10 seconds", zap.String("task_id", queuedTask.Task.ID))
	}()
}

func createTask(contextID string, state a2a.TaskState, message *a2a.Message) *a2a.Task {
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

	storeTask(task)
	return task
}

func storeTask(task *a2a.Task) {
	allTasksMu.Lock()
	defer allTasksMu.Unlock()
	allTasks[task.ID] = task
}

func getTask(taskID string) (*a2a.Task, bool) {
	allTasksMu.RLock()
	defer allTasksMu.RUnlock()
	task, ok := allTasks[taskID]
	return task, ok
}

func removeTask(taskID string) {
	allTasksMu.Lock()
	defer allTasksMu.Unlock()
	delete(allTasks, taskID)
}

func failTaskWithCleanup(task *a2a.Task, errorMessage string, logger *zap.Logger) {
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
	storeTask(task)

	go func() {
		time.Sleep(10 * time.Second)
		removeTask(task.ID)
		logger.Debug("cleaned up failed task after 10 seconds", zap.String("task_id", task.ID))
	}()
}
