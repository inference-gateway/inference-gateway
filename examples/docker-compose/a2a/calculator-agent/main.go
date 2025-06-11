package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strconv"

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
			Name:        "add",
			Description: stringPtr("Add two numbers together"),
			Parameters: &sdk.FunctionParameters{
				"type": "object",
				"properties": map[string]interface{}{
					"a": map[string]interface{}{
						"type":        "number",
						"description": "The first number to add",
					},
					"b": map[string]interface{}{
						"type":        "number",
						"description": "The second number to add",
					},
				},
				"required": []string{"a", "b"},
			},
		},
	},
	{
		Type: "function",
		Function: sdk.FunctionObject{
			Name:        "subtract",
			Description: stringPtr("Subtract the second number from the first number"),
			Parameters: &sdk.FunctionParameters{
				"type": "object",
				"properties": map[string]interface{}{
					"a": map[string]interface{}{
						"type":        "number",
						"description": "The number to subtract from",
					},
					"b": map[string]interface{}{
						"type":        "number",
						"description": "The number to subtract",
					},
				},
				"required": []string{"a", "b"},
			},
		},
	},
	{
		Type: "function",
		Function: sdk.FunctionObject{
			Name:        "multiply",
			Description: stringPtr("Multiply two numbers together"),
			Parameters: &sdk.FunctionParameters{
				"type": "object",
				"properties": map[string]interface{}{
					"a": map[string]interface{}{
						"type":        "number",
						"description": "The first number to multiply",
					},
					"b": map[string]interface{}{
						"type":        "number",
						"description": "The second number to multiply",
					},
				},
				"required": []string{"a", "b"},
			},
		},
	},
	{
		Type: "function",
		Function: sdk.FunctionObject{
			Name:        "divide",
			Description: stringPtr("Divide the first number by the second number"),
			Parameters: &sdk.FunctionParameters{
				"type": "object",
				"properties": map[string]interface{}{
					"a": map[string]interface{}{
						"type":        "number",
						"description": "The dividend (number to be divided)",
					},
					"b": map[string]interface{}{
						"type":        "number",
						"description": "The divisor (number to divide by)",
					},
				},
				"required": []string{"a", "b"},
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
			Name:        "calculator-agent",
			Description: "A mathematical calculator agent that performs basic arithmetic operations using the A2A protocol",
			URL:         "http://calculator-agent:8080",
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
					ID:          "arithmetic",
					Name:        "arithmetic",
					Description: "Perform basic arithmetic operations including addition, subtraction, multiplication, and division",
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

	logger.Info("starting calculator agent",
		zap.String("version", "1.0.0"),
		zap.String("port", cfg.Port),
		zap.String("inference_gateway_url", cfg.InferenceGatewayURL),
		zap.String("llm_provider", cfg.LLMProvider),
		zap.String("llm_model", cfg.LLMModel),
		zap.Bool("debug_mode", cfg.Debug))

	router := setupRouter(logger, client)

	logger.Info("calculator-agent starting on port 8080...")
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
	case "tasks/get":
		handleTaskGet(c, req, logger, client)
	case "tasks/cancel":
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

	var result string
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
			result = response.Choices[0].Message.Content
			break
		}

		assistantMessage := sdk.Message{
			Role:      sdk.Assistant,
			Content:   response.Choices[0].Message.Content,
			ToolCalls: toolCalls,
		}
		messages = append(messages, assistantMessage)

		for _, toolCall := range *toolCalls {
			var toolResult string

			switch toolCall.Function.Name {
			case "add":
				var params CalculatorParams
				if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &params); err != nil {
					logger.Error("failed to unmarshal add parameters", zap.Error(err))
					sendError(c, req.ID, int(ErrInvalidParams), "invalid parameters for add function", logger)
					return
				}
				toolResult = add(params.A, params.B)

			case "subtract":
				var params CalculatorParams
				if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &params); err != nil {
					logger.Error("failed to unmarshal subtract parameters", zap.Error(err))
					sendError(c, req.ID, int(ErrInvalidParams), "invalid parameters for subtract function", logger)
					return
				}
				toolResult = subtract(params.A, params.B)

			case "multiply":
				var params CalculatorParams
				if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &params); err != nil {
					logger.Error("failed to unmarshal multiply parameters", zap.Error(err))
					sendError(c, req.ID, int(ErrInvalidParams), "invalid parameters for multiply function", logger)
					return
				}
				toolResult = multiply(params.A, params.B)

			case "divide":
				var params CalculatorParams
				if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &params); err != nil {
					logger.Error("failed to unmarshal divide parameters", zap.Error(err))
					sendError(c, req.ID, int(ErrInvalidParams), "invalid parameters for divide function", logger)
					return
				}
				toolResult = divide(params.A, params.B)

			default:
				logger.Debug("ignoring unknown tool call", zap.String("name", toolCall.Function.Name))
				continue
			}

			toolResponse := sdk.Message{
				Role:       sdk.Tool,
				Content:    toolResult,
				ToolCallId: &toolCall.Id,
			}

			messages = append(messages, toolResponse)
		}

		iteration++
	}

	if result == "" {
		result = response.Choices[0].Message.Content
	}

	logger.Debug("calculation result generated", zap.String("result", result))

	c.JSON(http.StatusOK, a2a.JSONRPCSuccessResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  result,
	})
}

func handleMessageStream(c *gin.Context, req a2a.JSONRPCRequest, logger *zap.Logger, client sdk.Client) {
	logger.Info("streaming not implemented yet")
	sendError(c, req.ID, int(ErrServerError), "streaming not implemented", logger)
}

func handleTaskGet(c *gin.Context, req a2a.JSONRPCRequest, logger *zap.Logger, client sdk.Client) {
	logger.Info("tasks/get not implemented yet")
	sendError(c, req.ID, int(ErrServerError), "tasks/get not implemented", logger)
}

func handleTaskCancel(c *gin.Context, req a2a.JSONRPCRequest, logger *zap.Logger, client sdk.Client) {
	logger.Info("tasks/cancel not implemented yet")
	sendError(c, req.ID, int(ErrServerError), "tasks/cancel not implemented", logger)
}

func stringPtr(s string) *string {
	return &s
}

type CalculatorParams struct {
	A float64 `json:"a"`
	B float64 `json:"b"`
}

func add(a, b float64) string {
	result := a + b
	return strconv.FormatFloat(result, 'f', -1, 64)
}

func subtract(a, b float64) string {
	result := a - b
	return strconv.FormatFloat(result, 'f', -1, 64)
}

func multiply(a, b float64) string {
	result := a * b
	return strconv.FormatFloat(result, 'f', -1, 64)
}

func divide(a, b float64) string {
	if b == 0 {
		return "error: division by zero"
	}
	result := a / b
	return strconv.FormatFloat(result, 'f', -1, 64)
}
