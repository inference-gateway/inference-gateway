package mcp

import (
	"context"
	"errors"
	"sync"

	m "github.com/metoro-io/mcp-golang"

	config "github.com/inference-gateway/inference-gateway/config"
	logger "github.com/inference-gateway/inference-gateway/logger"
	types "github.com/inference-gateway/inference-gateway/providers/types"
)

var (
	// ErrClientNotInitialized is returned when a client method is called before initialization
	ErrClientNotInitialized = errors.New("mcp client not initialized")

	// ErrServerNotFound is returned when trying to use a server that doesn't exist
	ErrServerNotFound = errors.New("mcp server not found")

	// ErrNoServerURLs is returned when trying to initialize without any server URLs
	ErrNoServerURLs = errors.New("no mcp server urls provided")

	// ErrNoClientsInitialized is returned when no clients could be initialized
	ErrNoClientsInitialized = errors.New("no mcp clients could be initialized")
)

// ServerStatus represents the status of an MCP server
type ServerStatus string

const (
	ServerStatusUnknown     ServerStatus = "unknown"
	ServerStatusAvailable   ServerStatus = "available"
	ServerStatusUnavailable ServerStatus = "unavailable"
)

// MCPClientInterface defines the interface for MCP client implementations
//
//go:generate mockgen -source=client.go -destination=../../tests/mocks/mcp/client.go -package=mcpmocks -typed
type MCPClientInterface interface {
	// InitializeAll establishes connection with MCP servers and performs handshake
	InitializeAll(ctx context.Context) error

	// IsInitialized returns whether the client has been successfully initialized
	IsInitialized() bool

	// ExecuteTool invokes a tool on the appropriate MCP server
	ExecuteTool(ctx context.Context, request Request, serverURL string) (*CallToolResult, error)

	// GetServerCapabilities returns the server capabilities map
	GetServerCapabilities() map[string]ServerCapabilities

	// GetServers returns the list of MCP server URLs
	GetServers() []string

	// GetServerTools returns the tools available on the specified server
	GetServerTools(serverURL string) ([]Tool, error)

	// GetAllChatCompletionTools returns all pre-converted chat completion tools from all servers
	GetAllChatCompletionTools() []types.ChatCompletionTool

	// ConvertMCPToolsToChatCompletionTools converts MCP server tools to chat completion tools
	ConvertMCPToolsToChatCompletionTools([]Tool) []types.ChatCompletionTool

	// GetServerForTool returns the server URL that provides the specified tool
	GetServerForTool(toolName string) (string, error)

	// BuildSSEFallbackURL creates an SSE fallback URL from the main server URL (exposed for testing)
	BuildSSEFallbackURL(serverURL string) string

	// GetServerStatus returns the status of a specific server
	GetServerStatus(serverURL string) ServerStatus

	// GetAllServerStatuses returns the status of all servers
	GetAllServerStatuses() map[string]ServerStatus

	// StartStatusPolling starts the background status polling goroutine
	StartStatusPolling(ctx context.Context)

	// StopStatusPolling stops the background status polling goroutine
	StopStatusPolling()

	// StopBackgroundReconnection stops the background reconnection goroutine
	// (started internally by InitializeAll when some servers fail and
	// EnableReconnect is true). Safe to call even if reconnection was never
	// started.
	StopBackgroundReconnection()
}

// MCPClient provides methods to interact with MCP servers
type MCPClient struct {
	ServerURLs          []string
	Logger              logger.Logger
	Config              config.Config
	mu                  sync.RWMutex
	clients             map[string]*m.Client
	serverCapabilities  map[string]ServerCapabilities
	serverTools         map[string][]Tool
	chatCompletionTools []types.ChatCompletionTool
	initialized         bool
	serverStatuses      map[string]ServerStatus
	reconnecting        map[string]struct{}

	pollingCancel   context.CancelFunc
	pollingDone     chan struct{}
	reconnectCancel context.CancelFunc
	reconnectDone   chan struct{}
	reconnectMutex  sync.Mutex
}
