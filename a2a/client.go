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
	// AgentStatusUnknown indicates agent status is not available
	AgentStatusUnknown AgentStatus = "unknown"
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

// A2AClient provides methods to interact with A2A agents using the external client library
type A2AClient struct {
	AgentURLs         []string
	Logger            logger.Logger
	Config            config.Config
	AgentClients      map[string]client.A2AClient
	AgentCards        map[string]*AgentCard
	AgentCapabilities map[string]AgentCapabilities
	Initialized       bool

	// Status tracking
	AgentStatuses map[string]AgentStatus
	statusMutex   sync.RWMutex
	pollingCancel context.CancelFunc
	pollingDone   chan struct{}
}

// NewA2AClient creates a new A2A client instance using the external client library
func NewA2AClient(cfg config.Config, log logger.Logger) *A2AClient {
	agentURLs := parseAgentURLs(cfg.A2A.Agents)

	return &A2AClient{
		AgentURLs:         agentURLs,
		Logger:            log,
		Config:            cfg,
		AgentClients:      make(map[string]client.A2AClient),
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

// InitializeAll discovers and connects to A2A agents using the external client library
func (c *A2AClient) InitializeAll(ctx context.Context) error {
	if len(c.AgentURLs) == 0 {
		return ErrNoAgentURLs
	}

	var lastError error
	successfulInitializations := 0

	c.statusMutex.Lock()
	for _, agentURL := range c.AgentURLs {
		c.AgentStatuses[agentURL] = AgentStatusUnknown
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

// initializeAgent initializes a single agent using the external client library
func (c *A2AClient) initializeAgent(ctx context.Context, agentURL string) error {
	config := &client.Config{
		BaseURL: agentURL,
		Timeout: c.Config.A2A.ClientTimeout,
	}

	agentClient := client.NewClientWithConfig(config)
	c.AgentClients[agentURL] = agentClient

	externalAgentCard, err := agentClient.GetAgentCard(ctx)
	if err != nil {
		return fmt.Errorf("failed to get agent card: %w", err)
	}

	agentCard := c.convertExternalAgentCard(externalAgentCard)
	c.AgentCards[agentURL] = agentCard
	c.AgentCapabilities[agentURL] = agentCard.Capabilities

	return nil
}

// IsInitialized returns whether the client has been successfully initialized
func (c *A2AClient) IsInitialized() bool {
	return c.Initialized
}

// convertExternalAgentCard converts an external agent card to the internal format
func (c *A2AClient) convertExternalAgentCard(external *adk.AgentCard) *AgentCard {
	skills := make([]AgentSkill, len(external.Skills))
	for i, skill := range external.Skills {
		skills[i] = AgentSkill{
			ID:          skill.ID,
			Name:        skill.Name,
			Description: skill.Description,
			Tags:        skill.Tags,
			Examples:    skill.Examples,
			InputModes:  skill.InputModes,
			OutputModes: skill.OutputModes,
		}
	}

	capabilities := AgentCapabilities{
		Streaming:              external.Capabilities.Streaming,
		Extensions:             make([]AgentExtension, len(external.Capabilities.Extensions)),
		PushNotifications:      external.Capabilities.PushNotifications,
		StateTransitionHistory: external.Capabilities.StateTransitionHistory,
	}

	for i, ext := range external.Capabilities.Extensions {
		capabilities.Extensions[i] = AgentExtension{
			URI:         ext.URI,
			Description: ext.Description,
			Required:    ext.Required,
			Params:      ext.Params,
		}
	}

	var provider *AgentProvider
	if external.Provider != nil {
		provider = &AgentProvider{
			Organization: external.Provider.Organization,
			URL:          external.Provider.URL,
		}
	}

	return &AgentCard{
		Name:                              external.Name,
		Version:                           external.Version,
		Description:                       external.Description,
		URL:                               external.URL,
		Skills:                            skills,
		Capabilities:                      capabilities,
		Provider:                          provider,
		DocumentationURL:                  external.DocumentationURL,
		IconURL:                           external.IconURL,
		DefaultInputModes:                 external.DefaultInputModes,
		DefaultOutputModes:                external.DefaultOutputModes,
		SecuritySchemes:                   make(map[string]SecurityScheme),
		Security:                          external.Security,
		SupportsAuthenticatedExtendedCard: external.SupportsAuthenticatedExtendedCard,
	}
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

	agentClient, exists := c.AgentClients[agentURL]
	if !exists {
		return nil, ErrAgentNotFound
	}

	externalAgentCard, err := agentClient.GetAgentCard(ctx)
	if err != nil {
		return nil, err
	}

	agentCard := c.convertExternalAgentCard(externalAgentCard)
	c.AgentCards[agentURL] = agentCard
	c.AgentCapabilities[agentURL] = agentCard.Capabilities

	return agentCard, nil
}

// RefreshAgentCard forces a refresh of an agent card from the remote source using the external client
func (c *A2AClient) RefreshAgentCard(ctx context.Context, agentURL string) (*AgentCard, error) {
	if !c.isValidAgentURL(agentURL) {
		return nil, ErrAgentNotFound
	}

	agentClient, exists := c.AgentClients[agentURL]
	if !exists {
		return nil, ErrAgentNotFound
	}

	externalAgentCard, err := agentClient.GetAgentCard(ctx)
	if err != nil {
		return nil, err
	}

	agentCard := c.convertExternalAgentCard(externalAgentCard)
	c.AgentCards[agentURL] = agentCard
	c.AgentCapabilities[agentURL] = agentCard.Capabilities

	return agentCard, nil
}

// SendMessage sends a message to the specified agent using the external client library
func (c *A2AClient) SendMessage(ctx context.Context, request *SendMessageRequest, agentURL string) (*SendMessageSuccessResponse, error) {
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

	// Convert internal request to external format
	externalParams := c.convertToExternalMessageSendParams(request.Params)

	// Use the external client to send the task
	response, err := agentClient.SendTask(ctx, externalParams)
	if err != nil {
		return nil, err
	}

	// Convert response back to internal format
	return &SendMessageSuccessResponse{
		ID:      response.ID,
		JSONRPC: response.JSONRPC,
		Result:  response.Result,
	}, nil
}

// convertToExternalMessageSendParams converts internal MessageSendParams to external format
func (c *A2AClient) convertToExternalMessageSendParams(params MessageSendParams) adk.MessageSendParams {
	// Convert message
	externalMessage := adk.Message{
		MessageID:        params.Message.MessageID,
		Kind:             params.Message.Kind,
		Role:             params.Message.Role,
		Parts:            c.convertParts(params.Message.Parts),
		TaskID:           params.Message.TaskID,
		ContextID:        params.Message.ContextID,
		ReferenceTaskIds: params.Message.ReferenceTaskIds,
		Extensions:       params.Message.Extensions,
		Metadata:         params.Message.Metadata,
	}

	// Convert configuration if present
	var externalConfig *adk.MessageSendConfiguration
	if params.Configuration != nil {
		externalConfig = &adk.MessageSendConfiguration{
			AcceptedOutputModes: params.Configuration.AcceptedOutputModes,
			Blocking:            params.Configuration.Blocking,
		}
	}

	return adk.MessageSendParams{
		Message:       externalMessage,
		Configuration: externalConfig,
		Metadata:      params.Metadata,
	}
}

// convertParts converts internal Parts to external format
func (c *A2AClient) convertParts(parts []Part) []adk.Part {
	// This is a simplified conversion - in reality, you'd need to handle different Part types
	// For now, we'll return an empty slice and handle the conversion later
	return []adk.Part{}
}

// SendStreamingMessage sends a streaming message to the specified agent using the external client
func (c *A2AClient) SendStreamingMessage(ctx context.Context, request *SendStreamingMessageRequest, agentURL string) (<-chan []byte, error) {
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

	// Convert internal request to external format
	externalParams := c.convertToExternalMessageSendParams(request.Params)

	// Create a channel to receive streaming events from the external client
	eventChan := make(chan interface{}, 100)

	// Create a channel to send byte data to the caller
	stream := make(chan []byte, 100)

	// Start the streaming task using the external client
	go func() {
		defer close(eventChan)
		err := agentClient.SendTaskStreaming(ctx, externalParams, eventChan)
		if err != nil {
			c.Logger.Error("streaming task failed", err, "agent_url", agentURL)
		}
	}()

	// Convert external streaming events to byte stream
	go func() {
		defer close(stream)
		for {
			select {
			case event, ok := <-eventChan:
				if !ok {
					return
				}
				// Convert event to bytes (this is a simplified conversion)
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
func (c *A2AClient) GetTask(ctx context.Context, request *GetTaskRequest, agentURL string) (*GetTaskSuccessResponse, error) {
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

	// Convert internal request to external format
	externalParams := adk.TaskQueryParams{
		ID: request.Params.ID,
	}

	response, err := agentClient.GetTask(ctx, externalParams)
	if err != nil {
		return nil, err
	}

	return &GetTaskSuccessResponse{
		ID:      response.ID,
		JSONRPC: response.JSONRPC,
		Result:  response.Result.(Task),
	}, nil
}

// CancelTask cancels a running task using the external client
func (c *A2AClient) CancelTask(ctx context.Context, request *CancelTaskRequest, agentURL string) (*CancelTaskSuccessResponse, error) {
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

	// Convert internal request to external format
	externalParams := adk.TaskIdParams{
		ID: request.Params.ID,
	}

	response, err := agentClient.CancelTask(ctx, externalParams)
	if err != nil {
		return nil, err
	}

	return &CancelTaskSuccessResponse{
		ID:      response.ID,
		JSONRPC: response.JSONRPC,
		Result:  response.Result.(Task),
	}, nil
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
		c.Logger.Debug("agent health check failed", "agentURL", agentURL, "error", err, "component", "a2a_client")
	} else {
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
