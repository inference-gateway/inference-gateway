package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"

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
			Name:        "greet",
			Description: stringPtr("Greet the user in a specified language"),
			Parameters: &sdk.FunctionParameters{
				"type": "object",
				"properties": map[string]interface{}{
					"language": map[string]interface{}{
						"type":        "string",
						"description": "The language to greet in (e.g., 'en' for English, 'es' for Spanish)",
						"enum":        []string{"en", "es", "fr", "de", "zh"},
					},
					"name": map[string]interface{}{
						"type":        "string",
						"description": "The name of the person to greet",
					},
				},
				"required": []string{"language", "name"},
			},
		},
	},
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
		streaming := false
		pushNotifications := false
		stateTransitionHistory := false

		info := a2a.AgentCard{
			Name:        "helloworld-agent",
			Description: "A simple greeting agent that provides personalized greetings using the A2A protocol",
			URL:         "http://helloworld-agent:8080",
			Version:     "1.0.0",
			Capabilities: a2a.AgentCapabilities{
				Streaming:              &streaming,
				PushNotifications:      &pushNotifications,
				StateTransitionHistory: &stateTransitionHistory,
			},
			DefaultInputModes:  []string{"text/plain"},
			DefaultOutputModes: []string{"text/plain"},
			Skills: []a2a.AgentSkill{
				{
					ID:          "greeting",
					Name:        "greeting",
					Description: "Provide personalized greetings in multiple languages",
					InputModes:  []string{"text/plain"},
					OutputModes: []string{"text/plain"},
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

	logger.Info("starting helloworld agent",
		zap.String("version", "1.0.0"),
		zap.String("port", cfg.Port),
		zap.String("inference_gateway_url", cfg.InferenceGatewayURL),
		zap.String("llm_provider", cfg.LLMProvider),
		zap.String("llm_model", cfg.LLMModel),
		zap.Bool("debug_mode", cfg.Debug))

	router := setupRouter(logger, client)

	logger.Info("helloworld-agent starting on port 8080...")
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

func sendSuccess(c *gin.Context, id interface{}, result interface{}) {
	resp := a2a.JSONRPCSuccessResponse{
		JSONRPC: "2.0",
		ID:      id,
		Result:  result,
	}
	c.JSON(http.StatusOK, resp)
	logger.Info("sending success response", zap.Any("id", id), zap.Any("result", result))
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

	response, err := client.WithTools(&tools).WithHeader("X-A2A-Internal", "true").GenerateContent(context.Background(), sdk.Provider(cfg.LLMProvider), cfg.LLMModel, messages)
	if err != nil {
		logger.Error("failed to create chat completion", zap.Error(err))
		sendError(c, req.ID, int(ErrServerError), "server error", logger)
		return
	}

	if len(response.Choices) == 0 {
		logger.Error("no choices returned from chat completion")
		sendError(c, req.ID, int(ErrServerError), "no choices returned", logger)
		return
	}

	var greeting string
	var iteration int
	for iteration < cfg.MaxIterations {
		response, err = client.WithTools(&tools).WithHeader("X-A2A-Internal", "true").GenerateContent(context.Background(), sdk.Provider(cfg.LLMProvider), cfg.LLMModel, messages)
		if err != nil {
			logger.Error("failed to create chat completion", zap.Error(err))
			sendError(c, req.ID, int(ErrServerError), "server error", logger)
			return
		}

		if len(response.Choices) == 0 {
			logger.Error("no choices returned from chat completion")
			sendError(c, req.ID, int(ErrServerError), "no choices returned", logger)
			return
		}

		toolCalls := response.Choices[0].Message.ToolCalls

		if toolCalls == nil || len(*toolCalls) == 0 {
			logger.Debug("no tool calls found in response, using text content")
			break
		}

		assistantMessage := sdk.Message{
			Role:      sdk.Assistant,
			Content:   response.Choices[0].Message.Content,
			ToolCalls: toolCalls,
		}
		messages = append(messages, assistantMessage)

		for _, toolCall := range *toolCalls {
			if toolCall.Function.Name != "greet" {
				logger.Debug("ignoring tool call", zap.String("name", toolCall.Function.Name))
				continue
			}

			var greetParams GreetParams
			if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &greetParams); err != nil {
				logger.Error("failed to unmarshal greet parameters", zap.Error(err))
				sendError(c, req.ID, int(ErrInvalidParams), "invalid parameters for greet function", logger)
				return
			}

			greeting = greet(greetParams.Language, greetParams.Name)

			toolResponse := sdk.Message{
				Role:       sdk.Tool,
				Content:    greeting,
				ToolCallId: &toolCall.Id,
			}

			messages = append(messages, toolResponse)
		}

		iteration++
	}

	logger.Debug("greeting generated", zap.String("greeting", greeting))

	c.JSON(http.StatusOK, a2a.JSONRPCSuccessResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  greeting,
	})
}

func handleMessageStream(c *gin.Context, req a2a.JSONRPCRequest, logger *zap.Logger, client sdk.Client) {
	logger.Info("streaming not implemented yet")
	sendError(c, req.ID, int(ErrServerError), "streaming not implemented", logger)
}

func handleTaskGet(c *gin.Context, req a2a.JSONRPCRequest, logger *zap.Logger, client sdk.Client) {
	logger.Info("task/get not implemented yet")
	sendError(c, req.ID, int(ErrServerError), "task/get not implemented", logger)
}

func handleTaskCancel(c *gin.Context, req a2a.JSONRPCRequest, logger *zap.Logger, client sdk.Client) {
	logger.Info("task/cancel not implemented yet")
	sendError(c, req.ID, int(ErrServerError), "task/cancel not implemented", logger)
}

func stringPtr(s string) *string {
	return &s
}

type GreetParams struct {
	Language string `json:"language"`
	Name     string `json:"name"`
}

func greet(language, name string) string {
	switch language {
	case "en":
		return "Hello, " + name
	case "es":
		return "Hola, " + name
	case "fr":
		return "Bonjour, " + name
	case "de":
		return "Hallo, " + name
	case "zh":
		return "你好, " + name
	default:
		return "Hello, " + name
	}
}
