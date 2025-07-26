package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/inference-gateway/inference-gateway/config"
	"github.com/inference-gateway/inference-gateway/logger"
	"github.com/inference-gateway/inference-gateway/providers"
	m "github.com/metoro-io/mcp-golang"
	transport "github.com/metoro-io/mcp-golang/transport/http"
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
//go:generate mockgen -source=client.go -destination=../tests/mocks/mcp/client.go -package=mcpmocks
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
	GetAllChatCompletionTools() []providers.ChatCompletionTool

	// ConvertMCPToolsToChatCompletionTools converts MCP server tools to chat completion tools
	ConvertMCPToolsToChatCompletionTools([]Tool) []providers.ChatCompletionTool

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
}

// MCPClient provides methods to interact with MCP servers
type MCPClient struct {
	ServerURLs          []string
	Clients             map[string]*m.Client
	Logger              logger.Logger
	Config              config.Config
	ServerCapabilities  map[string]ServerCapabilities
	ServerTools         map[string][]Tool
	ChatCompletionTools []providers.ChatCompletionTool
	Initialized         bool
	ServerStatuses      map[string]ServerStatus
	statusMutex         sync.RWMutex
	pollingCancel       context.CancelFunc
	pollingDone         chan struct{}
}

// TransportMode represents the type of transport being used
type TransportMode string

const (
	TransportModeStreamableHTTP TransportMode = "streamable-http"
	TransportModeSSE            TransportMode = "sse"
	TransportModeHTTP           TransportMode = "http"
)

// customRoundTripper wraps http.RoundTripper to add streaming headers and handle SSE responses
type customRoundTripper struct {
	base        http.RoundTripper
	sessionID   string
	mode        TransportMode
	fallbackURL string
}

// parseSSEResponse extracts JSON data from SSE formatted response
func parseSSEResponse(responseBody string) (string, error) {
	lines := strings.Split(responseBody, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "data: ") {
			jsonData := strings.TrimPrefix(line, "data: ")
			if jsonData != "" && jsonData != "[DONE]" {
				return jsonData, nil
			}
		}
	}

	return "", fmt.Errorf("no valid JSON data found in SSE response")
}

func (c *customRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	req = req.Clone(req.Context())

	req.Header.Del("Authorization")
	req.Header.Del("Cookie")
	req.Header.Del("X-API-Key")

	switch c.mode {
	case TransportModeStreamableHTTP:
		req.Header.Set("Accept", "application/json, text/event-stream")
		req.Header.Set("Cache-Control", "no-cache")
		req.Header.Set("Connection", "keep-alive")
	case TransportModeSSE:
		req.Header.Set("Accept", "text/event-stream")
		req.Header.Set("Cache-Control", "no-cache")
		req.Header.Set("Connection", "keep-alive")
	default:
		req.Header.Set("Accept", "application/json, text/event-stream")
		req.Header.Set("Cache-Control", "no-cache")
		req.Header.Set("Connection", "keep-alive")
	}

	if c.sessionID != "" {
		req.Header.Set("mcp-session-id", c.sessionID)
	}

	if req.Method == "POST" && req.Body != nil {
		bodyBytes, err := io.ReadAll(req.Body)
		if err != nil {
			return nil, err
		}
		req.Body.Close()

		var jsonBody map[string]interface{}
		if err := json.Unmarshal(bodyBytes, &jsonBody); err == nil {
			if params, ok := jsonBody["params"].(map[string]interface{}); ok {
				if cursor, exists := params["cursor"]; exists && cursor == nil {
					delete(params, "cursor")
					if modifiedBody, err := json.Marshal(jsonBody); err == nil {
						bodyBytes = modifiedBody
					}
				}
			}
		}

		req.Body = io.NopCloser(strings.NewReader(string(bodyBytes)))
		req.ContentLength = int64(len(bodyBytes))
	}

	resp, err := c.base.RoundTrip(req)
	if err != nil {
		return resp, err
	}

	if sessionID := resp.Header.Get("mcp-session-id"); sessionID != "" {
		c.sessionID = sessionID
	}

	if resp.StatusCode >= 400 && resp.StatusCode < 500 && c.mode == TransportModeStreamableHTTP {
		return c.attemptSSEFallback(req)
	}

	contentType := resp.Header.Get("Content-Type")
	if strings.Contains(contentType, "text/event-stream") ||
		strings.Contains(contentType, "text/plain") {

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return resp, err
		}
		resp.Body.Close()

		bodyStr := string(body)
		if strings.Contains(bodyStr, "data: ") {
			jsonData, err := parseSSEResponse(bodyStr)
			if err != nil {
				return resp, fmt.Errorf("failed to parse SSE response: %v", err)
			}

			resp.Body = io.NopCloser(strings.NewReader(jsonData))
			resp.Header.Set("Content-Type", "application/json")
			resp.ContentLength = int64(len(jsonData))
		} else {
			resp.Body = io.NopCloser(strings.NewReader(bodyStr))
		}
	}

	return resp, nil
}

