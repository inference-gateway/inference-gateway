package tests

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/inference-gateway/inference-gateway/a2a"
	"github.com/inference-gateway/inference-gateway/config"
	"github.com/inference-gateway/inference-gateway/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestA2AClient_AgentCardCaching(t *testing.T) {
	tests := []struct {
		name        string
		description string
	}{
		{
			name:        "agent_card_cached_after_initialization",
			description: "Agent card should be cached after initialization and not fetch from remote on subsequent calls",
		},
		{
			name:        "cache_miss_fetches_from_remote",
			description: "Agent card should be fetched from remote when not in cache",
		},
		{
			name:        "refresh_updates_cache",
			description: "RefreshAgentCard should update the cache with fresh data",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockAgentCard := &a2a.AgentCard{
				Name:        "Test Agent",
				Description: "A test agent for unit testing",
				Version:     "1.0.0",
				Capabilities: a2a.AgentCapabilities{
					Pushnotifications:      false,
					Statetransitionhistory: true,
					Streaming:              true,
				},
				Skills: []a2a.AgentSkill{
					{
						Name:        "test_skill",
						Description: "A test skill",
						ID:          "test_skill_id",
						Tags:        []string{"test"},
						Inputmodes:  []string{"text"},
						Outputmodes: []string{"text"},
					},
				},
			}

			requestCount := 0

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requestCount++

				expectedPath := "/.well-known/agent.json"
				assert.Equal(t, expectedPath, r.URL.Path, "unexpected request path")

				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)

				agentCardBytes, err := json.Marshal(mockAgentCard)
				require.NoError(t, err)
				_, err = w.Write(agentCardBytes)
				require.NoError(t, err)
			}))
			defer server.Close()

			cfg := config.Config{
				A2A: &config.A2AConfig{
					Agents: server.URL,
				},
				Client: &config.ClientConfig{
					MaxIdleConns:          20,
					MaxIdleConnsPerHost:   20,
					IdleConnTimeout:       30 * time.Second,
					TlsMinVersion:         "TLS12",
					DisableCompression:    true,
					ResponseHeaderTimeout: 10 * time.Second,
					ExpectContinueTimeout: 1 * time.Second,
				},
			}

			log, err := logger.NewLogger("test")
			require.NoError(t, err)

			client := a2a.NewA2AClient(cfg, log)
			require.NotNil(t, client)

			ctx := context.Background()

			switch tt.name {
			case "agent_card_cached_after_initialization":
				err := client.InitializeAll(ctx)
				require.NoError(t, err)
				assert.Equal(t, 1, requestCount, "initialization should make exactly one HTTP request")

				assert.Len(t, client.AgentCards, 1, "agent card should be cached")
				cachedCard, exists := client.AgentCards[server.URL]
				require.True(t, exists, "agent card should exist in cache")
				assert.Equal(t, mockAgentCard.Name, cachedCard.Name)

				for i := 0; i < 3; i++ {
					card, err := client.GetAgentCard(ctx, server.URL)
					require.NoError(t, err)
					assert.Equal(t, mockAgentCard.Name, card.Name)
				}

				assert.Equal(t, 1, requestCount, "subsequent GetAgentCard calls should use cache")

			case "cache_miss_fetches_from_remote":
				card, err := client.GetAgentCard(ctx, server.URL)
				require.NoError(t, err)
				assert.Equal(t, mockAgentCard.Name, card.Name)
				assert.Equal(t, 1, requestCount, "should make one HTTP request for cache miss")

				card2, err := client.GetAgentCard(ctx, server.URL)
				require.NoError(t, err)
				assert.Equal(t, mockAgentCard.Name, card2.Name)
				assert.Equal(t, 1, requestCount, "second call should use cache")

			case "refresh_updates_cache":
				err := client.InitializeAll(ctx)
				require.NoError(t, err)
				assert.Equal(t, 1, requestCount, "initialization should make one request")

				mockAgentCard.Name = "Updated Test Agent"
				mockAgentCard.Description = "An updated test agent"

				refreshedCard, err := client.RefreshAgentCard(ctx, server.URL)
				require.NoError(t, err)
				assert.Equal(t, "Updated Test Agent", refreshedCard.Name)
				assert.Equal(t, 2, requestCount, "refresh should make additional HTTP request")

				cachedCard, exists := client.AgentCards[server.URL]
				require.True(t, exists)
				assert.Equal(t, "Updated Test Agent", cachedCard.Name)

				card, err := client.GetAgentCard(ctx, server.URL)
				require.NoError(t, err)
				assert.Equal(t, "Updated Test Agent", card.Name)
				assert.Equal(t, 2, requestCount, "should still be only 2 requests")
			}
		})
	}
}

