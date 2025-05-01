package middlewares

import (
	"context"
)

// MCPClientInterface defines the interface for MCP client implementations
//
//go:generate mockgen -source=interfaces.go -destination=../../tests/mocks/middlewares/mcp_client.go -package=mocks
type MCPClientInterface interface {
	// Initialize establishes connection with MCP servers and performs handshake
	Initialize(ctx context.Context) error

	// DiscoverCapabilities retrieves capabilities from all configured MCP servers
	DiscoverCapabilities(ctx context.Context) ([]map[string]interface{}, error)

	// ExecuteTool invokes a tool on the appropriate MCP server
	ExecuteTool(ctx context.Context, toolName string, params interface{}, serverURL string) (map[string]interface{}, error)

	// StreamChatWithTools sends a chat request with tool capabilities and streams the response
	StreamChatWithTools(ctx context.Context, messages []map[string]interface{}, serverURL string, callback func(chunk map[string]interface{}) error) error

	// IsInitialized returns whether the client has been successfully initialized
	IsInitialized() bool

	// GetServerCapabilities returns the server capabilities map
	GetServerCapabilities() map[string]map[string]interface{}
}
