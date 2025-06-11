package adk

import (
	sdk "github.com/inference-gateway/sdk"
	"go.uber.org/zap"
)

// ToolProvider defines the interface for providing tools to agents
// This interface should be implemented by domain-specific tool providers
type ToolsProvider interface {
	// GetToolDefinitions returns the tool definitions for this provider
	GetToolDefinitions() []sdk.ChatCompletionTool

	// HandleToolCall processes a tool call and returns the result
	HandleToolCall(toolCall sdk.ChatCompletionMessageToolCall) (string, error)

	// GetSupportedTools returns a list of supported tool names
	GetSupportedTools() []string

	// IsToolSupported checks if a tool is supported by this provider
	IsToolSupported(toolName string) bool
}

// ToolsHandler provides a generic implementation for handling tools
type ToolsHandler struct {
	tools  []ToolsProvider
	logger *zap.Logger
}

// NewToolsHandler creates a new generic tools handler with the provided tool providers
func NewToolsHandler(logger *zap.Logger, toolsHandler ...ToolsProvider) *ToolsHandler {
	return &ToolsHandler{
		tools:  toolsHandler,
		logger: logger,
	}
}

// GetAllToolDefinitions returns all tool definitions from all providers
func (t *ToolsHandler) GetAllToolDefinitions() []sdk.ChatCompletionTool {
	var allTools []sdk.ChatCompletionTool
	for _, tool := range t.tools {
		tools := tool.GetToolDefinitions()
		allTools = append(allTools, tools...)
	}
	return allTools
}

// HandleToolCall processes a tool call by delegating to the appropriate provider
func (t *ToolsHandler) HandleToolCall(toolCall sdk.ChatCompletionMessageToolCall) (string, error) {
	toolName := toolCall.Function.Name

	// Find the provider that supports this tool
	for _, provider := range t.tools {
		if provider.IsToolSupported(toolName) {
			t.logger.Debug("delegating tool call to provider",
				zap.String("tool", toolName))
			return provider.HandleToolCall(toolCall)
		}
	}

	// No provider found for this tool
	t.logger.Warn("no provider found for tool", zap.String("tool", toolName))
	return "", NewUnsupportedToolError(toolName)
}

// GetAllSupportedTools returns all supported tools from all providers
func (t *ToolsHandler) GetAllSupportedTools() []string {
	var allTools []string
	for _, provider := range t.tools {
		tools := provider.GetSupportedTools()
		allTools = append(allTools, tools...)
	}
	return allTools
}

// IsToolSupported checks if any provider supports the given tool
func (t *ToolsHandler) IsToolSupported(toolName string) bool {
	for _, provider := range t.tools {
		if provider.IsToolSupported(toolName) {
			return true
		}
	}
	return false
}

// UnsupportedToolError represents an error for unsupported tools
type UnsupportedToolError struct {
	ToolName string
}

func (e *UnsupportedToolError) Error() string {
	return "unsupported tool: " + e.ToolName
}

// NewUnsupportedToolError creates a new UnsupportedToolError
func NewUnsupportedToolError(toolName string) error {
	return &UnsupportedToolError{ToolName: toolName}
}
