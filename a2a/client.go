package a2a

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/inference-gateway/inference-gateway/config"
	"github.com/inference-gateway/inference-gateway/logger"
)

var (
	// ErrClientNotInitialized is returned when a client method is called before initialization
	ErrClientNotInitialized = errors.New("a2a client not initialized")

	// ErrAgentNotFound is returned when trying to use an agent that doesn't exist
	ErrAgentNotFound = errors.New("a2a agent not found")

	// ErrNoAgentURLs is returned when trying to initialize without any agent URLs
	ErrNoAgentURLs = errors.New("no a2a agent urls provided")

	// ErrNoAgentsInitialized is returned when no agents could be initialized
	ErrNoAgentsInitialized = errors.New("no a2a agents could be initialized")
)

// AgentStatus represents the status of an A2A agent
type AgentStatus string

const (
	// AgentStatusNA indicates agent status is not available
	AgentStatusNA AgentStatus = "n/a"
	// AgentStatusAvailable indicates agent is available and responding
	AgentStatusAvailable AgentStatus = "available"
	// AgentStatusUnavailable indicates agent is not responding
	AgentStatusUnavailable AgentStatus = "unavailable"
)

// A2AClientInterface defines the interface for A2A client implementations
//
//go:generate mockgen -source=client.go -destination=../tests/mocks/a2a/client.go -package=a2amocks
type A2AClientInterface interface {
	// InitializeAll discovers and connects to A2A agents
	InitializeAll(ctx context.Context) error

	// IsInitialized returns whether the client has been successfully initialized
	IsInitialized() bool

	// GetAgentCard retrieves an agent card from the specified agent URL
	GetAgentCard(ctx context.Context, agentURL string) (*AgentCard, error)

	// RefreshAgentCard forces a refresh of an agent card from the remote source
	RefreshAgentCard(ctx context.Context, agentURL string) (*AgentCard, error)

	// SendMessage sends a message to the specified agent (A2A's main task submission method)
	SendMessage(ctx context.Context, request *SendMessageRequest, agentURL string) (*SendMessageSuccessResponse, error)

	// SendStreamingMessage sends a streaming message to the specified agent
	SendStreamingMessage(ctx context.Context, request *SendStreamingMessageRequest, agentURL string) (<-chan []byte, error)

	// GetTask retrieves the status of a task
	GetTask(ctx context.Context, request *GetTaskRequest, agentURL string) (*GetTaskSuccessResponse, error)

	// CancelTask cancels a running task
	CancelTask(ctx context.Context, request *CancelTaskRequest, agentURL string) (*CancelTaskSuccessResponse, error)

	// GetAgents returns the list of A2A agent URLs
	GetAgents() []string

	// GetAgentCapabilities returns the agent capabilities map
	GetAgentCapabilities() map[string]AgentCapabilities

	// GetAgentSkills returns the skills available for the specified agent
	GetAgentSkills(agentURL string) ([]AgentSkill, error)

	// GetAgentStatus returns the status of a specific agent
	GetAgentStatus(agentURL string) AgentStatus

	// GetAllAgentStatuses returns the status of all agents
	GetAllAgentStatuses() map[string]AgentStatus

	// StartStatusPolling starts the background status polling goroutine
	StartStatusPolling(ctx context.Context)

	// StopStatusPolling stops the background status polling goroutine
	StopStatusPolling()
}

// A2AClient provides methods to interact with A2A agents
type A2AClient struct {
	AgentURLs         []string
	HTTPClient        *http.Client
	Logger            logger.Logger
	Config            config.Config
	AgentCards        map[string]*AgentCard
	AgentCapabilities map[string]AgentCapabilities
	Initialized       bool
	
	// Status tracking
	AgentStatuses    map[string]AgentStatus
	statusMutex      sync.RWMutex
	pollingCancel    context.CancelFunc
	pollingDone      chan struct{}
}

