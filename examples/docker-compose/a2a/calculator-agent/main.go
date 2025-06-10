package main

import (
	"fmt"
	"log"
	"math"
	"net/http"
	"os"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"

	a2a "github.com/inference-gateway/inference-gateway/a2a"
)

var logger *zap.Logger

func main() {
	var err error
	if os.Getenv("DEBUG") == "true" {
		logger, err = zap.NewDevelopment()
	} else {
		logger, err = zap.NewProduction()
	}
	if err != nil {
		log.Fatal("failed to initialize logger:", err)
	}
	defer logger.Sync()

	logger.Info("starting calculator agent",
		zap.String("version", "1.0.0"),
		zap.String("port", "8080"),
		zap.Bool("debug_mode", os.Getenv("DEBUG") == "true"))

	r := gin.Default()

	r.GET("/health", func(c *gin.Context) {
		// logger.Debug("health check requested")
		c.JSON(http.StatusOK, gin.H{"status": "healthy"})
	})

	r.POST("/a2a", handleA2ARequest)

	r.GET("/.well-known/agent.json", func(c *gin.Context) {
		logger.Debug("agent card requested",
			zap.String("remote_addr", c.ClientIP()))
		streaming := false
		pushNotifications := false
		stateTransitionHistory := false

		info := a2a.AgentCard{
			Name:        "calculator-agent",
			Description: "A mathematical calculator agent that performs basic and advanced calculations",
			URL:         "http://calculator-agent:8080",
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
					ID:          "add",
					Name:        "add",
					Description: "Add two numbers",
					InputModes:  []string{"text"},
					OutputModes: []string{"text"},
				},
				{
					ID:          "subtract",
					Name:        "subtract",
					Description: "Subtract two numbers",
					InputModes:  []string{"text"},
					OutputModes: []string{"text"},
				},
				{
					ID:          "multiply",
					Name:        "multiply",
					Description: "Multiply two numbers",
					InputModes:  []string{"text"},
					OutputModes: []string{"text"},
				},
				{
					ID:          "divide",
					Name:        "divide",
					Description: "Divide two numbers",
					InputModes:  []string{"text"},
					OutputModes: []string{"text"},
				},
				{
					ID:          "power",
					Name:        "power",
					Description: "Raise a number to a power",
					InputModes:  []string{"text"},
					OutputModes: []string{"text"},
				},
				{
					ID:          "sqrt",
					Name:        "sqrt",
					Description: "Calculate square root of a number",
					InputModes:  []string{"text"},
					OutputModes: []string{"text"},
				},
				{
					ID:          "factorial",
					Name:        "factorial",
					Description: "Calculate factorial of a number",
					InputModes:  []string{"text"},
					OutputModes: []string{"text"},
				},
			},
		}
		c.JSON(http.StatusOK, info)
	})

	log.Println("calculator-agent starting on port 8080...")
	if err := r.Run(":8080"); err != nil {
		logger.Fatal("failed to start server", zap.Error(err))
	}
}

func handleA2ARequest(c *gin.Context) {
	var req a2a.JSONRPCRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		logger.Error("failed to parse json-rpc request",
			zap.Error(err),
			zap.String("remote_addr", c.ClientIP()))
		sendError(c, req.ID, -32700, "parse error")
		return
	}

	logger.Debug("received json-rpc request",
		zap.String("method", req.Method),
		zap.String("jsonrpc", req.JSONRPC),
		zap.Any("request_id", req.ID),
		zap.Any("params", req.Params),
		zap.String("remote_addr", c.ClientIP()))

	if req.JSONRPC == "" {
		req.JSONRPC = "2.0"
	}

	if req.ID == nil {
		id := interface{}(uuid.New().String())
		req.ID = &id
		logger.Debug("generated request id", zap.Any("request_id", req.ID))
	}

	switch req.Method {
	case "add":
		logger.Debug("handling add operation", zap.Any("request_id", req.ID))
		handleAdd(c, req)
	case "subtract":
		logger.Debug("handling subtract operation", zap.Any("request_id", req.ID))
		handleSubtract(c, req)
	case "multiply":
		logger.Debug("handling multiply operation", zap.Any("request_id", req.ID))
		handleMultiply(c, req)
	case "divide":
		logger.Debug("handling divide operation", zap.Any("request_id", req.ID))
		handleDivide(c, req)
	case "power":
		logger.Debug("handling power operation", zap.Any("request_id", req.ID))
		handlePower(c, req)
	case "sqrt":
		logger.Debug("handling sqrt operation", zap.Any("request_id", req.ID))
		handleSqrt(c, req)
	case "factorial":
		logger.Debug("handling factorial operation", zap.Any("request_id", req.ID))
		handleFactorial(c, req)
	default:
		logger.Warn("unknown method requested",
			zap.String("method", req.Method),
			zap.Any("request_id", req.ID),
			zap.String("remote_addr", c.ClientIP()))
		sendError(c, req.ID, -32601, "method not found")
	}
}

