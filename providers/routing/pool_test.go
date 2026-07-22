package routing

import (
	"sync"
	"testing"

	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

func poolFor(t *testing.T, deployments ...Deployment) *Selector {
	t.Helper()
	sel, err := NewSelector(&PoolsConfig{
		Models: map[string]PoolConfig{
			"fast-chat": {Strategy: StrategyRoundRobin, Deployments: deployments},
		},
	})
	require.NoError(t, err)
	return sel
}

func TestSelectRoundRobinRotation(t *testing.T) {
	d0 := Deployment{Provider: "groq", Model: "llama-3.3-70b-versatile"}
	d1 := Deployment{Provider: "openai", Model: "gpt-4o-mini"}
	d2 := Deployment{Provider: "ollama", Model: "phi3"}
	sel := poolFor(t, d0, d1, d2)

	want := []Deployment{d0, d1, d2, d0, d1, d2, d0}
	for i, expected := range want {
		got, ok := sel.Select("fast-chat")
		assert.True(t, ok, "call %d should resolve", i)
		assert.Equal(t, expected, got, "call %d", i)
	}
}

func TestSelectUnknownAliasFallsThrough(t *testing.T) {
	sel := poolFor(t,
		Deployment{Provider: "groq", Model: "llama-3.3-70b-versatile"},
		Deployment{Provider: "openai", Model: "gpt-4o-mini"},
	)
	got, ok := sel.Select("not-a-pool")
	assert.False(t, ok)
	assert.Equal(t, Deployment{}, got)
}

func TestSelectEmptyStrategyDefaultsToRoundRobin(t *testing.T) {
	sel, err := NewSelector(&PoolsConfig{
		Models: map[string]PoolConfig{
			"cheap": {Deployments: []Deployment{{Provider: "groq", Model: "x"}, {Provider: "openai", Model: "y"}}},
		},
	})
	require.NoError(t, err)
	got, ok := sel.Select("cheap")
	assert.True(t, ok)
	assert.Equal(t, "groq", got.Provider)
}

func TestNewSelectorValidation(t *testing.T) {
	tests := []struct {
		name string
		cfg  *PoolsConfig
	}{
		{"nil config", nil},
		{"no models", &PoolsConfig{Models: map[string]PoolConfig{}}},
		{
			"unsupported strategy",
			&PoolsConfig{Models: map[string]PoolConfig{"a": {Strategy: "weighted", Deployments: []Deployment{{Provider: "groq", Model: "x"}}}}},
		},
		{
			"no deployments",
			&PoolsConfig{Models: map[string]PoolConfig{"a": {Strategy: StrategyRoundRobin}}},
		},
		{
			"single deployment",
			&PoolsConfig{Models: map[string]PoolConfig{"a": {Deployments: []Deployment{{Provider: "groq", Model: "x"}}}}},
		},
		{
			"missing model",
			&PoolsConfig{Models: map[string]PoolConfig{"a": {Deployments: []Deployment{{Provider: "groq", Model: "x"}, {Provider: "groq"}}}}},
		},
		{
			"unknown provider",
			&PoolsConfig{Models: map[string]PoolConfig{"a": {Deployments: []Deployment{{Provider: "nope", Model: "x"}, {Provider: "groq", Model: "y"}}}}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewSelector(tt.cfg)
			assert.Error(t, err)
		})
	}
}

// TestSelectConcurrent guards the atomic cursor against data races (run with -race)
// and confirms even distribution across the pool from a single replica.
func TestSelectConcurrent(t *testing.T) {
	sel := poolFor(t,
		Deployment{Provider: "groq", Model: "a"},
		Deployment{Provider: "openai", Model: "b"},
	)

	const n = 1000
	var wg sync.WaitGroup
	counts := make([]int64, 2)
	var mu sync.Mutex
	for range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			dep, ok := sel.Select("fast-chat")
			require.True(t, ok)
			mu.Lock()
			if dep.Model == "a" {
				counts[0]++
			} else {
				counts[1]++
			}
			mu.Unlock()
		}()
	}
	wg.Wait()
	assert.Equal(t, int64(n/2), counts[0])
	assert.Equal(t, int64(n/2), counts[1])
}
