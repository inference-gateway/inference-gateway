package main

import (
	"encoding/json"
	"strconv"

	sdk "github.com/inference-gateway/sdk"
	zap "go.uber.org/zap"
)

// Calculator service interface
type CalculatorService interface {
	Add(a, b float64) float64
	Subtract(a, b float64) float64
	Multiply(a, b float64) float64
	Divide(a, b float64) (float64, error)
}

// Mock calculator service implementation
type MockCalculatorService struct {
	logger *zap.Logger
}

func NewMockCalculatorService(logger *zap.Logger) *MockCalculatorService {
	return &MockCalculatorService{
		logger: logger,
	}
}

func (s *MockCalculatorService) Add(a, b float64) float64 {
	result := a + b
	s.logger.Debug("performed addition", zap.Float64("a", a), zap.Float64("b", b), zap.Float64("result", result))
	return result
}

func (s *MockCalculatorService) Subtract(a, b float64) float64 {
	result := a - b
	s.logger.Debug("performed subtraction", zap.Float64("a", a), zap.Float64("b", b), zap.Float64("result", result))
	return result
}

func (s *MockCalculatorService) Multiply(a, b float64) float64 {
	result := a * b
	s.logger.Debug("performed multiplication", zap.Float64("a", a), zap.Float64("b", b), zap.Float64("result", result))
	return result
}

func (s *MockCalculatorService) Divide(a, b float64) (float64, error) {
	if b == 0 {
		s.logger.Error("division by zero attempted", zap.Float64("a", a), zap.Float64("b", b))
		return 0, NewCalculatorError("division by zero")
	}
	result := a / b
	s.logger.Debug("performed division", zap.Float64("a", a), zap.Float64("b", b), zap.Float64("result", result))
	return result, nil
}

// Calculator error type
type CalculatorError struct {
	Message string
}

func NewCalculatorError(message string) *CalculatorError {
	return &CalculatorError{Message: message}
}

func (e *CalculatorError) Error() string {
	return e.Message
}

// Calculator parameters
type CalculatorParams struct {
	A float64 `json:"a"`
	B float64 `json:"b"`
}

// Calculator tool handler
type CalculatorToolHandler struct {
	service CalculatorService
	logger  *zap.Logger
}

func NewCalculatorToolHandler(service CalculatorService, logger *zap.Logger) *CalculatorToolHandler {
	return &CalculatorToolHandler{
		service: service,
		logger:  logger,
	}
}

func (h *CalculatorToolHandler) HandleToolCall(toolCall sdk.ChatCompletionMessageToolCall) (string, error) {
	switch toolCall.Function.Name {
	case "add":
		return h.handleAdd(toolCall)
	case "subtract":
		return h.handleSubtract(toolCall)
	case "multiply":
		return h.handleMultiply(toolCall)
	case "divide":
		return h.handleDivide(toolCall)
	default:
		h.logger.Warn("unknown tool call", zap.String("function", toolCall.Function.Name))
		return "", NewCalculatorError("unknown function: " + toolCall.Function.Name)
	}
}

func (h *CalculatorToolHandler) handleAdd(toolCall sdk.ChatCompletionMessageToolCall) (string, error) {
	var params CalculatorParams
	if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &params); err != nil {
		h.logger.Error("failed to unmarshal add parameters", zap.Error(err))
		return "", err
	}

	result := h.service.Add(params.A, params.B)
	return strconv.FormatFloat(result, 'f', -1, 64), nil
}

func (h *CalculatorToolHandler) handleSubtract(toolCall sdk.ChatCompletionMessageToolCall) (string, error) {
	var params CalculatorParams
	if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &params); err != nil {
		h.logger.Error("failed to unmarshal subtract parameters", zap.Error(err))
		return "", err
	}

	result := h.service.Subtract(params.A, params.B)
	return strconv.FormatFloat(result, 'f', -1, 64), nil
}

func (h *CalculatorToolHandler) handleMultiply(toolCall sdk.ChatCompletionMessageToolCall) (string, error) {
	var params CalculatorParams
	if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &params); err != nil {
		h.logger.Error("failed to unmarshal multiply parameters", zap.Error(err))
		return "", err
	}

	result := h.service.Multiply(params.A, params.B)
	return strconv.FormatFloat(result, 'f', -1, 64), nil
}

func (h *CalculatorToolHandler) handleDivide(toolCall sdk.ChatCompletionMessageToolCall) (string, error) {
	var params CalculatorParams
	if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &params); err != nil {
		h.logger.Error("failed to unmarshal divide parameters", zap.Error(err))
		return "", err
	}

	result, err := h.service.Divide(params.A, params.B)
	if err != nil {
		return "error: " + err.Error(), nil
	}
	return strconv.FormatFloat(result, 'f', -1, 64), nil
}