func handleAdd(c *gin.Context, req a2a.JSONRPCRequest) {
	logger.Debug("processing add operation",
		zap.Any("request_id", req.ID),
		zap.Any("params", req.Params))

	a, b, err := getTwoNumbers(req.Params)
	if err != nil {
		logger.Error("failed to parse operands for add operation",
			zap.Any("request_id", req.ID),
			zap.Error(err),
			zap.Any("params", req.Params))
		sendError(c, req.ID, -32602, err.Error())
		return
	}

	logger.Debug("performing addition calculation",
		zap.Any("request_id", req.ID),
		zap.Float64("operand_a", a),
		zap.Float64("operand_b", b))

	result := a + b

	logger.Debug("addition calculation completed",
		zap.Any("request_id", req.ID),
		zap.Float64("result", result))

	response := a2a.JSONRPCSuccessResponse{
		ID:      req.ID,
		JSONRPC: "2.0",
		Result: map[string]interface{}{
			"operation": "addition",
			"operands":  []float64{a, b},
			"result":    result,
			"agent":     "calculator-agent",
		},
	}

	logger.Debug("sending add operation response",
		zap.Any("request_id", req.ID),
		zap.Float64("result", result))

	c.JSON(http.StatusOK, response)
}

func handleSubtract(c *gin.Context, req a2a.JSONRPCRequest) {
	logger.Debug("processing subtract operation",
		zap.Any("request_id", req.ID),
		zap.Any("params", req.Params))

	a, b, err := getTwoNumbers(req.Params)
	if err != nil {
		logger.Error("failed to parse operands for subtract operation",
			zap.Any("request_id", req.ID),
			zap.Error(err),
			zap.Any("params", req.Params))
		sendError(c, req.ID, -32602, err.Error())
		return
	}

	logger.Debug("performing subtraction calculation",
		zap.Any("request_id", req.ID),
		zap.Float64("operand_a", a),
		zap.Float64("operand_b", b))

	result := a - b

	logger.Debug("subtraction calculation completed",
		zap.Any("request_id", req.ID),
		zap.Float64("result", result))

	response := a2a.JSONRPCSuccessResponse{
		ID:      req.ID,
		JSONRPC: "2.0",
		Result: map[string]interface{}{
			"operation": "subtraction",
			"operands":  []float64{a, b},
			"result":    result,
			"agent":     "calculator-agent",
		},
	}

	logger.Debug("sending subtract operation response",
		zap.Any("request_id", req.ID),
		zap.Float64("result", result))

	c.JSON(http.StatusOK, response)
}

func handleMultiply(c *gin.Context, req a2a.JSONRPCRequest) {
	logger.Debug("processing multiply operation",
		zap.Any("request_id", req.ID),
		zap.Any("params", req.Params))

	a, b, err := getTwoNumbers(req.Params)
	if err != nil {
		logger.Error("failed to parse operands for multiply operation",
			zap.Any("request_id", req.ID),
			zap.Error(err),
			zap.Any("params", req.Params))
		sendError(c, req.ID, -32602, err.Error())
		return
	}

	logger.Debug("performing multiplication calculation",
		zap.Any("request_id", req.ID),
		zap.Float64("operand_a", a),
		zap.Float64("operand_b", b))

	result := a * b

	logger.Debug("multiplication calculation completed",
		zap.Any("request_id", req.ID),
		zap.Float64("result", result))

	response := a2a.JSONRPCSuccessResponse{
		ID:      req.ID,
		JSONRPC: "2.0",
		Result: map[string]interface{}{
			"operation": "multiplication",
			"operands":  []float64{a, b},
			"result":    result,
			"agent":     "calculator-agent",
		},
	}

	logger.Debug("sending multiply operation response",
		zap.Any("request_id", req.ID),
		zap.Float64("result", result))

	c.JSON(http.StatusOK, response)
}

