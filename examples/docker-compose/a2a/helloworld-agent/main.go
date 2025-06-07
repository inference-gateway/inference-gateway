package main

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"

	"helloworld-agent/a2a"
)

var logger *zap.Logger

func main() {
	var err error
	logger, err = zap.NewProduction()
	if err != nil {
		panic("failed to initialize logger: " + err.Error())
	}
	defer logger.Sync()

	r := gin.Default()

	r.GET("/health", func(c *gin.Context) {
		logger.Info("health check requested")
		c.JSON(http.StatusOK, gin.H{"status": "healthy"})
	})

	r.POST("/a2a", handleA2ARequest)

	r.GET("/.well-known/agent.json", func(c *gin.Context) {
		logger.Info("agent info requested")
		info := a2a.AgentCard{
			Name:        "helloworld-agent",
			Description: "A simple greeting agent that provides personalized greetings using the A2A protocol",
			URL:         "http://localhost:8081",
			Version:     "1.0.0",
			Capabilities: a2a.AgentCapabilities{
				Streaming:              false,
				Pushnotifications:      false,
				Statetransitionhistory: false,
			},
			Defaultinputmodes:  []string{"text/plain"},
			Defaultoutputmodes: []string{"text/plain"},
			Skills: []a2a.AgentSkill{
				{
					ID:          "greeting",
					Name:        "greeting",
					Description: "Provide personalized greetings in multiple languages",
					Inputmodes:  []string{"text/plain"},
					Outputmodes: []string{"text/plain"},
				},
			},
		}
		c.JSON(http.StatusOK, info)
	})

	logger.Info("helloworld-agent starting", zap.String("port", "8081"))
	if err := r.Run(":8081"); err != nil {
		logger.Fatal("failed to start server", zap.Error(err))
	}
}

func containsSpanishRequest(text string) bool {
	lowerText := strings.ToLower(text)

	switch {
	case strings.Contains(lowerText, "spanish"),
		strings.Contains(lowerText, "español"),
		strings.Contains(lowerText, "espanol"),
		strings.Contains(lowerText, "en español"),
		strings.Contains(lowerText, "en espanol"),
		strings.Contains(lowerText, "greet me in spanish"),
		strings.Contains(lowerText, "greeting in spanish"),
		strings.Contains(lowerText, "hola"),
		strings.Contains(lowerText, "buenos días"),
		strings.Contains(lowerText, "buenas tardes"),
		strings.Contains(lowerText, "saludar"):
		logger.Debug("spanish language detected", zap.String("text", text))
		return true
	default:
		return false
	}
}

func handleA2ARequest(c *gin.Context) {
	var req a2a.JSONRPCRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		logger.Error("failed to parse json request", zap.Error(err))
		sendError(c, req.ID, -32700, "parse error")
		return
	}

	if req.Jsonrpc == "" {
		req.Jsonrpc = "2.0"
	}

	if req.ID == nil {
		req.ID = uuid.New().String()
	}

	logger.Info("received a2a request",
		zap.String("method", req.Method),
		zap.Any("id", req.ID))

	switch req.Method {
	case "message/send":
		handleMessageSend(c, req)
	case "message/stream":
		handleMessageStream(c, req)
	case "task/get":
		handleTaskGet(c, req)
	case "task/cancel":
		handleTaskCancel(c, req)
	default:
		logger.Warn("unknown method requested", zap.String("method", req.Method))
		sendError(c, req.ID, -32601, "method not found")
	}
}

func handleMessageSend(c *gin.Context, req a2a.JSONRPCRequest) {
	logger.Info("processing message/send request", zap.Any("requestId", req.ID))

	paramsMap, ok := req.Params["message"].(map[string]interface{})
	if !ok {
		logger.Error("invalid params: missing message", zap.Any("params", req.Params))
		sendError(c, req.ID, -32602, "invalid params: missing message")
		return
	}

	partsArray, ok := paramsMap["parts"].([]interface{})
	if !ok {
		logger.Error("invalid params: missing message parts", zap.Any("message", paramsMap))
		sendError(c, req.ID, -32602, "invalid params: missing message parts")
		return
	}

	var messageText string = "World"
	for _, partInterface := range partsArray {
		part, ok := partInterface.(map[string]interface{})
		if !ok {
			continue
		}

		if partType, exists := part["type"]; exists && partType == "text" {
			if text, textExists := part["text"].(string); textExists {
				messageText = text
				break
			}
		}
	}

	logger.Info("extracted message text", zap.String("text", messageText))

	greeting := generateGreeting(messageText)

	logger.Info("generated greeting",
		zap.String("greeting", greeting),
		zap.String("originalText", messageText))

	taskId := uuid.New().String()
	contextId := uuid.New().String()
	messageId := uuid.New().String()

	responseMessage := a2a.Message{
		Role:      "assistant",
		MessageId: messageId,
		ContextId: contextId,
		TaskId:    taskId,
		Parts: []a2a.Part{
			{
				Type: "text",
				Text: greeting,
			},
		},
	}

	task := a2a.Task{
		Id:        taskId,
		ContextId: contextId,
		Status: a2a.TaskStatus{
			State:     "completed",
			Timestamp: time.Now(),
			Message:   &responseMessage,
		},
		Artifacts: []a2a.Artifact{
			{
				ArtifactId: uuid.New().String(),
				Name:       "greeting",
				Parts: []a2a.Part{
					{
						Type: "text",
						Text: greeting,
					},
				},
			},
		},
		History: []a2a.Message{
			{
				Role:      "user",
				MessageId: getStringParam(paramsMap, "messageId", uuid.New().String()),
				ContextId: contextId,
				TaskId:    taskId,
				Parts: []a2a.Part{
					{
						Type: "text",
						Text: messageText,
					},
				},
			},
			responseMessage,
		},
		Kind: "task",
	}

	response := a2a.JSONRPCSuccessResponse{
		ID:      req.ID,
		Jsonrpc: "2.0",
		Result:  task,
	}

	logger.Info("sending response",
		zap.String("taskId", taskId),
		zap.String("status", "completed"))

	c.JSON(http.StatusOK, response)
}

