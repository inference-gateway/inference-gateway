package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	m "github.com/metoro-io/mcp-golang"

	config "github.com/inference-gateway/inference-gateway/config"
	logger "github.com/inference-gateway/inference-gateway/logger"
	types "github.com/inference-gateway/inference-gateway/providers/types"
)

// NewMCPClient is a variable holding the function to create a new MCP client
func NewMCPClient(serverURLs []string, logger logger.Logger, cfg config.Config) MCPClientInterface {
	return &MCPClient{
		ServerURLs:          serverURLs,
		Logger:              logger,
		Config:              cfg,
		clients:             make(map[string]*m.Client),
		serverCapabilities:  make(map[string]ServerCapabilities),
		serverTools:         make(map[string][]Tool),
		chatCompletionTools: make([]types.ChatCompletionTool, 0),
		serverStatuses:      make(map[string]ServerStatus),
		reconnecting:        make(map[string]struct{}),
		pollingDone:         make(chan struct{}),
	}
}

// InitializeAll implements MCPClientInterface with enhanced transport fallback.
func (mc *MCPClient) InitializeAll(ctx context.Context) error {
	if len(mc.ServerURLs) == 0 {
		return ErrNoServerURLs
	}

	var lastError error
	successfulInitializations := 0
	failedServers := make([]string, 0)

	mc.mu.Lock()
	for _, serverURL := range mc.ServerURLs {
		mc.serverStatuses[serverURL] = ServerStatusUnknown
	}
	mc.mu.Unlock()

	for _, serverURL := range mc.ServerURLs {
		if err := mc.initializeServer(ctx, serverURL); err != nil {
			mc.Logger.Error("failed to initialize mcp server", err, "server", serverURL, "component", "mcp_client")
			lastError = err
			failedServers = append(failedServers, serverURL)
			continue
		}

		successfulInitializations++
		mc.Logger.Info("successfully initialized mcp server", "server", serverURL, "component", "mcp_client")
	}

	mc.mu.Lock()
	mc.initialized = true
	mc.mu.Unlock()

	if successfulInitializations == 0 {
		if mc.scheduleReconnectionIfEnabled(failedServers) {
			mc.Logger.Warn("no servers successfully initialized; enabling MCP with background reconnection",
				"total_servers", len(mc.ServerURLs),
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
		"total_servers", len(mc.ServerURLs),
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
func (mc *MCPClient) initializeServer(ctx context.Context, serverURL string) error {
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
		mc.clients[serverURL] = client
		mc.serverCapabilities[serverURL] = newServerCapabilities()
		mc.serverTools[serverURL] = tools
		mc.serverStatuses[serverURL] = ServerStatusAvailable
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
	mc.serverStatuses[serverURL] = ServerStatusUnavailable
	mc.mu.Unlock()

	return fmt.Errorf("failed to initialize server after %d attempts: %w", maxRetries+1, lastErr)
}

// initializeClientWithTransport attempts to initialize a client with a specific transport
func (mc *MCPClient) initializeClientWithTransport(ctx context.Context, serverURL string, mode TransportMode) (*m.Client, error) {
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

// newServerCapabilities returns the capability entry recorded for a server
func newServerCapabilities() ServerCapabilities {
	return ServerCapabilities{
		Completions:  make(map[string]any),
		Experimental: make(map[string]map[string]any),
		Logging:      make(map[string]any),
		Prompts:      make(map[string]any),
		Resources:    make(map[string]any),
		Tools:        make(map[string]any),
	}
}

// rebuildChatCompletionToolsLocked re-aggregates the pre-converted chat completion tools; mc.mu must be held
func (mc *MCPClient) rebuildChatCompletionToolsLocked() {
	all := make([]types.ChatCompletionTool, 0)
	for serverURL, serverTools := range mc.serverTools {
		if len(serverTools) == 0 {
			mc.Logger.Debug("no tools to convert for server", "server", serverURL)
			continue
		}

		toolsToConvert := mc.filterTools(serverTools)
		if len(toolsToConvert) == 0 {
			mc.Logger.Debug("all tools filtered out by include/exclude config for server", "server", serverURL)
			continue
		}

		chatTools := mc.ConvertMCPToolsToChatCompletionTools(toolsToConvert)
		mc.Logger.Debug("converted tools for server", "server", serverURL, "inputToolCount", len(serverTools), "outputCount", len(chatTools))
		all = append(all, chatTools...)
	}

	mc.chatCompletionTools = all
	mc.Logger.Debug("total pre-converted tools", "count", len(all))
}

// discoverServerTools fetches and converts the server's tool list
func (mc *MCPClient) discoverServerTools(ctx context.Context, client *m.Client, serverURL string) ([]Tool, error) {
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
	for _, server := range failedServers {
		reconnectingServers[server] = true
	}

	for {
		select {
		case <-ctx.Done():
			mc.Logger.Info("background reconnection stopped due to context cancellation", "component", "mcp_client")
			return
		case <-ticker.C:
			mc.mu.RLock()
			serversToReconnect := make([]string, 0)
			for serverURL := range reconnectingServers {
				if status, exists := mc.serverStatuses[serverURL]; exists && status == ServerStatusUnavailable {
					serversToReconnect = append(serversToReconnect, serverURL)
				} else if status == ServerStatusAvailable {
					delete(reconnectingServers, serverURL)
					mc.Logger.Info("server successfully reconnected, removing from background reconnection",
						"server", serverURL, "component", "mcp_client")
				}
			}
			mc.mu.RUnlock()

			if len(reconnectingServers) == 0 {
				mc.Logger.Info("all servers successfully reconnected, stopping background reconnection", "component", "mcp_client")
				return
			}

			for _, serverURL := range serversToReconnect {
				go mc.attemptServerReconnection(ctx, serverURL)
			}
		}
	}
}

// attemptServerReconnection attempts to reconnect a single failed server
func (mc *MCPClient) attemptServerReconnection(ctx context.Context, serverURL string) {
	mc.mu.Lock()
	if _, busy := mc.reconnecting[serverURL]; busy {
		mc.mu.Unlock()
		return
	}
	mc.reconnecting[serverURL] = struct{}{}
	mc.mu.Unlock()

	defer func() {
		mc.mu.Lock()
		delete(mc.reconnecting, serverURL)
		mc.mu.Unlock()
	}()

	mc.Logger.Info("attempting server reconnection", "server", serverURL, "component", "mcp_client")

	reconnectCtx, cancel := context.WithTimeout(ctx, mc.Config.MCP.ClientTimeout)
	defer cancel()

	if err := mc.initializeServer(reconnectCtx, serverURL); err != nil {
		mc.Logger.Info("server reconnection failed", "server", serverURL, "error", err, "component", "mcp_client")
		return
	}

	mc.Logger.Info("server successfully reconnected", "server", serverURL, "component", "mcp_client")
}

// Ensure compile-time interface compliance
var _ MCPClientInterface = (*MCPClient)(nil)
