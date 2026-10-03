package a2a

import (
	"context"
	"maps"
	"net/http"
	"slices"
	"sync"
	"time"

	client "github.com/inference-gateway/adk/client"
	types "github.com/inference-gateway/adk/types"

	config "github.com/inference-gateway/inference-gateway/config"
	endpoints "github.com/inference-gateway/inference-gateway/internal/platform/endpoints"
	logger "github.com/inference-gateway/inference-gateway/internal/platform/logger"
)

const (
	agentKind = "a2a agent"
	userAgent = "inference-gateway"
)

// ParseAgents parses the A2A_AGENTS value with the MCP_SERVERS grammar: a
// comma-separated list of "alias=url" or bare "url" entries.
func ParseAgents(raw string) ([]endpoints.Spec, error) {
	return endpoints.Parse(raw, agentKind)
}

// DialAgents builds one ADK client per configured agent. The HTTP client has
// no overall timeout because the same client relays SSE streams; unary calls
// get A2A_CLIENT_TIMEOUT as a context deadline instead. Retries are off: a
// relay must not re-send a task the client sent once.
func DialAgents(specs []endpoints.Spec, cfg config.A2AConfig) map[string]client.A2AClient {
	agents := make(map[string]client.A2AClient, len(specs))
	for _, spec := range specs {
		adkCfg := client.DefaultConfig(spec.URL)
		adkCfg.Timeout = cfg.ClientTimeout
		adkCfg.UserAgent = userAgent
		adkCfg.MaxRetries = 0
		adkCfg.HTTPClient = &http.Client{}
		agents[spec.Alias] = client.NewClientWithConfig(adkCfg)
	}
	return agents
}

// AgentStatus is one registry entry as reported by GET /a2a/agents.
type AgentStatus struct {
	Alias     string           `json:"alias"`
	URL       string           `json:"url"`
	Card      *types.AgentCard `json:"card,omitempty"`
	Reachable bool             `json:"reachable"`
	LastSeen  *time.Time       `json:"lastSeen,omitempty"`
}

type agent struct {
	client    client.A2AClient
	card      *types.AgentCard
	reachable bool
	lastSeen  time.Time
}

// Registry is the in-memory map of configured agents and their last fetched
// cards. It is the only state the gateway keeps about agents: no tasks, no
// messages, no events.
type Registry struct {
	cfg    config.A2AConfig
	logger logger.Logger

	mu      sync.RWMutex
	agents  map[string]*agent
	aliases []string
}

// NewRegistry builds a registry over the given clients, keyed by alias, without
// contacting any agent; Start or Refresh fetch the cards.
func NewRegistry(cfg config.A2AConfig, log logger.Logger, clients map[string]client.A2AClient) *Registry {
	registry := &Registry{
		cfg:     cfg,
		logger:  log,
		agents:  make(map[string]*agent, len(clients)),
		aliases: slices.Sorted(maps.Keys(clients)),
	}
	for alias, adk := range clients {
		registry.agents[alias] = &agent{client: adk}
	}
	return registry
}

// Start fetches every card once and, when A2A_CARD_REFRESH_INTERVAL is
// positive, keeps refreshing them until ctx is cancelled. An unreachable agent
// is logged and retried on the next refresh; it never fails startup.
func (r *Registry) Start(ctx context.Context) {
	r.Refresh(ctx)
	if r.cfg.CardRefreshInterval <= 0 {
		return
	}
	go r.refreshLoop(ctx)
}

func (r *Registry) refreshLoop(ctx context.Context) {
	ticker := time.NewTicker(r.cfg.CardRefreshInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			r.Refresh(ctx)
		}
	}
}

// Refresh fetches every agent card concurrently and records the outcome.
func (r *Registry) Refresh(ctx context.Context) {
	var wg sync.WaitGroup
	for _, alias := range r.aliases {
		wg.Go(func() { r.refreshAgent(ctx, alias) })
	}
	wg.Wait()
}

