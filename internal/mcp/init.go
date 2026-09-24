package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	golang "github.com/metoro-io/mcp-golang"

	config "github.com/inference-gateway/inference-gateway/config"
	logger "github.com/inference-gateway/inference-gateway/logger"
	types "github.com/inference-gateway/inference-gateway/providers/types"
)

// NewMCPClient creates a new MCP client for the given server specs, as parsed
// from MCP_SERVERS by ParseServers.
func NewMCPClient(servers []ServerSpec, logger logger.Logger, cfg config.Config) MCPClientInterface {
	return &MCPClient{
		Servers:             servers,
		Logger:              logger,
		Config:              cfg,
		clients:             make(map[string]*golang.Client),
		serverTools:         make(map[string][]Tool),
		chatCompletionTools: make([]types.ChatCompletionTool, 0),
		serverStatuses:      make(map[string]ServerStatus),
		reconnecting:        make(map[string]struct{}),
		pollingDone:         make(chan struct{}),
	}
}

// InitializeAll implements MCPClientInterface with enhanced transport fallback.
func (mc *MCPClient) InitializeAll(ctx context.Context) error {
	if len(mc.Servers) == 0 {
		return ErrNoServerURLs
	}

	var lastError error
	successfulInitializations := 0
	failedServers := make([]string, 0)

	mc.mu.Lock()
	for _, server := range mc.Servers {
		mc.serverStatuses[server.Alias] = ServerStatusUnknown
	}
	mc.mu.Unlock()

	for _, server := range mc.Servers {
		if err := mc.initializeServer(ctx, server); err != nil {
			mc.Logger.Error("failed to initialize mcp server", err, "server", server.Alias, "url", server.URL, "component", "mcp_client")
			lastError = err
			failedServers = append(failedServers, server.Alias)
			continue
		}

		successfulInitializations++
		mc.Logger.Info("successfully initialized mcp server", "server", server.Alias, "url", server.URL, "component", "mcp_client")
	}

	mc.mu.Lock()
	mc.initialized = true
	mc.mu.Unlock()

	if successfulInitializations == 0 {
		if mc.scheduleReconnectionIfEnabled(failedServers) {
			mc.Logger.Warn("no servers successfully initialized; enabling MCP with background reconnection",
				"total_servers", len(mc.Servers),
				"failed_servers", len(failedServers),
				"component", "mcp_client")
			return nil
		}

		if lastError != nil {
			return fmt.Errorf("%w: %v", ErrNoClientsInitialized, lastError)
		}
		return ErrNoClientsInitialized
	}

	mc.Logger.Debug("mcp pre-converting all tools to chat completion format")

	mc.mu.Lock()
	mc.rebuildChatCompletionToolsLocked()
	mc.mu.Unlock()

	mc.Logger.Info("mcp client initialization completed",
		"successful_servers", successfulInitializations,
		"failed_servers", len(failedServers),
		"total_servers", len(mc.Servers),
		"component", "mcp_client")

	mc.scheduleReconnectionIfEnabled(failedServers)

	return nil
}

// scheduleReconnectionIfEnabled is the single guard point for kicking off the
// background reconnection goroutine.
func (mc *MCPClient) scheduleReconnectionIfEnabled(failedServers []string) bool {
	if !mc.Config.MCP.EnableReconnect || len(failedServers) == 0 {
		return false
	}
	mc.spawnBackgroundReconnection(failedServers)
	return true
}

// spawnBackgroundReconnection launches the reconnect goroutine with a
// cancellable context owned by the client.
func (mc *MCPClient) spawnBackgroundReconnection(failedServers []string) {
	mc.reconnectMutex.Lock()
	defer mc.reconnectMutex.Unlock()

	if mc.reconnectCancel != nil {
		return
	}

	reconnectCtx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	mc.reconnectCancel = cancel
	mc.reconnectDone = done

	go func() {
		defer close(done)
		defer cancel()
		mc.startBackgroundReconnection(reconnectCtx, failedServers)
	}()
}

// StopBackgroundReconnection cancels the reconnection goroutine (if any) and
// waits for it to exit. Safe to call when no reconnection has been started.
func (mc *MCPClient) StopBackgroundReconnection() {
	mc.reconnectMutex.Lock()
	cancel := mc.reconnectCancel
	done := mc.reconnectDone
	mc.reconnectCancel = nil
	mc.reconnectDone = nil
	mc.reconnectMutex.Unlock()

	if cancel == nil {
		return
	}

	cancel()
	if done != nil {
		<-done
	}
	mc.Logger.Info("stopped mcp background reconnection", "component", "mcp_client")
}

