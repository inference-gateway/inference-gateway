package config_test

import (
	"testing"
	"time"

	"github.com/inference-gateway/inference-gateway/config"
	"github.com/inference-gateway/inference-gateway/providers/client"
	"github.com/inference-gateway/inference-gateway/providers/constants"
	"github.com/inference-gateway/inference-gateway/providers/registry"
	"github.com/inference-gateway/inference-gateway/providers/types"
	"github.com/sethvargo/go-envconfig"
	"github.com/stretchr/testify/assert"
)

func defaultProviders(overrides map[types.Provider]func(*registry.ProviderConfig)) map[types.Provider]*registry.ProviderConfig {
	providers := make(map[types.Provider]*registry.ProviderConfig, len(registry.Registry))
	for id, defaults := range registry.Registry {
		cp := *defaults
		if override, ok := overrides[id]; ok {
			override(&cp)
		}
		providers[id] = &cp
	}
	return providers
}

func defaultConfig(mutate func(*config.Config)) config.Config {
	cfg := config.Config{
		Environment:               "production",
		AllowedModels:             "",
		DebugContentTruncateWords: 10,
		DebugMaxMessages:          100,
		Telemetry: &config.TelemetryConfig{
			Enable:      false,
			MetricsPort: "9464",
		},
		MCP: &config.MCPConfig{
			Enable:                 false,
			Expose:                 false,
			Servers:                "",
			ClientTimeout:          5 * time.Second,
			DialTimeout:            3 * time.Second,
			TlsHandshakeTimeout:    3 * time.Second,
			ResponseHeaderTimeout:  3 * time.Second,
			ExpectContinueTimeout:  1 * time.Second,
			RequestTimeout:         5 * time.Second,
			MaxRetries:             3,
			RetryInterval:          5 * time.Second,
			InitialBackoff:         1 * time.Second,
			EnableReconnect:        true,
			ReconnectInterval:      30 * time.Second,
			PollingEnable:          true,
			PollingInterval:        30 * time.Second,
			PollingTimeout:         5 * time.Second,
			DisableHealthcheckLogs: true,
		},
		Auth: &config.AuthConfig{
			Enable:           false,
			OidcIssuer:       "http://keycloak:8080/realms/inference-gateway-realm",
			OidcClientId:     "inference-gateway-client",
			OidcClientSecret: "",
		},
		Server: &config.ServerConfig{
			Host:         "0.0.0.0",
			Port:         "8080",
			ReadTimeout:  30 * time.Second,
			WriteTimeout: 30 * time.Second,
			IdleTimeout:  120 * time.Second,
		},
		Client: &client.ClientConfig{
			ClientTimeout:               30 * time.Second,
			ClientMaxIdleConns:          20,
			ClientMaxIdleConnsPerHost:   20,
			ClientIdleConnTimeout:       30 * time.Second,
			ClientTlsMinVersion:         "TLS12",
			ClientDisableCompression:    true,
			ClientResponseHeaderTimeout: 10 * time.Second,
			ClientExpectContinueTimeout: 1 * time.Second,
		},
		Providers: defaultProviders(nil),
	}
	if mutate != nil {
		mutate(&cfg)
	}
	return cfg
}

