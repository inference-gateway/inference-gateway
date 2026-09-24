package mcp

import (
	"context"
	"maps"
	"time"
)

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
	if !mc.Config.MCP.PollingEnabled {
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
	for _, server := range mc.Servers {
		go mc.checkServerHealth(ctx, server)
	}
}

// checkServerHealth checks the health of a single server
func (mc *MCPClient) checkServerHealth(ctx context.Context, server ServerSpec) {
	alias := server.Alias
	checkCtx, cancel := context.WithTimeout(ctx, mc.Config.MCP.PollingTimeout)
	defer cancel()

	mc.mu.RLock()
	_, discovered := mc.serverTools[alias]
	mc.mu.RUnlock()

	if !discovered {
		mc.Logger.Debug("server never discovered, leaving it to reconnection", "server", alias, "component", "mcp_client")
		return
	}

	_, err := mc.listTools(checkCtx, server.URL)

	newStatus := ServerStatusAvailable
	if err != nil {
		newStatus = ServerStatusUnavailable
		if !mc.Config.MCP.DisableHealthcheckLogs {
			mc.Logger.Debug("server health check failed", "server", alias, "error", err, "component", "mcp_client")
		}
	} else if !mc.Config.MCP.DisableHealthcheckLogs {
		mc.Logger.Debug("server health check passed", "server", alias, "component", "mcp_client")
	}

	mc.mu.Lock()
	oldStatus := mc.serverStatuses[alias]
	mc.serverStatuses[alias] = newStatus
	mc.mu.Unlock()

	if oldStatus != newStatus {
		mc.Logger.Info("server status changed", "server", alias, "oldStatus", string(oldStatus), "newStatus", string(newStatus), "component", "mcp_client")
	}

	if newStatus == ServerStatusUnavailable && oldStatus == ServerStatusAvailable && mc.Config.MCP.EnableReconnect {
		mc.Logger.Info("server became unavailable, scheduling reconnection", "server", alias, "component", "mcp_client")
		go mc.attemptServerReconnection(ctx, alias)
	}
}
