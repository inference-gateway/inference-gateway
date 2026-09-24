package mcp

import (
	"context"
	"errors"
	"net/http"
	"sync"

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
	// InitializeAll discovers every MCP server's tools; 2026-07-28 has no handshake
	InitializeAll(ctx context.Context) error

	// IsInitialized returns whether the client has been successfully initialized
	IsInitialized() bool

	// ExecuteTool invokes a tool on the MCP server with the given alias
	ExecuteTool(ctx context.Context, request Request, serverAlias string) (*CallToolResult, error)

	// GetServers returns the aliases of the configured MCP servers
	GetServers() []string

	// GetServerTools returns the tools available on the server with the given alias
	GetServerTools(serverAlias string) ([]Tool, error)

	// GetAllChatCompletionTools returns all pre-converted chat completion tools from all servers
	GetAllChatCompletionTools() []types.ChatCompletionTool

	// GetSelectorTools returns the two selector meta-tools, or empty when no tools are available
	GetSelectorTools() []types.ChatCompletionTool

	// GetToolsCatalog answers an mcp_tools_get call from the cached tool map
	GetToolsCatalog(query string, names []string) []ToolCatalogEntry

	// ResolveTool splits a namespaced mcp_<alias>_<tool> name into the server
	// alias that provides it and the bare tool name the server knows it by
	ResolveTool(namespacedName string) (serverAlias string, toolName string, err error)

	// GetAllServerStatuses returns the status of all servers, keyed by alias
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

// MCPClient provides methods to interact with MCP servers. Every per-server
// map is keyed by the server's alias; the URL stays an internal detail carried
// on the ServerSpec.
type MCPClient struct {
	Servers             []ServerSpec
	Logger              logger.Logger
	Config              config.Config
	mu                  sync.RWMutex
	httpClient          *http.Client
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
