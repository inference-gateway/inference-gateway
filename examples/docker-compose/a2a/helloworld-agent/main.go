package main

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	a2a "github.com/inference-gateway/inference-gateway/a2a"
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
			Description: "A simple greeting agent that provides personalized greetings",
			URL:         "http://localhost:8081",
			Version:     "1.0.0",
			Capabilities: a2a.AgentCapabilities{
				Streaming:              false,
				Pushnotifications:      false,
				Statetransitionhistory: false,
			},
			Defaultinputmodes:  []string{"text"},
			Defaultoutputmodes: []string{"text"},
			Skills: []a2a.AgentSkill{
				{
					ID:          "greet",
					Name:        "greet",
					Description: "Provide a personalized greeting",
					Inputmodes:  []string{"text"},
					Outputmodes: []string{"text"},
				},
				{
					ID:          "introduce",
					Name:        "introduce",
					Description: "Introduce the agent and its capabilities",
					Inputmodes:  []string{"text"},
					Outputmodes: []string{"text"},
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

func handleA2ARequest(c *gin.Context) {
	var req a2a.JSONRPCRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		sendError(c, req.ID, -32700, "parse error")
		return
	}

	// Set JSON-RPC version if not provided
	if req.JSONRPC == "" {
		req.JSONRPC = "2.0"
	}

	// Generate ID if not provided
	if req.ID == nil {
		req.ID = uuid.New().String()
	}

	switch req.Method {
	case "greet":
		handleGreet(c, req)
	case "introduce":
		handleIntroduce(c, req)
	default:
		sendError(c, req.ID, -32601, "method not found")
	}
}

func handleGreet(c *gin.Context, req a2a.JSONRPCRequest) {
	name, ok := req.Params["name"].(string)
	if !ok {
		name = "World"
	}

	language, ok := req.Params["language"].(string)
	if !ok {
		language = "en"
	}

	var greeting string
	switch language {
	case "es":
		greeting = "¡Hola, " + name + "!"
	case "fr":
		greeting = "Bonjour, " + name + "!"
	case "de":
		greeting = "Hallo, " + name + "!"
	case "ja":
		greeting = "こんにちは、" + name + "さん！"
	default:
		greeting = "Hello, " + name + "!"
	}

	response := a2a.JSONRPCSuccessResponse{
		ID:      req.ID,
		JSONRPC: "2.0",
		Result: map[string]interface{}{
			"greeting": greeting,
			"language": language,
			"agent":    "helloworld-agent",
		},
	}

	c.JSON(http.StatusOK, response)
}

func handleIntroduce(c *gin.Context, req a2a.JSONRPCRequest) {
	response := a2a.JSONRPCSuccessResponse{
		ID:      req.ID,
		JSONRPC: "2.0",
		Result: map[string]interface{}{
			"introduction": "I am the HelloWorld Agent. I can greet you in multiple languages including English, Spanish, French, German, and Japanese. Just call the 'greet' method with your name and preferred language!",
			"capabilities": []string{
				"Multi-language greetings",
				"Personalized messages",
				"Friendly responses",
			},
			"agent": "helloworld-agent",
		},
	}

	c.JSON(http.StatusOK, response)
}

func sendError(c *gin.Context, id interface{}, code int, message string) {
	response := a2a.JSONRPCErrorResponse{
		ID:      id,
		JSONRPC: "2.0",
		Error: &a2a.JSONRPCError{
			Code:    code,
			Message: message,
		},
	}
	c.JSON(http.StatusOK, response)
}
