package main

import (
	"fmt"
	"log"
	"math"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	a2a "calculator-agent/a2a"
)

func main() {
	r := gin.Default()

	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "healthy"})
	})

	r.POST("/a2a", handleA2ARequest)

	r.GET("/.well-known/agent.json", func(c *gin.Context) {
		info := a2a.AgentCard{
			Name:        "calculator-agent",
			Description: "A mathematical calculator agent that performs basic and advanced calculations",
			URL:         "http://localhost:8082",
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
					ID:          "add",
					Name:        "add",
					Description: "Add two numbers",
					Inputmodes:  []string{"text"},
					Outputmodes: []string{"text"},
				},
				{
					ID:          "subtract",
					Name:        "subtract",
					Description: "Subtract two numbers",
					Inputmodes:  []string{"text"},
					Outputmodes: []string{"text"},
				},
				{
					ID:          "multiply",
					Name:        "multiply",
					Description: "Multiply two numbers",
					Inputmodes:  []string{"text"},
					Outputmodes: []string{"text"},
				},
				{
					ID:          "divide",
					Name:        "divide",
					Description: "Divide two numbers",
					Inputmodes:  []string{"text"},
					Outputmodes: []string{"text"},
				},
				{
					ID:          "power",
					Name:        "power",
					Description: "Raise a number to a power",
					Inputmodes:  []string{"text"},
					Outputmodes: []string{"text"},
				},
				{
					ID:          "sqrt",
					Name:        "sqrt",
					Description: "Calculate square root of a number",
					Inputmodes:  []string{"text"},
					Outputmodes: []string{"text"},
				},
				{
					ID:          "factorial",
					Name:        "factorial",
					Description: "Calculate factorial of a number",
					Inputmodes:  []string{"text"},
					Outputmodes: []string{"text"},
				},
			},
		}
		c.JSON(http.StatusOK, info)
	})

	log.Println("calculator-agent starting on port 8082...")
	if err := r.Run(":8082"); err != nil {
		log.Fatal("failed to start server:", err)
	}
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
	case "add":
		handleAdd(c, req)
	case "subtract":
		handleSubtract(c, req)
	case "multiply":
		handleMultiply(c, req)
	case "divide":
		handleDivide(c, req)
	case "power":
		handlePower(c, req)
	case "sqrt":
		handleSqrt(c, req)
	case "factorial":
		handleFactorial(c, req)
	default:
		sendError(c, req.ID, -32601, "method not found")
	}
}

func handleAdd(c *gin.Context, req a2a.JSONRPCRequest) {
	a, b, err := getTwoNumbers(req.Params)
	if err != nil {
		sendError(c, req.ID, -32602, err.Error())
		return
	}

	result := a + b
	response := a2a.JSONRPCSuccessResponse{
		ID:      req.ID,
		Jsonrpc: "2.0",
		Result: map[string]interface{}{
			"operation": "addition",
			"operands":  []float64{a, b},
			"result":    result,
			"agent":     "calculator-agent",
		},
	}

	c.JSON(http.StatusOK, response)
}

func handleSubtract(c *gin.Context, req a2a.JSONRPCRequest) {
	a, b, err := getTwoNumbers(req.Params)
	if err != nil {
		sendError(c, req.ID, -32602, err.Error())
		return
	}

	result := a - b
	response := a2a.JSONRPCSuccessResponse{
		ID:      req.ID,
		Jsonrpc: "2.0",
		Result: map[string]interface{}{
			"operation": "subtraction",
			"operands":  []float64{a, b},
			"result":    result,
			"agent":     "calculator-agent",
		},
	}

	c.JSON(http.StatusOK, response)
}

func handleMultiply(c *gin.Context, req a2a.JSONRPCRequest) {
	a, b, err := getTwoNumbers(req.Params)
	if err != nil {
		sendError(c, req.ID, -32602, err.Error())
		return
	}

	result := a * b
	response := a2a.JSONRPCSuccessResponse{
		ID:      req.ID,
		Jsonrpc: "2.0",
		Result: map[string]interface{}{
			"operation": "multiplication",
			"operands":  []float64{a, b},
			"result":    result,
			"agent":     "calculator-agent",
		},
	}

	c.JSON(http.StatusOK, response)
}