func handleMessageStream(c *gin.Context, req a2a.JSONRPCRequest) {
	logger.Info("processing message/stream request", zap.Any("requestId", req.ID))
	handleMessageSend(c, req)
}

func handleTaskGet(c *gin.Context, req a2a.JSONRPCRequest) {
	logger.Warn("task/get not implemented", zap.Any("requestId", req.ID))
	sendError(c, req.ID, -32601, "task/get not implemented")
}

func handleTaskCancel(c *gin.Context, req a2a.JSONRPCRequest) {
	logger.Warn("task/cancel not implemented", zap.Any("requestId", req.ID))
	sendError(c, req.ID, -32601, "task/cancel not implemented")
}

func getStringParam(params map[string]interface{}, key string, defaultValue string) string {
	if value, exists := params[key]; exists {
		if str, ok := value.(string); ok {
			logger.Debug("parameter found",
				zap.String("key", key),
				zap.String("value", str))
			return str
		}
		logger.Warn("parameter value is not a string",
			zap.String("key", key),
			zap.Any("value", value))
	} else {
		logger.Debug("parameter not found, using default",
			zap.String("key", key),
			zap.String("default", defaultValue))
	}
	return defaultValue
}

func sendError(c *gin.Context, id interface{}, code int, message string) {
	logger.Error("sending error response",
		zap.Any("id", id),
		zap.Int("code", code),
		zap.String("message", message))

	response := a2a.JSONRPCErrorResponse{
		ID:      id,
		Jsonrpc: "2.0",
		Error: a2a.JSONRPCError{
			Code:    code,
			Message: message,
		},
	}
	c.JSON(http.StatusOK, response)
}

func detectLanguage(text string) string {
	lowerText := strings.ToLower(text)

	if containsSpanishRequest(lowerText) {
		return "spanish"
	}

	return "english"
}

func normalizeText(text string) string {
	return strings.ToLower(strings.TrimSpace(text))
}

func generateGreeting(messageText string) string {
	logger.Debug("generating greeting", zap.String("input", messageText))

	language := detectLanguage(messageText)
	normalizedText := normalizeText(messageText)

	logger.Debug("language detection",
		zap.String("detected", language),
		zap.String("normalized", normalizedText))

	switch language {
	case "spanish":
		return generateSpanishGreeting(normalizedText, messageText)
	case "english":
		return generateEnglishGreeting(normalizedText, messageText)
	default:
		logger.Warn("unknown language detected, defaulting to english",
			zap.String("language", language))
		return generateEnglishGreeting(normalizedText, messageText)
	}
}

func generateSpanishGreeting(normalizedText, originalText string) string {
	switch normalizedText {
	case "hola", "buenos días", "buenos dias", "buenas tardes", "saludar":
		return "¡Hola, Mundo!"
	case "world", "mundo":
		return "¡Hola, Mundo!"
	default:
		if containsSpanishRequest(originalText) {
			return "¡Hola, " + originalText + "!"
		}
		return "¡Hola, Mundo!"
	}
}

func generateEnglishGreeting(normalizedText, originalText string) string {
	switch normalizedText {
	case "hello", "hi", "hey", "greetings":
		return "Hello, World!"
	case "world":
		return "Hello, World!"
	case "say hello using the hello world agent.", "say hello using the hello world agent":
		return "Hello, World!"
	default:
		if originalText != "" && originalText != "World" {
			return "Hello, " + originalText + "!"
		}
		return "Hello, World!"
	}
}
