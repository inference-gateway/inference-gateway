package routing

import (
	"fmt"
	"maps"
	"os"
	"slices"
	"sync/atomic"

	registry "github.com/inference-gateway/inference-gateway/providers/registry"
	types "github.com/inference-gateway/inference-gateway/providers/types"
	yaml "gopkg.in/yaml.v3"
)

// StrategyRoundRobin is the only selection strategy supported in Phase 1. An
// empty strategy in the config defaults to it.
const StrategyRoundRobin = "round_robin"

// Deployment is one upstream backing a logical model alias: an already-configured
// provider plus the upstream model name to send to it.
type Deployment struct {
	Provider string `yaml:"provider"`
	Model    string `yaml:"model"`
}

// PoolConfig is the on-disk shape of a single logical alias: the selection
// strategy and an ordered list of deployments.
type PoolConfig struct {
	Strategy    string       `yaml:"strategy"`
	Deployments []Deployment `yaml:"deployments"`
}

// PoolsConfig is the on-disk routing file: logical alias -> pool.
type PoolsConfig struct {
	Models map[string]PoolConfig `yaml:"models"`
}

// pool is the runtime form of a PoolConfig with a per-replica round-robin cursor.
type pool struct {
	deployments []Deployment
	cursor      atomic.Uint64
}

// Selector resolves a logical model alias to an upstream deployment. Round-robin
// state lives in each Selector (i.e. per replica), so under multiple gateway
// replicas the rotation is best-effort per replica, not globally coordinated.
type Selector struct {
	pools map[string]*pool
}

// LoadPoolsConfig reads and parses the routing YAML file at path.
func LoadPoolsConfig(path string) (*PoolsConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read routing config: %w", err)
	}
	var cfg PoolsConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse routing config: %w", err)
	}
	return &cfg, nil
}

// NewSelector builds a Selector from parsed pools, validating that every alias
// has a supported strategy, at least two deployments to rotate over, and
// references a known provider. It returns an error rather than start routing to
// a broken pool.
func NewSelector(cfg *PoolsConfig) (*Selector, error) {
	if cfg == nil || len(cfg.Models) == 0 {
		return nil, fmt.Errorf("routing enabled but no models configured")
	}
	pools := make(map[string]*pool, len(cfg.Models))
	for alias, pc := range cfg.Models {
		if pc.Strategy != "" && pc.Strategy != StrategyRoundRobin {
			return nil, fmt.Errorf("model %q: unsupported strategy %q (only %q is supported)", alias, pc.Strategy, StrategyRoundRobin)
		}
		if len(pc.Deployments) < 2 {
			return nil, fmt.Errorf("model %q: round-robin requires at least 2 deployments, got %d", alias, len(pc.Deployments))
		}
		for i, d := range pc.Deployments {
			if d.Provider == "" || d.Model == "" {
				return nil, fmt.Errorf("model %q deployment %d: provider and model are required", alias, i)
			}
			if _, ok := registry.Registry[types.Provider(d.Provider)]; !ok {
				return nil, fmt.Errorf("model %q deployment %d: unknown provider %q", alias, i, d.Provider)
			}
		}
		pools[alias] = &pool{deployments: pc.Deployments}
	}
	return &Selector{pools: pools}, nil
}

// Select returns the next deployment for a logical alias in round-robin order.
// ok is false when alias is not a routed model, so callers fall back to the
// existing direct provider/model routing unchanged. Round-robin state is per
// Selector (per replica), not globally coordinated; a shared-store
// implementation is deferred to a later phase (#397).
func (s *Selector) Select(alias string) (deployment Deployment, ok bool) {
	p, found := s.pools[alias]
	if !found {
		return Deployment{}, false
	}
	i := p.cursor.Add(1) - 1
	return p.deployments[i%uint64(len(p.deployments))], true
}

// Aliases returns the configured logical model names, for startup logging.
func (s *Selector) Aliases() []string {
	return slices.Sorted(maps.Keys(s.pools))
}
