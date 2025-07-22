package a2a

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/inference-gateway/a2a/adk"
	"github.com/inference-gateway/a2a/adk/client"
	"github.com/inference-gateway/inference-gateway/config"
	"github.com/inference-gateway/inference-gateway/logger"
)

var (
	ErrClientNotInitialized = errors.New("a2a client not initialized")
	ErrAgentNotFound        = errors.New("a2a agent not found")
	ErrNoAgentURLs          = errors.New("no a2a agent urls provided")
	ErrNoAgentsInitialized  = errors.New("no a2a agents could be initialized")
)

// AgentStatus represents the status of an A2A agent
type AgentStatus string

const (
	AgentStatusUnknown     AgentStatus = "unknown"
	AgentStatusAvailable   AgentStatus = "available"
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
	GetAgentCard(ctx context.Context, agentURL string) (*adk.AgentCard, error)

	// RefreshAgentCard forces a refresh of an agent card from the remote source
	RefreshAgentCard(ctx context.Context, agentURL string) (*adk.AgentCard, error)

	// SendMessage sends a message to the specified agent (A2A's main task submission method)
	SendMessage(ctx context.Context, request *adk.SendMessageRequest, agentURL string) (*adk.SendMessageSuccessResponse, error)

	// SendStreamingMessage sends a streaming message to the specified agent
	SendStreamingMessage(ctx context.Context, request *adk.SendStreamingMessageRequest, agentURL string) (<-chan []byte, error)

	// GetTask retrieves the status of a task
	GetTask(ctx context.Context, request *adk.GetTaskRequest, agentURL string) (*adk.GetTaskSuccessResponse, error)

	// CancelTask cancels a running task
	CancelTask(ctx context.Context, request *adk.CancelTaskRequest, agentURL string) (*adk.CancelTaskSuccessResponse, error)

	// GetAgents returns the list of A2A agent URLs
	GetAgents() []string

	// GetAgentCapabilities returns the agent capabilities map
	GetAgentCapabilities() map[string]adk.AgentCapabilities

	// GetAgentSkills returns the skills available for the specified agent
	GetAgentSkills(agentURL string) ([]adk.AgentSkill, error)

	// GetAgentStatus returns the status of a specific agent
	GetAgentStatus(agentURL string) AgentStatus

	// GetAllAgentStatuses returns the status of all agents
	GetAllAgentStatuses() map[string]AgentStatus

	// StartStatusPolling starts the background status polling goroutine
	StartStatusPolling(ctx context.Context)

	// StopStatusPolling stops the background status polling goroutine
	StopStatusPolling()
}

// A2AClient provides methods to interact with A2A agents using the external client library
type A2AClient struct {
	AgentURLs         []string
	Logger            logger.Logger
	Config            config.Config
	AgentClients      map[string]client.A2AClient
	AgentCards        map[string]*adk.AgentCard
	AgentCapabilities map[string]adk.AgentCapabilities
	Initialized       bool
	AgentStatuses     map[string]AgentStatus
	statusMutex       sync.RWMutex
	pollingCancel     context.CancelFunc
	pollingDone       chan struct{}
}

