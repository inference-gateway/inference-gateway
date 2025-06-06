package main

import (
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"helloworld-agent/a2a"
)

func main() {
	r := gin.Default()

	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "healthy"})
	})

	r.POST("/a2a", handleA2ARequest)

	r.GET("/.well-known/agent.json", func(c *gin.Context) {
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

	log.Println("helloworld-agent starting on port 8081...")
	if err := r.Run(":8081"); err != nil {
		log.Fatal("failed to start server:", err)
	}
}

func containsSpanishRequest(text string) bool {
	lowerText := strings.ToLower(text)
	spanishKeywords := []string{
		"spanish", "español", "espanol", "spanish", "en español",
		"en espanol", "greet me in spanish", "greeting in spanish",
		"hola", "buenos días", "buenas tardes", "saludar",
	}

	for _, keyword := range spanishKeywords {
		if strings.Contains(lowerText, keyword) {
			return true
		}
	}
	return false
}

func handleA2ARequest(c *gin.Context) {
	var req a2a.JSONRPCRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		sendError(c, req.ID, -32700, "parse error")
		return
	}

	if req.Jsonrpc == "" {
		req.Jsonrpc = "2.0"
	}

	if req.ID == nil {
		req.ID = uuid.New().String()
	}

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
		sendError(c, req.ID, -32601, "method not found")
	}
}

func handleMessageSend(c *gin.Context, req a2a.JSONRPCRequest) {
	paramsMap, ok := req.Params["message"].(map[string]interface{})
	if !ok {
		sendError(c, req.ID, -32602, "invalid params: missing message")
		return
	}

	partsArray, ok := paramsMap["parts"].([]interface{})
	if !ok {
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

	var greeting string
	if containsSpanishRequest(messageText) {
		if messageText == "Hola" || messageText == "hola" ||
			messageText == "Buenos días" || messageText == "buenos días" ||
			messageText == "Buenas tardes" || messageText == "buenas tardes" ||
			containsSpanishRequest(messageText) {
			greeting = "¡Hola, Mundo!"
		} else {
			greeting = "¡Hola, " + messageText + "!"
		}
	} else if messageText == "Hello" || messageText == "hello" ||
		messageText == "Hi" || messageText == "hi" ||
		messageText == "Say hello using the hello world agent." {
		greeting = "Hello, World!"
	} else {
		greeting = "Hello, " + messageText + "!"
	}

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

	c.JSON(http.StatusOK, response)
}

func handleMessageStream(c *gin.Context, req a2a.JSONRPCRequest) {
	handleMessageSend(c, req)
}

func handleTaskGet(c *gin.Context, req a2a.JSONRPCRequest) {
	sendError(c, req.ID, -32601, "task/get not implemented")
}

func handleTaskCancel(c *gin.Context, req a2a.JSONRPCRequest) {
	sendError(c, req.ID, -32601, "task/cancel not implemented")
}

func getStringParam(params map[string]interface{}, key string, defaultValue string) string {
	if value, exists := params[key]; exists {
		if str, ok := value.(string); ok {
			return str
		}
	}
	return defaultValue
}

func sendError(c *gin.Context, id interface{}, code int, message string) {
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