// initializeServer initializes a single server with retry logic
func (mc *MCPClient) initializeServer(ctx context.Context, server ServerSpec) error {
	serverURL := server.URL
	maxRetries := mc.Config.MCP.MaxRetries
	initialBackoff := mc.Config.MCP.InitialBackoff
	var lastErr error

	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			backoffDelay := time.Duration(float64(initialBackoff) * float64(uint(1)<<uint(attempt-1)))
			if backoffDelay > mc.Config.MCP.RetryInterval {
				backoffDelay = mc.Config.MCP.RetryInterval
			}

			mc.Logger.Debug("retrying server initialization",
				"server", serverURL,
				"attempt", attempt+1,
				"max_attempts", maxRetries+1,
				"backoff_delay", backoffDelay,
				"component", "mcp_client")

			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(backoffDelay):
			}
		}

		client, err := mc.initializeClientWithTransport(ctx, serverURL, TransportModeStreamableHTTP)
		if err != nil {
			mc.Logger.Debug("streamable http failed, attempting sse fallback", "server", serverURL, "error", err.Error())

			client, err = mc.initializeClientWithTransport(ctx, serverURL, TransportModeSSE)
			if err != nil {
				lastErr = fmt.Errorf("both streamable http and sse transports failed: %w", err)
				mc.Logger.Debug("failed to initialize server",
					"server", serverURL,
					"attempt", attempt+1,
					"error", err,
					"component", "mcp_client")
				continue
			}
			mc.Logger.Info("successfully connected using sse transport fallback", "server", serverURL)
		} else {
			mc.Logger.Debug("successfully connected using streamable http transport", "server", serverURL)
		}

		tools, err := mc.discoverServerTools(ctx, client, serverURL)
		if err != nil {
			lastErr = fmt.Errorf("failed to discover server capabilities: %w", err)
			mc.Logger.Debug("failed to discover capabilities",
				"server", serverURL,
				"attempt", attempt+1,
				"error", err,
				"component", "mcp_client")
			continue
		}

		mc.mu.Lock()
		mc.clients[server.Alias] = client
		mc.serverTools[server.Alias] = tools
		mc.serverStatuses[server.Alias] = ServerStatusAvailable
		if mc.initialized {
			mc.rebuildChatCompletionToolsLocked()
		}
		mc.mu.Unlock()

		mc.Logger.Info("server initialized successfully",
			"server", serverURL,
			"attempts_used", attempt+1,
			"component", "mcp_client")

		return nil
	}

	mc.mu.Lock()
	mc.serverStatuses[server.Alias] = ServerStatusUnavailable
	mc.mu.Unlock()

	return fmt.Errorf("failed to initialize server after %d attempts: %w", maxRetries+1, lastErr)
}

// initializeClientWithTransport attempts to initialize a client with a specific transport
func (mc *MCPClient) initializeClientWithTransport(ctx context.Context, serverURL string, mode TransportMode) (*golang.Client, error) {
	client := mc.NewClientWithTransport(serverURL, mode)

	mc.Logger.Debug("attempting client initialization", "server", serverURL, "transport", string(mode), "timeout", mc.Config.MCP.RequestTimeout.String())

	initCtx, cancel := context.WithTimeout(ctx, mc.Config.MCP.RequestTimeout)
	defer cancel()

	_, err := client.Initialize(initCtx)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return nil, fmt.Errorf("initialization timed out with %s transport: %w", mode, err)
		}
		return nil, fmt.Errorf("initialization failed with %s transport: %w", mode, err)
	}

	return client, nil
}

// rebuildChatCompletionToolsLocked re-aggregates the pre-converted chat completion tools; mc.mu must be held
func (mc *MCPClient) rebuildChatCompletionToolsLocked() {
	all := make([]types.ChatCompletionTool, 0)
	for alias, serverTools := range mc.serverTools {
		if len(serverTools) == 0 {
			mc.Logger.Debug("no tools to convert for server", "server", alias)
			continue
		}

		toolsToConvert := mc.filterTools(alias, serverTools)
		if len(toolsToConvert) == 0 {
			mc.Logger.Debug("all tools filtered out by include/exclude config for server", "server", alias)
			continue
		}

		chatTools := mc.ConvertMCPToolsToChatCompletionTools(alias, toolsToConvert)
		mc.Logger.Debug("converted tools for server", "server", alias, "inputToolCount", len(serverTools), "outputCount", len(chatTools))
		all = append(all, chatTools...)
	}

	mc.chatCompletionTools = all
	mc.Logger.Debug("total pre-converted tools", "count", len(all))
}