// NewA2AClient creates a new A2A client instance using the external client library
func NewA2AClient(cfg config.Config, log logger.Logger) *A2AClient {
	agentURLs := parseAgentURLs(cfg.A2A.Agents)

	return &A2AClient{
		AgentURLs:         agentURLs,
		Logger:            log,
		Config:            cfg,
		AgentClients:      make(map[string]client.A2AClient),
		AgentCards:        make(map[string]*adk.AgentCard),
		AgentCapabilities: make(map[string]adk.AgentCapabilities),
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

// InitializeAll discovers and connects to A2A agents using the external client library
func (c *A2AClient) InitializeAll(ctx context.Context) error {
	if len(c.AgentURLs) == 0 {
		return ErrNoAgentURLs
	}

	var lastError error
	successfulInitializations := 0
	failedAgents := make([]string, 0)

	c.statusMutex.Lock()
	for _, agentURL := range c.AgentURLs {
		c.AgentStatuses[agentURL] = AgentStatusUnknown
	}
	c.statusMutex.Unlock()

	for _, agentURL := range c.AgentURLs {
		if err := c.initializeAgent(ctx, agentURL); err != nil {
			c.Logger.Error("failed to initialize a2a agent", err, "agentURL", agentURL, "component", "a2a_client")
			lastError = err
			failedAgents = append(failedAgents, agentURL)
			continue
		}

		successfulInitializations++
		c.Logger.Info("successfully initialized a2a agent", "agentURL", agentURL, "component", "a2a_client")
	}

	c.Initialized = true

	if successfulInitializations == 0 {
		c.Logger.Warn("no agents successfully initialized, but enabling A2A with background reconnection",
			"total_agents", len(c.AgentURLs),
			"failed_agents", len(failedAgents),
			"component", "a2a_client")

		if c.Config.A2A.EnableReconnect && len(failedAgents) > 0 {
			go c.startBackgroundReconnection(ctx, failedAgents)
		}

		if lastError != nil {
			return fmt.Errorf("%w: %v", ErrNoAgentsInitialized, lastError)
		}
		return ErrNoAgentsInitialized
	}

	c.Logger.Info("a2a client initialization completed",
		"successful_agents", successfulInitializations,
		"failed_agents", len(failedAgents),
		"total_agents", len(c.AgentURLs),
		"component", "a2a_client")

	if c.Config.A2A.EnableReconnect && len(failedAgents) > 0 {
		c.Logger.Info("starting background reconnection for failed agents",
			"failed_agents", failedAgents,
			"component", "a2a_client")
		go c.startBackgroundReconnection(ctx, failedAgents)
	}

	return nil
}

// initializeAgent initializes a single agent using the external client library with retry logic
func (c *A2AClient) initializeAgent(ctx context.Context, agentURL string) error {
	config := &client.Config{
		BaseURL: agentURL,
		Timeout: c.Config.A2A.ClientTimeout,
	}

	agentClient := client.NewClientWithConfig(config)
	c.AgentClients[agentURL] = agentClient

	maxRetries := c.Config.A2A.MaxRetries
	initialBackoff := c.Config.A2A.InitialBackoff
	var lastErr error

	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			backoffDelay := time.Duration(float64(initialBackoff) * float64(uint(1)<<uint(attempt-1)))
			if backoffDelay > c.Config.A2A.RetryInterval {
				backoffDelay = c.Config.A2A.RetryInterval
			}

			c.Logger.Debug("retrying agent initialization",
				"agentURL", agentURL,
				"attempt", attempt+1,
				"max_attempts", maxRetries+1,
				"backoff_delay", backoffDelay,
				"component", "a2a_client")

			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(backoffDelay):
			}
		}

		agentCard, err := agentClient.GetAgentCard(ctx)
		if err != nil {
			lastErr = fmt.Errorf("failed to get agent card: %w", err)
			c.Logger.Debug("failed to get agent card",
				"agentURL", agentURL,
				"attempt", attempt+1,
				"error", err,
				"component", "a2a_client")
			continue
		}

		c.AgentCards[agentURL] = agentCard
		c.AgentCapabilities[agentURL] = agentCard.Capabilities

		c.statusMutex.Lock()
		c.AgentStatuses[agentURL] = AgentStatusAvailable
		c.statusMutex.Unlock()

		c.Logger.Info("agent initialized successfully",
			"agentURL", agentURL,
			"attempts_used", attempt+1,
			"component", "a2a_client")

		return nil
	}

	c.statusMutex.Lock()
	c.AgentStatuses[agentURL] = AgentStatusUnavailable
	c.statusMutex.Unlock()

	return fmt.Errorf("failed to initialize agent after %d attempts: %w", maxRetries+1, lastErr)
}

// IsInitialized returns whether the client has been successfully initialized
func (c *A2AClient) IsInitialized() bool {
	return c.Initialized
}

// GetAgentCard retrieves an agent card from the specified agent URL
// First checks the cache, then fetches from remote if not found
func (c *A2AClient) GetAgentCard(ctx context.Context, agentURL string) (*adk.AgentCard, error) {
	if !c.isValidAgentURL(agentURL) {
		return nil, ErrAgentNotFound
	}

	if cachedCard, exists := c.AgentCards[agentURL]; exists {
		c.Logger.Debug("retrieved agent card from cache", "agentURL", agentURL, "component", "a2a_client")
		return cachedCard, nil
	}

	agentClient, exists := c.AgentClients[agentURL]
	if !exists {
		return nil, ErrAgentNotFound
	}

	agentCard, err := agentClient.GetAgentCard(ctx)
	if err != nil {
		return nil, err
	}

	c.AgentCards[agentURL] = agentCard
	c.AgentCapabilities[agentURL] = agentCard.Capabilities

	return agentCard, nil
}

// RefreshAgentCard forces a refresh of an agent card from the remote source using the external client
func (c *A2AClient) RefreshAgentCard(ctx context.Context, agentURL string) (*adk.AgentCard, error) {
	if !c.isValidAgentURL(agentURL) {
		return nil, ErrAgentNotFound
	}

	agentClient, exists := c.AgentClients[agentURL]
	if !exists {
		return nil, ErrAgentNotFound
	}

	agentCard, err := agentClient.GetAgentCard(ctx)
	if err != nil {
		return nil, err
	}

	c.AgentCards[agentURL] = agentCard
	c.AgentCapabilities[agentURL] = agentCard.Capabilities

	return agentCard, nil
}