func handleDivide(c *gin.Context, req a2a.JSONRPCRequest) {
	logger.Debug("processing divide operation",
		zap.Any("request_id", req.ID),
		zap.Any("params", req.Params))

	a, b, err := getTwoNumbers(req.Params)
	if err != nil {
		logger.Error("failed to parse operands for divide operation",
			zap.Any("request_id", req.ID),
			zap.Error(err),
			zap.Any("params", req.Params))
		sendError(c, req.ID, -32602, err.Error())
		return
	}

	logger.Debug("performing division calculation",
		zap.Any("request_id", req.ID),
		zap.Float64("dividend", a),
		zap.Float64("divisor", b))

	if b == 0 {
		logger.Error("division by zero attempted",
			zap.Any("request_id", req.ID),
			zap.Float64("dividend", a),
			zap.Float64("divisor", b))
		sendError(c, req.ID, -32603, "division by zero")
		return
	}

	result := a / b

	logger.Debug("division calculation completed",
		zap.Any("request_id", req.ID),
		zap.Float64("result", result))

	response := a2a.JSONRPCSuccessResponse{
		ID:      req.ID,
		JSONRPC: "2.0",
		Result: map[string]interface{}{
			"operation": "division",
			"operands":  []float64{a, b},
			"result":    result,
			"agent":     "calculator-agent",
		},
	}

	logger.Debug("sending divide operation response",
		zap.Any("request_id", req.ID),
		zap.Float64("result", result))

	c.JSON(http.StatusOK, response)
}

func handlePower(c *gin.Context, req a2a.JSONRPCRequest) {
	logger.Debug("processing power operation",
		zap.Any("request_id", req.ID),
		zap.Any("params", req.Params))

	base, exponent, err := getTwoNumbers(req.Params)
	if err != nil {
		logger.Error("failed to parse operands for power operation",
			zap.Any("request_id", req.ID),
			zap.Error(err),
			zap.Any("params", req.Params))
		sendError(c, req.ID, -32602, err.Error())
		return
	}

	logger.Debug("performing power calculation",
		zap.Any("request_id", req.ID),
		zap.Float64("base", base),
		zap.Float64("exponent", exponent))

	result := math.Pow(base, exponent)

	logger.Debug("power calculation completed",
		zap.Any("request_id", req.ID),
		zap.Float64("result", result))

	response := a2a.JSONRPCSuccessResponse{
		ID:      req.ID,
		JSONRPC: "2.0",
		Result: map[string]interface{}{
			"operation": "power",
			"base":      base,
			"exponent":  exponent,
			"result":    result,
			"agent":     "calculator-agent",
		},
	}

	logger.Debug("sending power operation response",
		zap.Any("request_id", req.ID),
		zap.Float64("result", result))

	c.JSON(http.StatusOK, response)
}

func handleSqrt(c *gin.Context, req a2a.JSONRPCRequest) {
	logger.Debug("processing sqrt operation",
		zap.Any("request_id", req.ID),
		zap.Any("params", req.Params))

	number, err := getOneNumber(req.Params)
	if err != nil {
		logger.Error("failed to parse operand for sqrt operation",
			zap.Any("request_id", req.ID),
			zap.Error(err),
			zap.Any("params", req.Params))
		sendError(c, req.ID, -32602, err.Error())
		return
	}

	logger.Debug("performing square root calculation",
		zap.Any("request_id", req.ID),
		zap.Float64("operand", number))

	if number < 0 {
		logger.Error("square root of negative number attempted",
			zap.Any("request_id", req.ID),
			zap.Float64("operand", number))
		sendError(c, req.ID, -32603, "cannot calculate square root of negative number")
		return
	}

	result := math.Sqrt(number)

	logger.Debug("square root calculation completed",
		zap.Any("request_id", req.ID),
		zap.Float64("result", result))

	response := a2a.JSONRPCSuccessResponse{
		ID:      req.ID,
		JSONRPC: "2.0",
		Result: map[string]interface{}{
			"operation": "square root",
			"operand":   number,
			"result":    result,
			"agent":     "calculator-agent",
		},
	}

	logger.Debug("sending sqrt operation response",
		zap.Any("request_id", req.ID),
		zap.Float64("result", result))

	c.JSON(http.StatusOK, response)
}

func handleFactorial(c *gin.Context, req a2a.JSONRPCRequest) {
	logger.Debug("processing factorial operation",
		zap.Any("request_id", req.ID),
		zap.Any("params", req.Params))

	number, err := getOneNumber(req.Params)
	if err != nil {
		logger.Error("failed to parse operand for factorial operation",
			zap.Any("request_id", req.ID),
			zap.Error(err),
			zap.Any("params", req.Params))
		sendError(c, req.ID, -32602, err.Error())
		return
	}

	logger.Debug("performing factorial calculation",
		zap.Any("request_id", req.ID),
		zap.Float64("operand", number))

	if number < 0 || number != math.Floor(number) {
		logger.Error("factorial of non-integer or negative number attempted",
			zap.Any("request_id", req.ID),
			zap.Float64("operand", number),
			zap.Bool("is_negative", number < 0),
			zap.Bool("is_integer", number == math.Floor(number)))
		sendError(c, req.ID, -32603, "factorial requires a non-negative integer")
		return
	}

	result := factorial(int(number))

	logger.Debug("factorial calculation completed",
		zap.Any("request_id", req.ID),
		zap.Int64("result", result))

	response := a2a.JSONRPCSuccessResponse{
		ID:      req.ID,
		JSONRPC: "2.0",
		Result: map[string]interface{}{
			"operation": "factorial",
			"operand":   int(number),
			"result":    result,
			"agent":     "calculator-agent",
		},
	}

	logger.Debug("sending factorial operation response",
		zap.Any("request_id", req.ID),
		zap.Int64("result", result))

	c.JSON(http.StatusOK, response)
}

