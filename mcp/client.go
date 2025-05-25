package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
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

// MCPClientInterface defines the interface for MCP client implementations
//
//go:generate mockgen -source=client.go -destination=../tests/mocks/mcp_client.go -package=mocks
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
}

// NewClient creates a new MCP client for a given server URL
func (mc *MCPClient) NewClient(url string) *m.Client {
	httpClient := &http.Client{
		Timeout: mc.Config.MCP.ClientTimeout,
		Transport: &http.Transport{
			DialContext: (&net.Dialer{
				Timeout:   mc.Config.MCP.DialTimeout,
				KeepAlive: 30 * time.Second,
			}).DialContext,
			TLSHandshakeTimeout:   mc.Config.MCP.TlsHandshakeTimeout,
			ResponseHeaderTimeout: mc.Config.MCP.ResponseHeaderTimeout,
			ExpectContinueTimeout: mc.Config.MCP.ExpectContinueTimeout,
		},
	}

	httpTransport := transport.NewHTTPClientTransport(url).WithClient(httpClient)

	return m.NewClient(httpTransport)
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
		Content: make([]interface{}, len(result.Content)),
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

// InitializeAll implements MCPClientInterface.
func (mc *MCPClient) InitializeAll(ctx context.Context) error {
	if len(mc.ServerURLs) == 0 {
		return ErrNoServerURLs
	}

	for _, url := range mc.ServerURLs {
		mc.Logger.Debug("MCP: Initializing client", "server", url)

		client := mc.NewClient(url)

		mc.Logger.Debug("MCP: Attempting client initialization with timeout", "server", url, "timeout", mc.Config.MCP.RequestTimeout.String())
		result, err := client.Initialize(ctx)
		if err != nil {
			if errors.Is(err, context.DeadlineExceeded) {
				mc.Logger.Error("MCP: Client initialization timed out", err, "server", url)
			} else {
				mc.Logger.Error("MCP: Failed to initialize client", err, "server", url)
				mc.Logger.Debug("MCP: Client initialization error details", "error", err.Error(), "server", url)
			}
			continue
		}

		mc.Logger.Debug("MCP: Client initialized successfully for server", "server", url)
		mc.Clients[url] = client

		capabilities := ServerCapabilities{
			Completions:  make(map[string]interface{}),
			Experimental: make(map[string]interface{}),
			Logging:      make(map[string]interface{}),
			Prompts:      make(map[string]interface{}),
			Resources:    make(map[string]interface{}),
			Tools:        make(map[string]interface{}),
		}

		if capBytes, err := json.Marshal(result.Capabilities); err == nil {
			var capMap map[string]interface{}
			if err = json.Unmarshal(capBytes, &capMap); err == nil {
				if comp, ok := capMap["completions"].(map[string]interface{}); ok {
					capabilities.Completions = comp
				}
				if exp, ok := capMap["experimental"].(map[string]interface{}); ok {
					capabilities.Experimental = exp
				}
				if log, ok := capMap["logging"].(map[string]interface{}); ok {
					capabilities.Logging = log
				}
				if prompts, ok := capMap["prompts"].(map[string]interface{}); ok {
					capabilities.Prompts = prompts
				}
				if resources, ok := capMap["resources"].(map[string]interface{}); ok {
					capabilities.Resources = resources
				}
				if tools, ok := capMap["tools"].(map[string]interface{}); ok {
					capabilities.Tools = tools
				}
			}
		}

		mc.ServerCapabilities[url] = capabilities

		mc.Logger.Debug("MCP: Fetching available tools", "server", url)

		toolsCtx, toolsCancel := context.WithTimeout(ctx, mc.Config.MCP.RequestTimeout)
		defer toolsCancel()

		mc.Logger.Debug("MCP: Attempting to list tools with timeout", "server", url, "timeout", mc.Config.MCP.RequestTimeout.String())
		toolsResult, err := client.ListTools(toolsCtx, nil)
		if err != nil {
			if errors.Is(err, context.DeadlineExceeded) {
				mc.Logger.Error("MCP: Tools listing timed out", err, "server", url)
			} else {
				mc.Logger.Error("MCP: Failed to list tools", err, "server", url)
				mc.Logger.Debug("MCP: Tools listing error details", "error", err.Error(), "server", url)
			}
		} else {
			mc.Logger.Debug("MCP: Successfully retrieved tools list", "server", url)
			serverTools := make([]Tool, 0, len(toolsResult.Tools))

			for _, tool := range toolsResult.Tools {
				enhancedDesc := tool.Description
				if enhancedDesc == nil {
					enhancedDesc = new(string)
					*enhancedDesc = ""
				}
				*enhancedDesc += fmt.Sprintf(" [IMPORTANT: Must specify mcpServer=\"%s\" when calling this tool]", url)

				inputSchema := make(map[string]interface{})
				if tool.InputSchema != nil {
					if inputBytes, err := json.Marshal(tool.InputSchema); err == nil {
						_ = json.Unmarshal(inputBytes, &inputSchema)
					}
				}

				serverTools = append(serverTools, Tool{
					Name:        tool.Name,
					Description: *enhancedDesc,
					Inputschema: inputSchema,
				})
			}

			mc.ServerTools[url] = serverTools
			mc.Logger.Debug("MCP: Found tools for server", "server", url, "count", len(serverTools), "tools", serverTools)
		}

		mc.Logger.Debug("MCP: Client initialized successfully", "server", url)
	}

	if len(mc.Clients) == 0 {
		return ErrNoClientsInitialized
	}

	mc.Logger.Debug("MCP: Pre-converting all tools to chat completion format")
	allChatCompletionTools := make([]providers.ChatCompletionTool, 0)

	for serverURL, serverTools := range mc.ServerTools {
		if len(serverTools) == 0 {
			mc.Logger.Debug("MCP: No tools to convert for server", "server", serverURL)
			continue
		}

		chatTools := mc.ConvertMCPToolsToChatCompletionTools(serverTools)
		mc.Logger.Debug("MCP: Converted tools for server", "server", serverURL, "count", len(chatTools))
		allChatCompletionTools = append(allChatCompletionTools, chatTools...)
	}

	mc.ChatCompletionTools = allChatCompletionTools
	mc.Logger.Debug("MCP: Total pre-converted tools", "count", len(mc.ChatCompletionTools))

	mc.Initialized = true
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

		inputSchema := tool.Inputschema

		if inputSchema == nil {
			inputSchema = make(map[string]interface{})
		}

		props, ok := inputSchema["properties"].(map[string]interface{})
		if !ok {
			props = make(map[string]interface{})
			inputSchema["properties"] = props
		}

		if _, exists := props["mcpServer"]; !exists {
			props["mcpServer"] = map[string]interface{}{
				"type":        "string",
				"description": "Required. The MCP server URL to use for this tool call. Analyze the tool description to determine which server to use.",
			}
		}

		required, ok := inputSchema["required"].([]interface{})
		if !ok {
			required = []interface{}{}
		}

		mcpServerRequired := false
		for _, req := range required {
			if reqStr, ok := req.(string); ok && reqStr == "mcpServer" {
				mcpServerRequired = true
				break
			}
		}

		if !mcpServerRequired {
			required = append(required, "mcpServer")
			inputSchema["required"] = required
		}

		tools = append(tools, providers.ChatCompletionTool{
			Type: "function",
			Function: providers.FunctionObject{
				Name:        tool.Name,
				Description: &description,
				Parameters:  (*providers.FunctionParameters)(&inputSchema),
			},
		})
	}

	return tools
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