// SendMessage sends a message to the specified agent using the external client library
func (c *A2AClient) SendMessage(ctx context.Context, request *adk.SendMessageRequest, agentURL string) (*adk.SendMessageSuccessResponse, error) {
	if !c.Initialized {
		return nil, ErrClientNotInitialized
	}

	if !c.isValidAgentURL(agentURL) {
		return nil, ErrAgentNotFound
	}

	agentClient, exists := c.AgentClients[agentURL]
	if !exists {
		return nil, ErrAgentNotFound
	}

	response, err := agentClient.SendTask(ctx, request.Params)
	if err != nil {
		return nil, err
	}

	return &adk.SendMessageSuccessResponse{
		ID:      response.ID,
		JSONRPC: response.JSONRPC,
		Result:  response.Result,
	}, nil
}

// SendStreamingMessage sends a streaming message to the specified agent using the external client
func (c *A2AClient) SendStreamingMessage(ctx context.Context, request *adk.SendStreamingMessageRequest, agentURL string) (<-chan []byte, error) {
	if !c.Initialized {
		return nil, ErrClientNotInitialized
	}

	if !c.isValidAgentURL(agentURL) {
		return nil, ErrAgentNotFound
	}

	agentClient, exists := c.AgentClients[agentURL]
	if !exists {
		return nil, ErrAgentNotFound
	}

	eventChan := make(chan interface{}, 100)

	stream := make(chan []byte, 100)

	go func() {
		defer close(eventChan)
		err := agentClient.SendTaskStreaming(ctx, request.Params, eventChan)
		if err != nil {
			c.Logger.Error("streaming task failed", err, "agent_url", agentURL)
		}
	}()

	go func() {
		defer close(stream)
		for {
			select {
			case event, ok := <-eventChan:
				if !ok {
					return
				}
				if eventBytes, err := json.Marshal(event); err == nil {
					select {
					case stream <- eventBytes:
					case <-ctx.Done():
						c.Logger.Debug("streaming cancelled while sending data", "agent_url", agentURL)
						return
					}
				}
			case <-ctx.Done():
				c.Logger.Debug("streaming cancelled due to context", "agent_url", agentURL)
				return
			}
		}
	}()

	return stream, nil
}

// GetTask retrieves the status of a task using the external client
func (c *A2AClient) GetTask(ctx context.Context, request *adk.GetTaskRequest, agentURL string) (*adk.GetTaskSuccessResponse, error) {
	if !c.Initialized {
		return nil, ErrClientNotInitialized
	}

	if !c.isValidAgentURL(agentURL) {
		return nil, ErrAgentNotFound
	}

	agentClient, exists := c.AgentClients[agentURL]
	if !exists {
		return nil, ErrAgentNotFound
	}

	response, err := agentClient.GetTask(ctx, request.Params)
	if err != nil {
		return nil, err
	}

	var task adk.Task
	if err := json.Unmarshal(response.Result.(json.RawMessage), &task); err != nil {
		return nil, fmt.Errorf("failed to unmarshal task result: %w", err)
	}

	return &adk.GetTaskSuccessResponse{
		ID:      response.ID,
		JSONRPC: response.JSONRPC,
		Result:  task,
	}, nil
}

// CancelTask cancels a running task using the external client
func (c *A2AClient) CancelTask(ctx context.Context, request *adk.CancelTaskRequest, agentURL string) (*adk.CancelTaskSuccessResponse, error) {
	if !c.Initialized {
		return nil, ErrClientNotInitialized
	}

	if !c.isValidAgentURL(agentURL) {
		return nil, ErrAgentNotFound
	}

	agentClient, exists := c.AgentClients[agentURL]
	if !exists {
		return nil, ErrAgentNotFound
	}

	response, err := agentClient.CancelTask(ctx, request.Params)
	if err != nil {
		return nil, err
	}

	// Unmarshal the Result field from json.RawMessage to adk.Task
	var task adk.Task
	if err := json.Unmarshal(response.Result.(json.RawMessage), &task); err != nil {
		return nil, fmt.Errorf("failed to unmarshal task result: %w", err)
	}

	return &adk.CancelTaskSuccessResponse{
		ID:      response.ID,
		JSONRPC: response.JSONRPC,
		Result:  task,
	}, nil
}

// GetAgents returns the list of A2A agent URLs
func (c *A2AClient) GetAgents() []string {
	return c.AgentURLs
}

// GetAgentCapabilities returns the agent capabilities map
func (c *A2AClient) GetAgentCapabilities() map[string]adk.AgentCapabilities {
	return c.AgentCapabilities
}