func handleDivide(c *gin.Context, req a2a.JSONRPCRequest) {
	a, b, err := getTwoNumbers(req.Params)
	if err != nil {
		sendError(c, req.ID, -32602, err.Error())
		return
	}

	if b == 0 {
		sendError(c, req.ID, -32603, "division by zero")
		return
	}

	result := a / b
	response := a2a.JSONRPCSuccessResponse{
		ID:      req.ID,
		Jsonrpc: "2.0",
		Result: map[string]interface{}{
			"operation": "division",
			"operands":  []float64{a, b},
			"result":    result,
			"agent":     "calculator-agent",
		},
	}

	c.JSON(http.StatusOK, response)
}

func handlePower(c *gin.Context, req a2a.JSONRPCRequest) {
	base, exponent, err := getTwoNumbers(req.Params)
	if err != nil {
		sendError(c, req.ID, -32602, err.Error())
		return
	}

	result := math.Pow(base, exponent)
	response := a2a.JSONRPCSuccessResponse{
		ID:      req.ID,
		Jsonrpc: "2.0",
		Result: map[string]interface{}{
			"operation": "power",
			"base":      base,
			"exponent":  exponent,
			"result":    result,
			"agent":     "calculator-agent",
		},
	}

	c.JSON(http.StatusOK, response)
}

func handleSqrt(c *gin.Context, req a2a.JSONRPCRequest) {
	number, err := getOneNumber(req.Params)
	if err != nil {
		sendError(c, req.ID, -32602, err.Error())
		return
	}

	if number < 0 {
		sendError(c, req.ID, -32603, "cannot calculate square root of negative number")
		return
	}

	result := math.Sqrt(number)
	response := a2a.JSONRPCSuccessResponse{
		ID:      req.ID,
		Jsonrpc: "2.0",
		Result: map[string]interface{}{
			"operation": "square root",
			"operand":   number,
			"result":    result,
			"agent":     "calculator-agent",
		},
	}

	c.JSON(http.StatusOK, response)
}

func handleFactorial(c *gin.Context, req a2a.JSONRPCRequest) {
	number, err := getOneNumber(req.Params)
	if err != nil {
		sendError(c, req.ID, -32602, err.Error())
		return
	}

	if number < 0 || number != math.Floor(number) {
		sendError(c, req.ID, -32603, "factorial requires a non-negative integer")
		return
	}

	result := factorial(int(number))
	response := a2a.JSONRPCSuccessResponse{
		ID:      req.ID,
		Jsonrpc: "2.0",
		Result: map[string]interface{}{
			"operation": "factorial",
			"operand":   int(number),
			"result":    result,
			"agent":     "calculator-agent",
		},
	}

	c.JSON(http.StatusOK, response)
}

func getTwoNumbers(params map[string]interface{}) (float64, float64, error) {
	a, ok := params["a"]
	if !ok {
		return 0, 0, fmt.Errorf("parameter 'a' is required")
	}

	b, ok := params["b"]
	if !ok {
		return 0, 0, fmt.Errorf("parameter 'b' is required")
	}

	numA, err := toFloat64(a)
	if err != nil {
		return 0, 0, fmt.Errorf("parameter 'a' must be a number")
	}

	numB, err := toFloat64(b)
	if err != nil {
		return 0, 0, fmt.Errorf("parameter 'b' must be a number")
	}

	return numA, numB, nil
}

func getOneNumber(params map[string]interface{}) (float64, error) {
	number, ok := params["number"]
	if !ok {
		return 0, fmt.Errorf("parameter 'number' is required")
	}

	num, err := toFloat64(number)
	if err != nil {
		return 0, fmt.Errorf("parameter 'number' must be a number")
	}

	return num, nil
}

func toFloat64(value interface{}) (float64, error) {
	switch v := value.(type) {
	case float64:
		return v, nil
	case float32:
		return float64(v), nil
	case int:
		return float64(v), nil
	case int64:
		return float64(v), nil
	case string:
		return strconv.ParseFloat(v, 64)
	default:
		return 0, fmt.Errorf("cannot convert to number")
	}
}

func factorial(n int) int64 {
	if n <= 1 {
		return 1
	}
	result := int64(1)
	for i := 2; i <= n; i++ {
		result *= int64(i)
	}
	return result
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