func (r *Registry) refreshAgent(ctx context.Context, alias string) {
	entry := r.agents[alias]
	callCtx, cancel := r.callContext(ctx)
	defer cancel()

	card, err := entry.client.GetAgentCard(callCtx)

	r.mu.Lock()
	defer r.mu.Unlock()
	if err != nil {
		entry.reachable = false
		r.logger.Warn("a2a agent card fetch failed", "agent", alias, "url", entry.client.GetBaseURL(), "error", err.Error())
		return
	}
	entry.card = card
	entry.reachable = true
	entry.lastSeen = time.Now()
	r.logger.Info("a2a agent card fetched", "agent", alias, "name", card.Name, "skills", len(card.Skills))
}

// callContext bounds one non-streaming upstream call by A2A_CLIENT_TIMEOUT.
func (r *Registry) callContext(ctx context.Context) (context.Context, context.CancelFunc) {
	if r.cfg.ClientTimeout <= 0 {
		return context.WithCancel(ctx)
	}
	return context.WithTimeout(ctx, r.cfg.ClientTimeout)
}

// Aliases lists the configured aliases in sorted order.
func (r *Registry) Aliases() []string {
	return slices.Clone(r.aliases)
}

// Agents reports every registry entry for GET /a2a/agents.
func (r *Registry) Agents() []AgentStatus {
	r.mu.RLock()
	defer r.mu.RUnlock()

	statuses := make([]AgentStatus, 0, len(r.aliases))
	for _, alias := range r.aliases {
		entry := r.agents[alias]
		status := AgentStatus{Alias: alias, URL: entry.client.GetBaseURL(), Card: entry.card, Reachable: entry.reachable}
		if !entry.lastSeen.IsZero() {
			lastSeen := entry.lastSeen
			status.LastSeen = &lastSeen
		}
		statuses = append(statuses, status)
	}
	return statuses
}

func (r *Registry) lookup(alias string) (*agent, bool) {
	entry, ok := r.agents[alias]
	return entry, ok
}

// Card is the gateway's own agent card: the union of every registered agent's
// skills with ids prefixed <alias>_, capabilities that hold only when every
// registered agent reports them, and one interface per agent naming the alias
// as its tenant, after a tenant-less default interface.
func (r *Registry) Card(url, version string) types.AgentCard {
	r.mu.RLock()
	defer r.mu.RUnlock()

	streaming, pushNotifications := len(r.aliases) > 0, len(r.aliases) > 0
	inputModes, outputModes := map[string]struct{}{}, map[string]struct{}{}
	skills := make([]types.AgentSkill, 0)
	interfaces := []types.AgentInterface{gatewayInterface(url, nil)}

	for _, alias := range r.aliases {
		interfaces = append(interfaces, gatewayInterface(url, &alias))
		card := r.agents[alias].card
		if card == nil {
			streaming, pushNotifications = false, false
			continue
		}
		streaming = streaming && boolValue(card.Capabilities.Streaming)
		pushNotifications = pushNotifications && boolValue(card.Capabilities.PushNotifications)
		for _, mode := range card.DefaultInputModes {
			inputModes[mode] = struct{}{}
		}
		for _, mode := range card.DefaultOutputModes {
			outputModes[mode] = struct{}{}
		}
		for _, skill := range card.Skills {
			skill.ID = SkillID(alias, skill.ID)
			skills = append(skills, skill)
		}
	}

	return types.AgentCard{
		Name:                config.APPLICATION_NAME,
		Description:         "Inference Gateway A2A server delegating to " + joinAliases(r.aliases),
		Version:             version,
		Capabilities:        types.AgentCapabilities{Streaming: &streaming, PushNotifications: &pushNotifications},
		DefaultInputModes:   slices.Sorted(maps.Keys(inputModes)),
		DefaultOutputModes:  slices.Sorted(maps.Keys(outputModes)),
		Skills:              skills,
		SupportedInterfaces: interfaces,
	}
}

func gatewayInterface(url string, tenant *string) types.AgentInterface {
	return types.AgentInterface{
		ProtocolBinding: protocolBindingJSONRPC,
		ProtocolVersion: types.A2AProtocolVersion,
		Tenant:          tenant,
		URL:             url,
	}
}

// SkillID is the id a registered agent's skill carries on the gateway card.
func SkillID(alias, skillID string) string {
	return alias + "_" + skillID
}

func boolValue(b *bool) bool {
	return b != nil && *b
}