// attemptSSEFallback tries to fallback to SSE transport when Streamable HTTP fails
func (c *customRoundTripper) attemptSSEFallback(req *http.Request) (*http.Response, error) {
	c.mode = TransportModeSSE

	if c.fallbackURL != "" {
		originalURL := req.URL
		fallbackURL, err := url.Parse(c.fallbackURL)
		if err == nil {
			req.URL = fallbackURL
			req.Header.Set("Accept", "text/event-stream")

			resp, err := c.base.RoundTrip(req)
			if err != nil {
				req.URL = originalURL
				return nil, fmt.Errorf("both streamable HTTP and SSE transports failed: %v", err)
			}
			return resp, nil
		}
	}

	return nil, fmt.Errorf("streamable HTTP transport failed and no SSE fallback URL configured")
}

// NewClient creates a new MCP client for a given server URL with enhanced transport support
func (mc *MCPClient) NewClient(url string) *m.Client {
	return mc.NewClientWithTransport(url, TransportModeStreamableHTTP)
}

// NewClientWithTransport creates a new MCP client with specific transport mode
func (mc *MCPClient) NewClientWithTransport(serverURL string, mode TransportMode) *m.Client {
	baseTransport := &http.Transport{
		DialContext: (&net.Dialer{
			Timeout:   mc.Config.MCP.DialTimeout,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		TLSHandshakeTimeout:   mc.Config.MCP.TlsHandshakeTimeout,
		ResponseHeaderTimeout: mc.Config.MCP.ResponseHeaderTimeout,
		ExpectContinueTimeout: mc.Config.MCP.ExpectContinueTimeout,
	}

	fallbackURL := mc.BuildSSEFallbackURL(serverURL)

	httpClient := &http.Client{
		Timeout: mc.Config.MCP.ClientTimeout,
		Transport: &customRoundTripper{
			base:        baseTransport,
			mode:        mode,
			fallbackURL: fallbackURL,
		},
	}

	var acceptHeader string
	switch mode {
	case TransportModeStreamableHTTP:
		acceptHeader = "application/json, text/event-stream"
	case TransportModeSSE:
		acceptHeader = "text/event-stream"
	default:
		acceptHeader = "application/json, text/event-stream"
	}

	httpTransport := transport.NewHTTPClientTransport(serverURL).WithHeader(
		"Accept", acceptHeader).WithClient(httpClient)

	return m.NewClient(httpTransport)
}

// BuildSSEFallbackURL creates an SSE fallback URL from the main server URL
func (mc *MCPClient) BuildSSEFallbackURL(serverURL string) string {
	if strings.HasSuffix(serverURL, "/mcp") {
		return strings.TrimSuffix(serverURL, "/mcp") + "/sse"
	}
	if strings.HasSuffix(serverURL, "/") {
		return serverURL + "sse"
	}
	return serverURL + "/sse"
}

// NewMCPClient is a variable holding the function to create a new MCP client
func NewMCPClient(serverURLs []string, logger logger.Logger, cfg config.Config) MCPClientInterface {
	return &MCPClient{
		ServerURLs:          serverURLs,
		Clients:             make(map[string]*m.Client),
		Logger:              logger,
		Config:              cfg,
		ServerCapabilities:  make(map[string]ServerCapabilities),
		ServerTools:         make(map[string][]Tool),
		ChatCompletionTools: make([]providers.ChatCompletionTool, 0),
		Initialized:         false,
		ServerStatuses:      make(map[string]ServerStatus),
		pollingDone:         make(chan struct{}),
	}
}

// ExecuteTool implements MCPClientInterface.
func (mc *MCPClient) ExecuteTool(ctx context.Context, request Request, serverURL string) (*CallToolResult, error) {
	if !mc.Initialized {
		return nil, ErrClientNotInitialized
	}

	client, exists := mc.Clients[serverURL]
	if !exists {
		return nil, ErrServerNotFound
	}

	toolName := request.Params["name"].(string)
	toolArgs := request.Params["arguments"]

	result, err := client.CallTool(ctx, toolName, toolArgs)
	if err != nil {
		return nil, err
	}

	response := CallToolResult{
		Content: make([]ContentBlock, len(result.Content)),
	}

	for i, content := range result.Content {
		contentBytes, err := json.Marshal(content)
		if err != nil {
			mc.Logger.Error("Failed to marshal content", err)
			continue
		}

		var contentMap map[string]interface{}
		if err = json.Unmarshal(contentBytes, &contentMap); err != nil {
			mc.Logger.Error("Failed to unmarshal content", err)
			continue
		}

		response.Content[i] = contentMap
	}

	return &response, nil
}

// GetServerCapabilities implements MCPClientInterface.
func (mc *MCPClient) GetServerCapabilities() map[string]ServerCapabilities {
	return mc.ServerCapabilities
}

// InitializeAll implements MCPClientInterface with enhanced transport fallback.
func (mc *MCPClient) InitializeAll(ctx context.Context) error {
	if len(mc.ServerURLs) == 0 {
		return ErrNoServerURLs
	}

	var lastError error
	successfulInitializations := 0
	failedServers := make([]string, 0)

	mc.statusMutex.Lock()
	for _, serverURL := range mc.ServerURLs {
		mc.ServerStatuses[serverURL] = ServerStatusUnknown
	}
	mc.statusMutex.Unlock()

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

	mc.Initialized = true

	if successfulInitializations == 0 {
		mc.Logger.Warn("no servers successfully initialized, but enabling MCP with background reconnection",
			"total_servers", len(mc.ServerURLs),
			"failed_servers", len(failedServers),
			"component", "mcp_client")

		if mc.Config.MCP.EnableReconnect && len(failedServers) > 0 {
			go mc.startBackgroundReconnection(ctx, failedServers)
		}

		if lastError != nil {
			return fmt.Errorf("%w: %v", ErrNoClientsInitialized, lastError)
		}
		return ErrNoClientsInitialized
	}

	mc.Logger.Debug("mcp pre-converting all tools to chat completion format")
	mc.Logger.Debug("mcp serverTools map status", "serverCount", len(mc.ServerTools))

	for serverURL, serverTools := range mc.ServerTools {
		mc.Logger.Debug("mcp server tools status", "server", serverURL, "toolCount", len(serverTools))
	}

	allChatCompletionTools := make([]providers.ChatCompletionTool, 0)

	for serverURL, serverTools := range mc.ServerTools {
		if len(serverTools) == 0 {
			mc.Logger.Debug("no tools to convert for server", "server", serverURL)
			continue
		}

		mc.Logger.Debug("converting tools for server", "server", serverURL, "inputToolCount", len(serverTools))
		chatTools := mc.ConvertMCPToolsToChatCompletionTools(serverTools)
		mc.Logger.Debug("converted tools for server", "server", serverURL, "outputCount", len(chatTools))
		allChatCompletionTools = append(allChatCompletionTools, chatTools...)
	}

	mc.ChatCompletionTools = allChatCompletionTools
	mc.Logger.Debug("total pre-converted tools", "count", len(mc.ChatCompletionTools))

	mc.Logger.Info("mcp client initialization completed",
		"successful_servers", successfulInitializations,
		"failed_servers", len(failedServers),
		"total_servers", len(mc.ServerURLs),
		"component", "mcp_client")

	if mc.Config.MCP.EnableReconnect && len(failedServers) > 0 {
		mc.Logger.Info("starting background reconnection for failed servers",
			"failed_servers", failedServers,
			"component", "mcp_client")
		go mc.startBackgroundReconnection(ctx, failedServers)
	}

	return nil
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

		mc.Clients[serverURL] = client

		if err := mc.discoverServerCapabilities(ctx, client, serverURL); err != nil {
			lastErr = fmt.Errorf("failed to discover server capabilities: %w", err)
			mc.Logger.Debug("failed to discover capabilities",
				"server", serverURL,
				"attempt", attempt+1,
				"error", err,
				"component", "mcp_client")
			continue
		}

		mc.statusMutex.Lock()
		mc.ServerStatuses[serverURL] = ServerStatusAvailable
		mc.statusMutex.Unlock()

		mc.Logger.Info("server initialized successfully",
			"server", serverURL,
			"attempts_used", attempt+1,
			"component", "mcp_client")

		return nil
	}

	mc.statusMutex.Lock()
	mc.ServerStatuses[serverURL] = ServerStatusUnavailable
	mc.statusMutex.Unlock()

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

// discoverServerCapabilities discovers and stores server capabilities and tools
func (mc *MCPClient) discoverServerCapabilities(ctx context.Context, client *m.Client, serverURL string) error {
	capabilities := ServerCapabilities{
		Completions:  make(map[string]interface{}),
		Experimental: make(map[string]map[string]interface{}),
		Logging:      make(map[string]interface{}),
		Prompts:      make(map[string]interface{}),
		Resources:    make(map[string]interface{}),
		Tools:        make(map[string]interface{}),
	}

	mc.ServerCapabilities[serverURL] = capabilities
	mc.Logger.Debug("mcp server capabilities discovered", "server", serverURL)

	return mc.discoverServerTools(ctx, client, serverURL)
}

// discoverServerTools discovers and stores server tools
func (mc *MCPClient) discoverServerTools(ctx context.Context, client *m.Client, serverURL string) error {
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
		return err
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

		inputSchema := make(map[string]interface{})
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

	mc.ServerTools[serverURL] = serverTools
	mc.Logger.Debug("found tools for server", "server", serverURL, "count", len(serverTools))

	return nil
}

func (mc *MCPClient) GetServers() []string {
	if !mc.Initialized {
		return nil
	}

	servers := make([]string, 0, len(mc.Clients))
	for serverURL := range mc.Clients {
		servers = append(servers, serverURL)
	}
	return servers
}

func (mc *MCPClient) GetServerTools(serverURL string) ([]Tool, error) {
	if !mc.Initialized {
		return nil, ErrClientNotInitialized
	}

	tools := mc.ServerTools[serverURL]
	if tools == nil {
		return nil, fmt.Errorf("no tools found for server %s", serverURL)
	}

	return tools, nil
}

// ConvertMCPToolsToChatCompletionTools converts MCP server tools to chat completion tools
func (mc *MCPClient) ConvertMCPToolsToChatCompletionTools(serverTools []Tool) []providers.ChatCompletionTool {
	tools := make([]providers.ChatCompletionTool, 0)
	for _, tool := range serverTools {
		description := tool.Description

		inputSchema := tool.InputSchema

		if inputSchema == nil {
			inputSchema = make(map[string]interface{})
		}

		tools = append(tools, providers.ChatCompletionTool{
			Type: "function",
			Function: providers.FunctionObject{
				Name:        "mcp_" + tool.Name,
				Description: description,
				Parameters:  (*providers.FunctionParameters)(&inputSchema),
			},
		})
	}

	return tools
}

// GetServerForTool returns the server URL that provides the specified tool
func (mc *MCPClient) GetServerForTool(toolName string) (string, error) {
	if !mc.Initialized {
		return "", fmt.Errorf("mcp client not initialized")
	}

	for serverURL, serverTools := range mc.ServerTools {
		for _, tool := range serverTools {
			if tool.Name == toolName {
				return serverURL, nil
			}
		}
	}

	return "", fmt.Errorf("tool %s not found on any server", toolName)
}

// GetAllChatCompletionTools returns all pre-converted chat completion tools from all servers
func (mc *MCPClient) GetAllChatCompletionTools() []providers.ChatCompletionTool {
	if !mc.Initialized {
		return []providers.ChatCompletionTool{}
	}
	return mc.ChatCompletionTools
}

// IsInitialized implements MCPClientInterface.
func (mc *MCPClient) IsInitialized() bool {
	return mc.Initialized
}

// GetServerStatus returns the status of a specific server
func (mc *MCPClient) GetServerStatus(serverURL string) ServerStatus {
	mc.statusMutex.RLock()
	defer mc.statusMutex.RUnlock()

	if status, exists := mc.ServerStatuses[serverURL]; exists {
		return status
	}
	return ServerStatusUnknown
}

// GetAllServerStatuses returns the status of all servers
func (mc *MCPClient) GetAllServerStatuses() map[string]ServerStatus {
	mc.statusMutex.RLock()
	defer mc.statusMutex.RUnlock()

	statusCopy := make(map[string]ServerStatus)
	for url, status := range mc.ServerStatuses {
		statusCopy[url] = status
	}
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

	client, exists := mc.Clients[serverURL]
	if !exists {
		mc.Logger.Debug("server client not found for health check", "server", serverURL, "component", "mcp_client")
		return
	}

	// Try to list tools as a health check (similar to how it works in initialization)
	var cursor *string
	_, err := client.ListTools(checkCtx, cursor)

	newStatus := ServerStatusAvailable
	if err != nil {
		newStatus = ServerStatusUnavailable
		if !mc.Config.MCP.DisableHealthcheckLogs {
			mc.Logger.Debug("server health check failed", "server", serverURL, "error", err, "component", "mcp_client")
		}

		mc.statusMutex.RLock()
		oldStatus := mc.ServerStatuses[serverURL]
		mc.statusMutex.RUnlock()

		if oldStatus == ServerStatusAvailable && mc.Config.MCP.EnableReconnect {
			mc.Logger.Info("server became unavailable, scheduling reconnection", "server", serverURL, "component", "mcp_client")
			go mc.attemptServerReconnection(ctx, serverURL)
		}
	} else if !mc.Config.MCP.DisableHealthcheckLogs {
		mc.Logger.Debug("server health check passed", "server", serverURL, "component", "mcp_client")
	}

	mc.statusMutex.Lock()
	oldStatus := mc.ServerStatuses[serverURL]
	mc.ServerStatuses[serverURL] = newStatus
	mc.statusMutex.Unlock()

	if oldStatus != newStatus {
		mc.Logger.Info("server status changed", "server", serverURL, "oldStatus", string(oldStatus), "newStatus", string(newStatus), "component", "mcp_client")
	}
}

// startBackgroundReconnection starts a background goroutine to reconnect failed servers
func (mc *MCPClient) startBackgroundReconnection(ctx context.Context, failedServers []string) {
	mc.Logger.Info("starting background reconnection for failed servers",
		"servers", failedServers,
		"interval", mc.Config.MCP.ReconnectInterval,
		"component", "mcp_client")

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
			mc.statusMutex.RLock()
			serversToReconnect := make([]string, 0)
			for serverURL := range reconnectingServers {
				if status, exists := mc.ServerStatuses[serverURL]; exists && status == ServerStatusUnavailable {
					serversToReconnect = append(serversToReconnect, serverURL)
				} else if status == ServerStatusAvailable {
					delete(reconnectingServers, serverURL)
					mc.Logger.Info("server successfully reconnected, removing from background reconnection",
						"server", serverURL, "component", "mcp_client")
				}
			}
			mc.statusMutex.RUnlock()

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
	mc.Logger.Info("attempting server reconnection", "server", serverURL, "component", "mcp_client")

	reconnectCtx, cancel := context.WithTimeout(ctx, mc.Config.MCP.ClientTimeout)
	defer cancel()

	if err := mc.initializeServer(reconnectCtx, serverURL); err != nil {
		mc.Logger.Info("server reconnection failed", "server", serverURL, "error", err, "component", "mcp_client")
		return
	}

	mc.Logger.Info("server successfully reconnected", "server", serverURL, "component", "mcp_client")
}