// discoverServerTools fetches and converts the server's tool list
func (mc *MCPClient) discoverServerTools(ctx context.Context, client *golang.Client, serverURL string) ([]Tool, error) {
	mc.Logger.Debug("fetching available tools", "server", serverURL)

	toolsCtx, toolsCancel := context.WithTimeout(ctx, mc.Config.MCP.RequestTimeout)
	defer toolsCancel()

	mc.Logger.Debug("attempting to list tools with timeout", "server", serverURL, "timeout", mc.Config.MCP.RequestTimeout.String())
	var cursor *string
	toolsResult, err := client.ListTools(toolsCtx, cursor)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			mc.Logger.Error("tools listing timed out", err, "server", serverURL)
		} else {
			mc.Logger.Error("failed to list tools", err, "server", serverURL)
			mc.Logger.Debug("tools listing error details", "error", err.Error(), "server", serverURL)
		}
		return nil, err
	}

	mc.Logger.Debug("successfully retrieved tools list", "server", serverURL, "rawToolsCount", len(toolsResult.Tools))
	for i, tool := range toolsResult.Tools {
		mc.Logger.Debug("mcp raw tool discovered", "server", serverURL, "index", i, "name", tool.Name, "hasDescription", tool.Description != nil, "hasInputSchema", tool.InputSchema != nil)
	}

	serverTools := make([]Tool, 0, len(toolsResult.Tools))

	for _, tool := range toolsResult.Tools {
		enhancedDesc := tool.Description
		if enhancedDesc == nil {
			enhancedDesc = new(string)
			*enhancedDesc = ""
		}

		inputSchema := make(map[string]any)
		if tool.InputSchema != nil {
			if inputBytes, err := json.Marshal(tool.InputSchema); err == nil {
				_ = json.Unmarshal(inputBytes, &inputSchema)
			}
		}

		serverTools = append(serverTools, Tool{
			Name:        tool.Name,
			Description: enhancedDesc,
			InputSchema: inputSchema,
		})

		mc.Logger.Debug("processed tool", "server", serverURL, "toolName", tool.Name, "enhancedDesc", *enhancedDesc)
	}

	mc.Logger.Debug("found tools for server", "server", serverURL, "count", len(serverTools))

	return serverTools, nil
}

// startBackgroundReconnection starts a background goroutine to reconnect failed servers
func (mc *MCPClient) startBackgroundReconnection(ctx context.Context, failedServers []string) {
	mc.Logger.Info("starting background reconnection for failed servers",
		"servers", failedServers,
		"interval", mc.Config.MCP.ReconnectInterval,
		"component", "mcp_client")

	defer func() {
		mc.reconnectMutex.Lock()
		mc.reconnectCancel = nil
		mc.reconnectMutex.Unlock()
	}()

	ticker := time.NewTicker(mc.Config.MCP.ReconnectInterval)
	defer ticker.Stop()

	reconnectingServers := make(map[string]bool)
	for _, alias := range failedServers {
		reconnectingServers[alias] = true
	}

	for {
		select {
		case <-ctx.Done():
			mc.Logger.Info("background reconnection stopped due to context cancellation", "component", "mcp_client")
			return
		case <-ticker.C:
			mc.mu.RLock()
			serversToReconnect := make([]string, 0)
			for alias := range reconnectingServers {
				if status, exists := mc.serverStatuses[alias]; exists && status == ServerStatusUnavailable {
					serversToReconnect = append(serversToReconnect, alias)
				} else if status == ServerStatusAvailable {
					delete(reconnectingServers, alias)
					mc.Logger.Info("server successfully reconnected, removing from background reconnection",
						"server", alias, "component", "mcp_client")
				}
			}
			mc.mu.RUnlock()

			if len(reconnectingServers) == 0 {
				mc.Logger.Info("all servers successfully reconnected, stopping background reconnection", "component", "mcp_client")
				return
			}

			for _, alias := range serversToReconnect {
				go mc.attemptServerReconnection(ctx, alias)
			}
		}
	}
}

// attemptServerReconnection attempts to reconnect a single failed server
func (mc *MCPClient) attemptServerReconnection(ctx context.Context, alias string) {
	server, ok := mc.serverSpec(alias)
	if !ok {
		mc.Logger.Debug("no server spec for alias, skipping reconnection", "server", alias, "component", "mcp_client")
		return
	}

	mc.mu.Lock()
	if _, busy := mc.reconnecting[alias]; busy {
		mc.mu.Unlock()
		return
	}
	mc.reconnecting[alias] = struct{}{}
	mc.mu.Unlock()

	defer func() {
		mc.mu.Lock()
		delete(mc.reconnecting, alias)
		mc.mu.Unlock()
	}()

	mc.Logger.Info("attempting server reconnection", "server", alias, "component", "mcp_client")

	reconnectCtx, cancel := context.WithTimeout(ctx, mc.Config.MCP.ClientTimeout)
	defer cancel()

	if err := mc.initializeServer(reconnectCtx, server); err != nil {
		mc.Logger.Info("server reconnection failed", "server", alias, "error", err, "component", "mcp_client")
		return
	}

	mc.Logger.Info("server successfully reconnected", "server", alias, "component", "mcp_client")
}

// serverSpec looks up a configured server by alias.
func (mc *MCPClient) serverSpec(alias string) (ServerSpec, bool) {
	for _, server := range mc.Servers {
		if server.Alias == alias {
			return server, true
		}
	}
	return ServerSpec{}, false
}

// Ensure compile-time interface compliance
var _ MCPClientInterface = (*MCPClient)(nil)
