package main

import (
	"fmt"
	"time"

	adk "github.com/inference-gateway/a2a/adk"
	sdk "github.com/inference-gateway/sdk"
	zap "go.uber.org/zap"
)

// Calculator tool provider
type CalculatorToolProvider struct {
	handler *CalculatorToolHandler
}

func NewCalculatorToolProvider(handler *CalculatorToolHandler) *CalculatorToolProvider {
	return &CalculatorToolProvider{
		handler: handler,
	}
}

func (p *CalculatorToolProvider) GetToolDefinitions() []sdk.ChatCompletionTool {
	return []sdk.ChatCompletionTool{
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
}

func (p *CalculatorToolProvider) HandleToolCall(toolCall sdk.ChatCompletionMessageToolCall) (string, error) {
	return p.handler.HandleToolCall(toolCall)
}

// GetSupportedTools returns a list of supported tool names
func (p *CalculatorToolProvider) GetSupportedTools() []string {
	return []string{"add", "subtract", "multiply", "divide"}
}

// IsToolSupported checks if a tool is supported by this provider
func (p *CalculatorToolProvider) IsToolSupported(toolName string) bool {
	supportedTools := p.GetSupportedTools()
	for _, tool := range supportedTools {
		if tool == toolName {
			return true
		}
	}
	return false
}

// Calculator task result processor
type CalculatorTaskResultProcessor struct {
	logger *zap.Logger
}

func NewCalculatorTaskResultProcessor(logger *zap.Logger) *CalculatorTaskResultProcessor {
	return &CalculatorTaskResultProcessor{
		logger: logger,
	}
}

func (p *CalculatorTaskResultProcessor) ProcessToolResult(toolCallResult string) *adk.Message {
	p.logger.Debug("processing calculator task result", zap.String("result", toolCallResult))

	// For calculator, we can complete the task immediately with the result
	return &adk.Message{
		Role:      "assistant",
		Parts:     []adk.Part{adk.TextPart{Kind: "text", Text: toolCallResult}},
		MessageID: "calc_result_" + fmt.Sprintf("%d", time.Now().UnixNano()),
	}
}

// Calculator agent info provider
type CalculatorAgentInfoProvider struct {
	logger *zap.Logger
}

func NewCalculatorAgentInfoProvider(logger *zap.Logger) *CalculatorAgentInfoProvider {
	return &CalculatorAgentInfoProvider{
		logger: logger,
	}
}

func (p *CalculatorAgentInfoProvider) GetAgentCard(baseConfig adk.Config) adk.AgentCard {
	return adk.AgentCard{
		Name:        baseConfig.AgentName,
		Description: "A mathematical calculator agent that performs basic arithmetic operations using the A2A protocol",
		URL:         "http://calculator-agent:8080",
		Version:     baseConfig.AgentVersion,
		Capabilities: adk.AgentCapabilities{
			Streaming:              &[]bool{true}[0],
			PushNotifications:      &[]bool{false}[0],
			StateTransitionHistory: &[]bool{false}[0],
		},
		DefaultInputModes:  []string{"text/plain"},
		DefaultOutputModes: []string{"text/plain"},
		Skills: []adk.AgentSkill{
			{
				ID:          "add",
				Name:        "Add Numbers",
				Description: "Add two numbers together",
				InputModes:  []string{"text/plain"},
				OutputModes: []string{"text/plain"},
			},
			{
				ID:          "subtract",
				Name:        "Subtract Numbers",
				Description: "Subtract the second number from the first number",
				InputModes:  []string{"text/plain"},
				OutputModes: []string{"text/plain"},
			},
			{
				ID:          "multiply",
				Name:        "Multiply Numbers",
				Description: "Multiply two numbers together",
				InputModes:  []string{"text/plain"},
				OutputModes: []string{"text/plain"},
			},
			{
				ID:          "divide",
				Name:        "Divide Numbers",
				Description: "Divide the first number by the second number",
				InputModes:  []string{"text/plain"},
				OutputModes: []string{"text/plain"},
			},
		},
	}
}

func stringPtr(s string) *string {
	return &s
}
