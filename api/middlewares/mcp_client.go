package middlewares

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/inference-gateway/inference-gateway/logger"
)

// MCPClient provides methods to interact with MCP servers
type MCPClient struct {
	ServerURLs []string
	AuthToken  string
	HTTPClient *http.Client
	EnableSSE  bool
	Logger     logger.Logger
}

// NewMCPClient creates a new client for interacting with MCP servers
func NewMCPClient(serverURLs []string, authToken string, enableSSE bool, logger logger.Logger) *MCPClient {
	return &MCPClient{
		ServerURLs: serverURLs,
		AuthToken:  authToken,
		HTTPClient: &http.Client{},
		EnableSSE:  enableSSE,
		Logger:     logger,
	}
}

// DiscoverCapabilities queries MCP servers to discover their capabilities
func (c *MCPClient) DiscoverCapabilities(ctx context.Context) ([]map[string]interface{}, error) {
	var allCapabilities []map[string]interface{}

	for _, serverURL := range c.ServerURLs {
		capabilities, err := c.discoverServerCapabilities(ctx, serverURL)
		if err != nil {
			c.Logger.Error("Failed to discover capabilities from server", err, "server", serverURL)
			continue
		}

		allCapabilities = append(allCapabilities, capabilities)
	}

	if len(allCapabilities) == 0 {
		return nil, fmt.Errorf("failed to discover capabilities from any MCP server")
	}

	return allCapabilities, nil
}

// discoverServerCapabilities queries a single MCP server to discover its capabilities
func (c *MCPClient) discoverServerCapabilities(ctx context.Context, serverURL string) (map[string]interface{}, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", serverURL+"/capabilities", nil)
	if err != nil {
		return nil, err
	}

	if c.AuthToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.AuthToken)
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to discover capabilities, status code: %d", resp.StatusCode)
	}

	var capabilities map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&capabilities); err != nil {
		return nil, err
	}

	// Add server URL to capabilities for routing tool calls
	capabilities["_server_url"] = serverURL

	return capabilities, nil
}

// ExecuteTool invokes a tool on the appropriate MCP server
func (c *MCPClient) ExecuteTool(ctx context.Context, toolName string, params interface{}, serverURL string) (map[string]interface{}, error) {
	var paramsMap map[string]interface{}

	// Handle different types of params
	switch p := params.(type) {
	case map[string]interface{}:
		paramsMap = p
	case string:
		if err := json.Unmarshal([]byte(p), &paramsMap); err != nil {
			return nil, fmt.Errorf("invalid tool parameters: %v", err)
		}
	default:
		return nil, fmt.Errorf("unsupported tool parameters type")
	}

	toolRequest := map[string]interface{}{
		"name":   toolName,
		"params": paramsMap,
	}

	requestBody, err := json.Marshal(toolRequest)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", serverURL+"/tools", bytes.NewReader(requestBody))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	if c.AuthToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.AuthToken)
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to execute tool, status code: %d", resp.StatusCode)
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return result, nil
}

// StreamChatWithTools sends a chat request to the MCP server with tool capabilities and returns a streaming response
func (c *MCPClient) StreamChatWithTools(ctx context.Context, messages []map[string]interface{}, serverURL string, callback func(chunk map[string]interface{}) error) error {
	if !c.EnableSSE {
		return fmt.Errorf("SSE streaming is not enabled")
	}

	chatRequest := map[string]interface{}{
		"messages": messages,
	}

	requestBody, err := json.Marshal(chatRequest)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", serverURL+"/chat", bytes.NewReader(requestBody))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")
	if c.AuthToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.AuthToken)
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to start chat stream, status code: %d", resp.StatusCode)
	}

	reader := bufio.NewReader(resp.Body)
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				break
			}
			return err
		}

		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "data: ") {
			continue
		}

		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			break
		}

		var chunk map[string]interface{}
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue
		}

		if err := callback(chunk); err != nil {
			return err
		}
	}

	return nil
}