func TestLoad(t *testing.T) {
	tests := []struct {
		name          string
		env           map[string]string
		expectedCfg   config.Config
		expectedError string
	}{
		{
			name:        "Success_Defaults",
			env:         map[string]string{},
			expectedCfg: defaultConfig(nil),
		},
		{
			name: "Success_AllEnvVariablesSet",
			env: map[string]string{
				"TELEMETRY_ENABLE":              "true",
				"TELEMETRY_METRICS_PUSH_ENABLE": "true",
				"ENVIRONMENT":                   "development",
				"SERVER_HOST":                   "localhost",
				"SERVER_PORT":                   "9090",
				"SERVER_READ_TIMEOUT":           "60s",
				"SERVER_WRITE_TIMEOUT":          "60s",
				"SERVER_IDLE_TIMEOUT":           "180s",
				"OLLAMA_API_URL":                "http://custom-ollama:8080",
				"GROQ_API_KEY":                  "groq123",
				"OPENAI_API_KEY":                "openai123",
			},
			expectedCfg: defaultConfig(func(cfg *config.Config) {
				cfg.Environment = "development"
				cfg.Telemetry.Enable = true
				cfg.Telemetry.MetricsPushEnable = true
				cfg.Server.Host = "localhost"
				cfg.Server.Port = "9090"
				cfg.Server.ReadTimeout = 60 * time.Second
				cfg.Server.WriteTimeout = 60 * time.Second
				cfg.Server.IdleTimeout = 180 * time.Second
				cfg.Providers = defaultProviders(map[types.Provider]func(*registry.ProviderConfig){
					constants.OllamaID: func(p *registry.ProviderConfig) { p.URL = "http://custom-ollama:8080" },
					constants.GroqID:   func(p *registry.ProviderConfig) { p.Token = "groq123" },
					constants.OpenaiID: func(p *registry.ProviderConfig) { p.Token = "openai123" },
				})
			}),
		},
		{
			name: "PartialEnvVariables",
			env: map[string]string{
				"TELEMETRY_ENABLE": "true",
				"ENVIRONMENT":      "development",
				"OLLAMA_API_URL":   "http://custom-ollama:8080",
			},
			expectedCfg: defaultConfig(func(cfg *config.Config) {
				cfg.Environment = "development"
				cfg.Telemetry.Enable = true
				cfg.Providers = defaultProviders(map[types.Provider]func(*registry.ProviderConfig){
					constants.OllamaID: func(p *registry.ProviderConfig) { p.URL = "http://custom-ollama:8080" },
				})
			}),
		},
		{
			name: "Error_InvalidServerReadTimeout",
			env: map[string]string{
				"SERVER_READ_TIMEOUT": "invalid",
			},
			expectedError: "Server: ReadTimeout: time: invalid duration \"invalid\"",
		},
		{
			name: "Error_InvalidServerWriteTimeout",
			env: map[string]string{
				"SERVER_WRITE_TIMEOUT": "invalid",
			},
			expectedError: "Server: WriteTimeout: time: invalid duration \"invalid\"",
		},
		{
			name: "Error_InvalidServerIdleTimeout",
			env: map[string]string{
				"SERVER_IDLE_TIMEOUT": "invalid",
			},
			expectedError: "Server: IdleTimeout: time: invalid duration \"invalid\"",
		},
		{
			name: "Error_InvalidClientTimeout",
			env: map[string]string{
				"CLIENT_TIMEOUT": "invalid",
			},
			expectedError: "Client: ClientTimeout: time: invalid duration \"invalid\"",
		},
		{
			name: "Error_InvalidClientIdleConnTimeout",
			env: map[string]string{
				"CLIENT_IDLE_CONN_TIMEOUT": "invalid",
			},
			expectedError: "Client: ClientIdleConnTimeout: time: invalid duration \"invalid\"",
		},
		{
			name: "Error_InvalidClientResponseHeaderTimeout",
			env: map[string]string{
				"CLIENT_RESPONSE_HEADER_TIMEOUT": "invalid",
			},
			expectedError: "Client: ClientResponseHeaderTimeout: time: invalid duration \"invalid\"",
		},
		{
			name: "Error_InvalidClientExpectContinueTimeout",
			env: map[string]string{
				"CLIENT_EXPECT_CONTINUE_TIMEOUT": "invalid",
			},
			expectedError: "Client: ClientExpectContinueTimeout: time: invalid duration \"invalid\"",
		},
		{
			name: "Error_InvalidMCPClientTimeout",
			env: map[string]string{
				"MCP_CLIENT_TIMEOUT": "invalid",
			},
			expectedError: "MCP: ClientTimeout: time: invalid duration \"invalid\"",
		},
		{
			name: "Error_InvalidMCPDialTimeout",
			env: map[string]string{
				"MCP_DIAL_TIMEOUT": "invalid",
			},
			expectedError: "MCP: DialTimeout: time: invalid duration \"invalid\"",
		},
		{
			name: "Error_InvalidMCPTlsHandshakeTimeout",
			env: map[string]string{
				"MCP_TLS_HANDSHAKE_TIMEOUT": "invalid",
			},
			expectedError: "MCP: TlsHandshakeTimeout: time: invalid duration \"invalid\"",
		},
		{
			name: "Error_InvalidMCPResponseHeaderTimeout",
			env: map[string]string{
				"MCP_RESPONSE_HEADER_TIMEOUT": "invalid",
			},
			expectedError: "MCP: ResponseHeaderTimeout: time: invalid duration \"invalid\"",
		},
		{
			name: "Error_InvalidMCPExpectContinueTimeout",
			env: map[string]string{
				"MCP_EXPECT_CONTINUE_TIMEOUT": "invalid",
			},
			expectedError: "MCP: ExpectContinueTimeout: time: invalid duration \"invalid\"",
		},
		{
			name: "Error_InvalidMCPRequestTimeout",
			env: map[string]string{
				"MCP_REQUEST_TIMEOUT": "invalid",
			},
			expectedError: "MCP: RequestTimeout: time: invalid duration \"invalid\"",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Config{}
			lookuper := envconfig.MapLookuper(tt.env)

			result, err := cfg.Load(lookuper)

			if tt.expectedError != "" {
				assert.EqualError(t, err, tt.expectedError)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tt.expectedCfg, result)
		})
	}
}

func TestLoadDoesNotMutateRegistryDefaults(t *testing.T) {
	originalURL := registry.Registry[constants.OllamaID].URL
	originalToken := registry.Registry[constants.GroqID].Token

	cfg := &config.Config{}
	_, err := cfg.Load(envconfig.MapLookuper(map[string]string{
		"OLLAMA_API_URL": "http://mutated:1234",
		"GROQ_API_KEY":   "leaked-token",
	}))

	assert.NoError(t, err)
	assert.Equal(t, originalURL, registry.Registry[constants.OllamaID].URL)
	assert.Equal(t, originalToken, registry.Registry[constants.GroqID].Token)
}