func getTwoNumbers(params map[string]interface{}) (float64, float64, error) {
	logger.Debug("parsing two number parameters", zap.Any("params", params))

	a, ok := params["a"]
	if !ok {
		logger.Debug("parameter 'a' missing from request")
		return 0, 0, fmt.Errorf("parameter 'a' is required")
	}

	b, ok := params["b"]
	if !ok {
		logger.Debug("parameter 'b' missing from request")
		return 0, 0, fmt.Errorf("parameter 'b' is required")
	}

	numA, err := toFloat64(a)
	if err != nil {
		logger.Debug("failed to convert parameter 'a' to number",
			zap.Any("value", a),
			zap.Error(err))
		return 0, 0, fmt.Errorf("parameter 'a' must be a number")
	}

	numB, err := toFloat64(b)
	if err != nil {
		logger.Debug("failed to convert parameter 'b' to number",
			zap.Any("value", b),
			zap.Error(err))
		return 0, 0, fmt.Errorf("parameter 'b' must be a number")
	}

	logger.Debug("successfully parsed two numbers",
		zap.Float64("a", numA),
		zap.Float64("b", numB))

	return numA, numB, nil
}

func getOneNumber(params map[string]interface{}) (float64, error) {
	logger.Debug("parsing single number parameter", zap.Any("params", params))

	number, ok := params["number"]
	if !ok {
		logger.Debug("parameter 'number' missing from request")
		return 0, fmt.Errorf("parameter 'number' is required")
	}

	num, err := toFloat64(number)
	if err != nil {
		logger.Debug("failed to convert parameter 'number' to number",
			zap.Any("value", number),
			zap.Error(err))
		return 0, fmt.Errorf("parameter 'number' must be a number")
	}

	logger.Debug("successfully parsed single number",
		zap.Float64("number", num))

	return num, nil
}

func toFloat64(value interface{}) (float64, error) {
	logger.Debug("converting value to float64",
		zap.Any("value", value),
		zap.String("type", fmt.Sprintf("%T", value)))

	switch v := value.(type) {
	case float64:
		logger.Debug("value is already float64", zap.Float64("result", v))
		return v, nil
	case float32:
		result := float64(v)
		logger.Debug("converted float32 to float64", zap.Float64("result", result))
		return result, nil
	case int:
		result := float64(v)
		logger.Debug("converted int to float64", zap.Float64("result", result))
		return result, nil
	case int64:
		result := float64(v)
		logger.Debug("converted int64 to float64", zap.Float64("result", result))
		return result, nil
	case string:
		result, err := strconv.ParseFloat(v, 64)
		if err != nil {
			logger.Debug("failed to parse string to float64",
				zap.String("string_value", v),
				zap.Error(err))
			return 0, err
		}
		logger.Debug("converted string to float64", zap.Float64("result", result))
		return result, nil
	default:
		logger.Debug("unsupported type for float64 conversion",
			zap.Any("value", value),
			zap.String("type", fmt.Sprintf("%T", value)))
		return 0, fmt.Errorf("cannot convert to number")
	}
}

func factorial(n int) int64 {
	logger.Debug("calculating factorial",
		zap.Int("input", n))

	if n <= 1 {
		logger.Debug("factorial base case reached",
			zap.Int("input", n),
			zap.Int64("result", 1))
		return 1
	}

	result := int64(1)
	for i := 2; i <= n; i++ {
		result *= int64(i)
	}

	logger.Debug("factorial calculation completed",
		zap.Int("input", n),
		zap.Int64("result", result))

	return result
}

func sendError(c *gin.Context, id interface{}, code int, message string) {
	logger.Error("sending error response",
		zap.Any("request_id", id),
		zap.Int("error_code", code),
		zap.String("error_message", message),
		zap.String("remote_addr", c.ClientIP()))

	response := a2a.JSONRPCErrorResponse{
		ID:      id,
		JSONRPC: "2.0",
		Error: a2a.JSONRPCError{
			Code:    code,
			Message: message,
		},
	}
	c.JSON(http.StatusOK, response)
}