func TestA2AClient_InvalidAgentURL(t *testing.T) {
	cfg := config.Config{
		A2A: &config.A2AConfig{
			Agents: "https://agent1.example.com,https://agent2.example.com",
		},
		Client: &config.ClientConfig{
			MaxIdleConns:          20,
			MaxIdleConnsPerHost:   20,
			IdleConnTimeout:       30 * time.Second,
			TlsMinVersion:         "TLS12",
			DisableCompression:    true,
			ResponseHeaderTimeout: 10 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
		},
	}

	log, err := logger.NewLogger("test")
	require.NoError(t, err)

	client := a2a.NewA2AClient(cfg, log)
	ctx := context.Background()

	_, err = client.GetAgentCard(ctx, "https://invalid-agent.example.com")
	assert.ErrorIs(t, err, a2a.ErrAgentNotFound)

	_, err = client.RefreshAgentCard(ctx, "https://invalid-agent.example.com")
	assert.ErrorIs(t, err, a2a.ErrAgentNotFound)
}

func TestA2AClient_HTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, "Internal Server Error")
	}))
	defer server.Close()

	cfg := config.Config{
		A2A: &config.A2AConfig{
			Agents: server.URL,
		},
		Client: &config.ClientConfig{
			MaxIdleConns:          20,
			MaxIdleConnsPerHost:   20,
			IdleConnTimeout:       30 * time.Second,
			TlsMinVersion:         "TLS12",
			DisableCompression:    true,
			ResponseHeaderTimeout: 10 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
		},
	}

	log, err := logger.NewLogger("test")
	require.NoError(t, err)

	client := a2a.NewA2AClient(cfg, log)
	ctx := context.Background()

	_, err = client.GetAgentCard(ctx, server.URL)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "agent card request failed with status 500")

	_, err = client.RefreshAgentCard(ctx, server.URL)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "agent card request failed with status 500")
}

func TestA2AClient_NetworkTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"name": "Slow Agent"}`)
	}))
	defer server.Close()

	cfg := config.Config{
		A2A: &config.A2AConfig{
			Agents: server.URL,
		},
		Client: &config.ClientConfig{
			MaxIdleConns:          20,
			MaxIdleConnsPerHost:   20,
			IdleConnTimeout:       30 * time.Second,
			TlsMinVersion:         "TLS12",
			DisableCompression:    true,
			ResponseHeaderTimeout: 10 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
		},
	}

	log, err := logger.NewLogger("test")
	require.NoError(t, err)

	client := a2a.NewA2AClient(cfg, log)

	client.HTTPClient.Timeout = 100 * time.Millisecond

	ctx := context.Background()

	_, err = client.GetAgentCard(ctx, server.URL)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to make request")
}

func TestA2AClient_MalformedJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"name": "Test Agent", "invalid": json}`)
	}))
	defer server.Close()

	cfg := config.Config{
		A2A: &config.A2AConfig{
			Agents: server.URL,
		},
		Client: &config.ClientConfig{
			MaxIdleConns:          20,
			MaxIdleConnsPerHost:   20,
			IdleConnTimeout:       30 * time.Second,
			TlsMinVersion:         "TLS12",
			DisableCompression:    true,
			ResponseHeaderTimeout: 10 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
		},
	}

	log, err := logger.NewLogger("test")
	require.NoError(t, err)

	client := a2a.NewA2AClient(cfg, log)
	ctx := context.Background()

	_, err = client.GetAgentCard(ctx, server.URL)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to unmarshal agent card")
}