// NewA2AClient creates a new A2A client instance
func NewA2AClient(cfg config.Config, log logger.Logger) *A2AClient {
	agentURLs := parseAgentURLs(cfg.A2A.Agents)

	var tlsMinVersion uint16 = tls.VersionTLS12
	if cfg.Client.TlsMinVersion == "TLS13" {
		tlsMinVersion = tls.VersionTLS13
	}

	return &A2AClient{
		AgentURLs: agentURLs,
		HTTPClient: &http.Client{
			Timeout: cfg.A2A.ClientTimeout,
			Transport: &http.Transport{
				MaxIdleConns:        cfg.Client.MaxIdleConns,
				MaxIdleConnsPerHost: cfg.Client.MaxIdleConnsPerHost,
				IdleConnTimeout:     cfg.Client.IdleConnTimeout,
				TLSClientConfig: &tls.Config{
					MinVersion: tlsMinVersion,
				},
				ForceAttemptHTTP2:     true,
				DisableCompression:    cfg.Client.DisableCompression,
				ResponseHeaderTimeout: cfg.Client.ResponseHeaderTimeout,
				ExpectContinueTimeout: cfg.Client.ExpectContinueTimeout,
			},
		},
		Logger:            log,
		Config:            cfg,
		AgentCards:        make(map[string]*AgentCard),
		AgentCapabilities: make(map[string]AgentCapabilities),
		Initialized:       false,
		AgentStatuses:     make(map[string]AgentStatus),
		pollingDone:       make(chan struct{}),
	}
}

