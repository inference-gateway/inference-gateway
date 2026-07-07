package mcp

import (
	"context"
	"maps"
	"time"
)

// GetServerStatus returns the status of a specific server
func (mc *MCPClient) GetServerStatus(serverURL string) ServerStatus {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	if status, exists := mc.serverStatuses[serverURL]; exists {
		return status
	}
	return ServerStatusUnknown
}

// GetAllServerStatuses returns the status of all servers
func (mc *MCPClient) GetAllServerStatuses() map[string]ServerStatus {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	statusCopy := make(map[string]ServerStatus, len(mc.serverStatuses))
	maps.Copy(statusCopy, mc.serverStatuses)
	return statusCopy
}

// StartStatusPolling starts the background status polling goroutine
func (mc *MCPClient) StartStatusPolling(ctx context.Context) {
	if !mc.Config.MCP.PollingEnable {
		mc.Logger.Debug("mcp status polling disabled, not starting background polling")
		return
	}

	pollingCtx, cancel := context.WithCancel(ctx)
	mc.pollingCancel = cancel

	go mc.statusPollingLoop(pollingCtx)
	mc.Logger.Info("started mcp server status polling", "interval", mc.Config.MCP.PollingInterval, "component", "mcp_client")
}

// StopStatusPolling stops the background status polling goroutine
func (mc *MCPClient) StopStatusPolling() {
	if mc.pollingCancel != nil {
		mc.pollingCancel()
		<-mc.pollingDone
		mc.Logger.Info("stopped mcp server status polling", "component", "mcp_client")
	}
}

// statusPollingLoop continuously polls server health status
func (mc *MCPClient) statusPollingLoop(ctx context.Context) {
	defer close(mc.pollingDone)

	ticker := time.NewTicker(mc.Config.MCP.PollingInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			mc.pollServerStatuses(ctx)
		}
	}
}

// pollServerStatuses checks the health status of all servers
func (mc *MCPClient) pollServerStatuses(ctx context.Context) {
	for _, serverURL := range mc.ServerURLs {
		go mc.checkServerHealth(ctx, serverURL)
	}
}

// checkServerHealth checks the health of a single server
func (mc *MCPClient) checkServerHealth(ctx context.Context, serverURL string) {
	checkCtx, cancel := context.WithTimeout(ctx, mc.Config.MCP.PollingTimeout)
	defer cancel()

	mc.mu.RLock()
	client, exists := mc.clients[serverURL]
	mc.mu.RUnlock()

	if !exists {
		mc.Logger.Debug("server client not found for health check", "server", serverURL, "component", "mcp_client")
		return
	}

	var cursor *string
	_, err := client.ListTools(checkCtx, cursor)

	newStatus := ServerStatusAvailable
	if err != nil {
		newStatus = ServerStatusUnavailable
		if !mc.Config.MCP.DisableHealthcheckLogs {
			mc.Logger.Debug("server health check failed", "server", serverURL, "error", err, "component", "mcp_client")
		}
	} else if !mc.Config.MCP.DisableHealthcheckLogs {
		mc.Logger.Debug("server health check passed", "server", serverURL, "component", "mcp_client")
	}

	mc.mu.Lock()
	oldStatus := mc.serverStatuses[serverURL]
	mc.serverStatuses[serverURL] = newStatus
	mc.mu.Unlock()

	if oldStatus != newStatus {
		mc.Logger.Info("server status changed", "server", serverURL, "oldStatus", string(oldStatus), "newStatus", string(newStatus), "component", "mcp_client")
	}

	if newStatus == ServerStatusUnavailable && oldStatus == ServerStatusAvailable && mc.Config.MCP.EnableReconnect {
		mc.Logger.Info("server became unavailable, scheduling reconnection", "server", serverURL, "component", "mcp_client")
		go mc.attemptServerReconnection(ctx, serverURL)
	}
}
