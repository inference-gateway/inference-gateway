package middlewares

import (
	"context"
)

// MCPClientInterface defines the interface for MCP client implementations
//
//go:generate mockgen -source=interfaces.go -destination=../../tests/mocks/middlewares/mcp_client.go -package=mocks
type MCPClientInterface interface {
	DiscoverCapabilities(ctx context.Context) ([]map[string]interface{}, error)
	ExecuteTool(ctx context.Context, toolName string, params interface{}, serverURL string) (map[string]interface{}, error)
}
