package main

import (
	adk "github.com/inference-gateway/a2a/adk"
	sdk "github.com/inference-gateway/sdk"
)

// GreetingToolProvider implements ToolProvider for greeting functionality
type GreetingToolProvider struct {
	greetingToolHandler *GreetingToolHandler
}

// NewGreetingToolProvider creates a new GreetingToolProvider instance
func NewGreetingToolProvider(greetingToolHandler *GreetingToolHandler) *GreetingToolProvider {
	return &GreetingToolProvider{
		greetingToolHandler: greetingToolHandler,
	}
}

// GetToolDefinitions returns the tool definitions for greeting functionality
func (gtp *GreetingToolProvider) GetToolDefinitions() []sdk.ChatCompletionTool {
	return []sdk.ChatCompletionTool{
		{
			Type: "function",
			Function: sdk.FunctionObject{
				Name:        "greet",
				Description: adk.StringPtr("Greet a person in a specified language"),
				Parameters: &sdk.FunctionParameters{
					"type": "object",
					"properties": map[string]interface{}{
						"language": map[string]interface{}{
							"type":        "string",
							"description": "The language to greet in (e.g., 'en' for English, 'es' for Spanish)",
							"enum":        []string{"en", "es", "fr", "de", "zh", "ja", "ko", "it", "pt", "ru"},
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
}

// HandleToolCall processes a tool call and returns the result
func (gtp *GreetingToolProvider) HandleToolCall(toolCall sdk.ChatCompletionMessageToolCall) (string, error) {
	switch toolCall.Function.Name {
	case "greet":
		return gtp.greetingToolHandler.HandleGreetTool(toolCall.Function.Arguments)
	default:
		return "", adk.NewUnsupportedToolError(toolCall.Function.Name)
	}
}

// GetSupportedTools returns a list of supported tool names
func (gtp *GreetingToolProvider) GetSupportedTools() []string {
	return []string{"greet"}
}

// IsToolSupported checks if a tool is supported
func (gtp *GreetingToolProvider) IsToolSupported(toolName string) bool {
	supportedTools := gtp.GetSupportedTools()
	for _, tool := range supportedTools {
		if tool == toolName {
			return true
		}
	}
	return false
}