// parseAgentURLs splits the comma-separated agent URLs string
func parseAgentURLs(agents string) []string {
	if agents == "" {
		return nil
	}

	urls := strings.Split(agents, ",")
	result := make([]string, 0, len(urls))
	for _, url := range urls {
		trimmed := strings.TrimSpace(url)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

// InitializeAll discovers and connects to A2A agents
func (c *A2AClient) InitializeAll(ctx context.Context) error {
	if len(c.AgentURLs) == 0 {
		return ErrNoAgentURLs
	}

	var lastError error
	successfulInitializations := 0

	// Initialize all agent statuses to "n/a"
	c.statusMutex.Lock()
	for _, agentURL := range c.AgentURLs {
		c.AgentStatuses[agentURL] = AgentStatusNA
	}
	c.statusMutex.Unlock()

	for _, agentURL := range c.AgentURLs {
		if err := c.initializeAgent(ctx, agentURL); err != nil {
			c.Logger.Error("failed to initialize a2a agent", err, "agentURL", agentURL, "component", "a2a_client")
			lastError = err
			continue
		}

		successfulInitializations++
		c.Logger.Info("successfully initialized a2a agent", "agentURL", agentURL, "component", "a2a_client")
	}

	if successfulInitializations == 0 {
		if lastError != nil {
			return fmt.Errorf("%w: %v", ErrNoAgentsInitialized, lastError)
		}
		return ErrNoAgentsInitialized
	}

	c.Initialized = true
	c.Logger.Info("a2a client initialization completed", "successful_agents", successfulInitializations, "total_agents", len(c.AgentURLs), "component", "a2a_client")

	return nil
}

// initializeAgent initializes a single agent by fetching its agent card
func (c *A2AClient) initializeAgent(ctx context.Context, agentURL string) error {
	agentCard, err := c.fetchAgentCardFromRemote(ctx, agentURL)
	if err != nil {
		return fmt.Errorf("failed to get agent card: %w", err)
	}

	c.AgentCards[agentURL] = agentCard
	c.AgentCapabilities[agentURL] = agentCard.Capabilities

	return nil
}

// IsInitialized returns whether the client has been successfully initialized
func (c *A2AClient) IsInitialized() bool {
	return c.Initialized
}

// GetAgentCard retrieves an agent card from the specified agent URL
// First checks the cache, then fetches from remote if not found
func (c *A2AClient) GetAgentCard(ctx context.Context, agentURL string) (*AgentCard, error) {
	if !c.isValidAgentURL(agentURL) {
		return nil, ErrAgentNotFound
	}

	if cachedCard, exists := c.AgentCards[agentURL]; exists {
		c.Logger.Debug("retrieved agent card from cache", "agentURL", agentURL, "component", "a2a_client")
		return cachedCard, nil
	}

	agentCard, err := c.fetchAgentCardFromRemote(ctx, agentURL)
	if err != nil {
		return nil, err
	}

	c.AgentCards[agentURL] = agentCard
	c.AgentCapabilities[agentURL] = agentCard.Capabilities

	return agentCard, nil
}

// fetchAgentCardFromRemote fetches an agent card from the remote agent URL
func (c *A2AClient) fetchAgentCardFromRemote(ctx context.Context, agentURL string) (*AgentCard, error) {
	cardURL, err := url.JoinPath(agentURL, ".well-known/agent.json")
	if err != nil {
		return nil, fmt.Errorf("failed to build agent card URL: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "GET", cardURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "inference-gateway-a2a-client/1.0")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("agent card request failed with status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	var agentCard AgentCard
	if err := json.Unmarshal(body, &agentCard); err != nil {
		return nil, fmt.Errorf("failed to unmarshal agent card: %w", err)
	}

	return &agentCard, nil
}

// RefreshAgentCard forces a refresh of an agent card from the remote source
func (c *A2AClient) RefreshAgentCard(ctx context.Context, agentURL string) (*AgentCard, error) {
	if !c.isValidAgentURL(agentURL) {
		return nil, ErrAgentNotFound
	}

	agentCard, err := c.fetchAgentCardFromRemote(ctx, agentURL)
	if err != nil {
		return nil, err
	}

	c.AgentCards[agentURL] = agentCard
	c.AgentCapabilities[agentURL] = agentCard.Capabilities

	return agentCard, nil
}

// SendMessage sends a message to the specified agent (A2A's main task submission method)
func (c *A2AClient) SendMessage(ctx context.Context, request *SendMessageRequest, agentURL string) (*SendMessageSuccessResponse, error) {
	if !c.Initialized {
		return nil, ErrClientNotInitialized
	}

	if !c.isValidAgentURL(agentURL) {
		return nil, ErrAgentNotFound
	}

	response, err := c.makeJSONRPCRequest(ctx, request, agentURL, &SendMessageSuccessResponse{})
	if err != nil {
		return nil, err
	}

	return response.(*SendMessageSuccessResponse), nil
}

// SendStreamingMessage sends a streaming message to the specified agent
func (c *A2AClient) SendStreamingMessage(ctx context.Context, request *SendStreamingMessageRequest, agentURL string) (<-chan []byte, error) {
	if !c.Initialized {
		return nil, ErrClientNotInitialized
	}

	if !c.isValidAgentURL(agentURL) {
		return nil, ErrAgentNotFound
	}

	rpcURL, err := url.JoinPath(agentURL, "a2a")
	if err != nil {
		return nil, fmt.Errorf("failed to build JSON-RPC URL: %w", err)
	}

	requestBody, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", rpcURL, bytes.NewReader(requestBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("User-Agent", "inference-gateway-a2a-client/1.0")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make streaming request: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("streaming JSON-RPC request failed with status %d", resp.StatusCode)
	}

	stream := make(chan []byte, 100)
	go func() {
		defer resp.Body.Close()
		defer close(stream)

		reader := bufio.NewReaderSize(resp.Body, 4096)

		for {
			select {
			case <-ctx.Done():
				c.Logger.Debug("streaming cancelled due to context", "agent_url", agentURL)
				return
			default:
			}

			line, err := reader.ReadBytes('\n')
			if err != nil {
				if err != io.EOF {
					c.Logger.Error("error reading stream", err, "agent_url", agentURL)
				} else {
					c.Logger.Debug("stream ended gracefully", "agent_url", agentURL)
				}
				return
			}

			if len(line) > 0 {
				select {
				case stream <- line:
				case <-ctx.Done():
					c.Logger.Debug("streaming cancelled while sending data", "agent_url", agentURL)
					return
				}
			}
		}
	}()

	return stream, nil
}

// GetTask retrieves the status of a task
func (c *A2AClient) GetTask(ctx context.Context, request *GetTaskRequest, agentURL string) (*GetTaskSuccessResponse, error) {
	if !c.Initialized {
		return nil, ErrClientNotInitialized
	}

	if !c.isValidAgentURL(agentURL) {
		return nil, ErrAgentNotFound
	}

	response, err := c.makeJSONRPCRequest(ctx, request, agentURL, &GetTaskSuccessResponse{})
	if err != nil {
		return nil, err
	}

	return response.(*GetTaskSuccessResponse), nil
}

// CancelTask cancels a running task
func (c *A2AClient) CancelTask(ctx context.Context, request *CancelTaskRequest, agentURL string) (*CancelTaskSuccessResponse, error) {
	if !c.Initialized {
		return nil, ErrClientNotInitialized
	}

	if !c.isValidAgentURL(agentURL) {
		return nil, ErrAgentNotFound
	}

	response, err := c.makeJSONRPCRequest(ctx, request, agentURL, &CancelTaskSuccessResponse{})
	if err != nil {
		return nil, err
	}

	return response.(*CancelTaskSuccessResponse), nil
}

// GetAgents returns the list of A2A agent URLs
func (c *A2AClient) GetAgents() []string {
	return c.AgentURLs
}

// GetAgentCapabilities returns the agent capabilities map
func (c *A2AClient) GetAgentCapabilities() map[string]AgentCapabilities {
	return c.AgentCapabilities
}

// GetAgentSkills returns the skills available for the specified agent
func (c *A2AClient) GetAgentSkills(agentURL string) ([]AgentSkill, error) {
	agentCard, exists := c.AgentCards[agentURL]
	if !exists {
		return nil, ErrAgentNotFound
	}

	return agentCard.Skills, nil
}

// makeJSONRPCRequest makes a JSON-RPC request to the specified agent
func (c *A2AClient) makeJSONRPCRequest(ctx context.Context, request interface{}, agentURL string, response interface{}) (interface{}, error) {
	rpcURL, err := url.JoinPath(agentURL, "a2a")
	if err != nil {
		return nil, fmt.Errorf("failed to build JSON-RPC URL: %w", err)
	}

	requestBody, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", rpcURL, bytes.NewReader(requestBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "inference-gateway-a2a-client/1.0")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("JSON-RPC request failed with status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if err := json.Unmarshal(body, response); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return response, nil
}

// isValidAgentURL checks if the agent URL is in the list of configured agents
func (c *A2AClient) isValidAgentURL(agentURL string) bool {
	for _, url := range c.AgentURLs {
		if url == agentURL {
			return true
		}
	}
	return false
}

// GetAgentStatus returns the status of a specific agent
func (c *A2AClient) GetAgentStatus(agentURL string) AgentStatus {
	c.statusMutex.RLock()
	defer c.statusMutex.RUnlock()
	
	if status, exists := c.AgentStatuses[agentURL]; exists {
		return status
	}
	return AgentStatusNA
}

// GetAllAgentStatuses returns the status of all agents
func (c *A2AClient) GetAllAgentStatuses() map[string]AgentStatus {
	c.statusMutex.RLock()
	defer c.statusMutex.RUnlock()
	
	statusCopy := make(map[string]AgentStatus)
	for url, status := range c.AgentStatuses {
		statusCopy[url] = status
	}
	return statusCopy
}

// StartStatusPolling starts the background status polling goroutine
func (c *A2AClient) StartStatusPolling(ctx context.Context) {
	if !c.Config.A2A.Enable {
		c.Logger.Debug("a2a status polling disabled, not starting background polling")
		return
	}

	pollingCtx, cancel := context.WithCancel(ctx)
	c.pollingCancel = cancel

	go c.statusPollingLoop(pollingCtx)
	c.Logger.Info("started a2a agent status polling", "interval", c.Config.A2A.PollingInterval, "component", "a2a_client")
}

// StopStatusPolling stops the background status polling goroutine
func (c *A2AClient) StopStatusPolling() {
	if c.pollingCancel != nil {
		c.pollingCancel()
		<-c.pollingDone
		c.Logger.Info("stopped a2a agent status polling", "component", "a2a_client")
	}
}

// statusPollingLoop continuously polls agent health status
func (c *A2AClient) statusPollingLoop(ctx context.Context) {
	defer close(c.pollingDone)
	
	ticker := time.NewTicker(c.Config.A2A.PollingInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			c.pollAgentStatuses(ctx)
		}
	}
}

// pollAgentStatuses checks the health status of all agents
func (c *A2AClient) pollAgentStatuses(ctx context.Context) {
	for _, agentURL := range c.AgentURLs {
		go c.checkAgentHealth(ctx, agentURL)
	}
}

// checkAgentHealth checks the health of a single agent
func (c *A2AClient) checkAgentHealth(ctx context.Context, agentURL string) {
	// Create a timeout context for the health check
	checkCtx, cancel := context.WithTimeout(ctx, c.Config.A2A.PollingTimeout)
	defer cancel()

	// Perform health check by trying to fetch the agent card
	_, err := c.fetchAgentCardFromRemote(checkCtx, agentURL)
	
	// Update agent status based on health check result
	newStatus := AgentStatusAvailable
	if err != nil {
		newStatus = AgentStatusUnavailable
		c.Logger.Debug("agent health check failed", "agentURL", agentURL, "error", err, "component", "a2a_client")
	} else {
		c.Logger.Debug("agent health check passed", "agentURL", agentURL, "component", "a2a_client")
	}

	// Update status with thread safety
	c.statusMutex.Lock()
	oldStatus := c.AgentStatuses[agentURL]
	c.AgentStatuses[agentURL] = newStatus
	c.statusMutex.Unlock()

	// Log status changes
	if oldStatus != newStatus {
		c.Logger.Info("agent status changed", "agentURL", agentURL, "oldStatus", string(oldStatus), "newStatus", string(newStatus), "component", "a2a_client")
	}
}
