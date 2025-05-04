package mcp

import (
	"context"

	mcp "github.com/metoro-io/mcp-golang"

	"github.com/inference-gateway/inference-gateway/logger"
)

// MCPClientInterface defines the interface for MCP client implementations
//
//go:generate mockgen -source=client.go -destination=../tests/mocks/mcp_client.go -package=mocks
type MCPClientInterface interface {
	// InitializeAll establishes connection with MCP servers and performs handshake
	InitializeAll(ctx context.Context) error

	// IsInitialized returns whether the client has been successfully initialized
	IsInitialized() bool

	// DiscoverCapabilities retrieves capabilities from all configured MCP servers
	DiscoverCapabilities(ctx context.Context) ([]MCPCapability, error)

	// ExecuteTool invokes a tool on the appropriate MCP server
	ExecuteTool(ctx context.Context, toolName string, params MCPToolParams, serverURL string) (MCPToolResult, error)

	// StreamChatWithTools sends a chat request with tool capabilities and streams the response
	StreamChatWithTools(ctx context.Context, messages []PromptMessage, serverURL string) error

	// GetServerCapabilities returns the server capabilities map
	GetServerCapabilities() map[string]ServerCapabilities
}

// MCPClient provides methods to interact with MCP servers
type MCPClient struct {
	ServerURLs         []string
	Clients            map[string]*mcp.Client
	Logger             logger.Logger
	ServerCapabilities map[string]map[string]interface{}
	Initialized        bool
}

// NewMCPClient is a variable holding the function to create a new MCP client
// This allows for overriding in tests
func NewMCPClient(serverURLs []string, logger logger.Logger) MCPClientInterface {
	return &MCPClient{
		ServerURLs:         serverURLs,
		Clients:            make(map[string]*mcp.Client),
		Logger:             logger,
		ServerCapabilities: make(map[string]map[string]interface{}),
		Initialized:        false,
	}
}