// GetAgentSkills returns the skills available for the specified agent
func (c *A2AClient) GetAgentSkills(agentURL string) ([]adk.AgentSkill, error) {
	agentCard, exists := c.AgentCards[agentURL]
	if !exists {
		return nil, ErrAgentNotFound
	}

	return agentCard.Skills, nil
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
	return AgentStatusUnknown
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

// checkAgentHealth checks the health of a single agent using the external client
func (c *A2AClient) checkAgentHealth(ctx context.Context, agentURL string) {
	checkCtx, cancel := context.WithTimeout(ctx, c.Config.A2A.PollingTimeout)
	defer cancel()

	agentClient, exists := c.AgentClients[agentURL]
	if !exists {
		c.Logger.Debug("agent client not found for health check", "agentURL", agentURL, "component", "a2a_client")
		return
	}

	_, err := agentClient.GetHealth(checkCtx)
	if err != nil {
		_, err = agentClient.GetAgentCard(checkCtx)
	}

	newStatus := AgentStatusAvailable
	if err != nil {
		newStatus = AgentStatusUnavailable
		if !c.Config.A2A.DisableHealthcheckLogs {
			c.Logger.Debug("agent health check failed", "agentURL", agentURL, "error", err, "component", "a2a_client")
		}

		c.statusMutex.RLock()
		oldStatus := c.AgentStatuses[agentURL]
		c.statusMutex.RUnlock()

		if oldStatus == AgentStatusAvailable && c.Config.A2A.EnableReconnect {
			c.Logger.Info("agent became unavailable, scheduling reconnection", "agentURL", agentURL, "component", "a2a_client")
			go c.attemptAgentReconnection(ctx, agentURL)
		}
	} else if !c.Config.A2A.DisableHealthcheckLogs {
		c.Logger.Debug("agent health check passed", "agentURL", agentURL, "component", "a2a_client")
	}

	c.statusMutex.Lock()
	oldStatus := c.AgentStatuses[agentURL]
	c.AgentStatuses[agentURL] = newStatus
	c.statusMutex.Unlock()

	if oldStatus != newStatus {
		c.Logger.Info("agent status changed", "agentURL", agentURL, "oldStatus", string(oldStatus), "newStatus", string(newStatus), "component", "a2a_client")
	}
}

// startBackgroundReconnection starts a background goroutine to reconnect failed agents
func (c *A2AClient) startBackgroundReconnection(ctx context.Context, failedAgents []string) {
	c.Logger.Info("starting background reconnection for failed agents",
		"agents", failedAgents,
		"interval", c.Config.A2A.ReconnectInterval,
		"component", "a2a_client")

	ticker := time.NewTicker(c.Config.A2A.ReconnectInterval)
	defer ticker.Stop()

	reconnectingAgents := make(map[string]bool)
	for _, agent := range failedAgents {
		reconnectingAgents[agent] = true
	}

	for {
		select {
		case <-ctx.Done():
			c.Logger.Info("background reconnection stopped due to context cancellation", "component", "a2a_client")
			return
		case <-ticker.C:
			c.statusMutex.RLock()
			agentsToReconnect := make([]string, 0)
			for agentURL := range reconnectingAgents {
				if status, exists := c.AgentStatuses[agentURL]; exists && status == AgentStatusUnavailable {
					agentsToReconnect = append(agentsToReconnect, agentURL)
				} else if status == AgentStatusAvailable {
					delete(reconnectingAgents, agentURL)
					c.Logger.Info("agent successfully reconnected, removing from background reconnection",
						"agentURL", agentURL, "component", "a2a_client")
				}
			}
			c.statusMutex.RUnlock()

			if len(reconnectingAgents) == 0 {
				c.Logger.Info("all agents successfully reconnected, stopping background reconnection", "component", "a2a_client")
				return
			}

			for _, agentURL := range agentsToReconnect {
				go c.attemptAgentReconnection(ctx, agentURL)
			}
		}
	}
}

// attemptAgentReconnection attempts to reconnect a single failed agent
func (c *A2AClient) attemptAgentReconnection(ctx context.Context, agentURL string) {
	c.Logger.Info("attempting agent reconnection", "agentURL", agentURL, "component", "a2a_client")

	reconnectCtx, cancel := context.WithTimeout(ctx, c.Config.A2A.ClientTimeout)
	defer cancel()

	if err := c.initializeAgent(reconnectCtx, agentURL); err != nil {
		c.Logger.Info("agent reconnection failed", "agentURL", agentURL, "error", err, "component", "a2a_client")
		return
	}

	c.Logger.Info("agent successfully reconnected", "agentURL", agentURL, "component", "a2a_client")
}
