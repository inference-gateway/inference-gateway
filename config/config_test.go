package config_test

import (
	"testing"
	"time"

	"github.com/inference-gateway/inference-gateway/config"
	"github.com/inference-gateway/inference-gateway/providers"
	"github.com/sethvargo/go-envconfig"
	"github.com/stretchr/testify/assert"
)

func TestLoad(t *testing.T) {
	tests := []struct {
		name          string
		env           map[string]string
		expectedCfg   config.Config
		expectedError string
	}{
		{
			name: "Success_Defaults",
			env:  map[string]string{},
			expectedCfg: config.Config{
				ApplicationName: "inference-gateway",
				EnableTelemetry: false,
				Environment:     "production",
				EnableAuth:      false,
				OIDC: &config.OIDC{
					IssuerURL:    "http://keycloak:8080/realms/inference-gateway-realm",
					ClientID:     "inference-gateway-client",
					ClientSecret: "",
				},
				Server: &config.ServerConfig{
					Host:         "0.0.0.0",
					Port:         "8080",
					ReadTimeout:  30 * time.Second,
					WriteTimeout: 30 * time.Second,
					IdleTimeout:  120 * time.Second,
				},
				Providers: map[string]*config.BaseProviderConfig{
					providers.OllamaID: {
						ID:       providers.OllamaID,
						Name:     "Ollama",
						URL:      "http://ollama:8080",
						AuthType: config.ProviderAuthTypeNone,
						Endpoints: struct {
							List     string
							Generate string
						}{},
					},
					providers.GroqID: {
						ID:       providers.GroqID,
						Name:     "Groq",
						URL:      "https://api.groq.com",
						AuthType: config.ProviderAuthTypeBearer,
						Endpoints: struct {
							List     string
							Generate string
						}{},
					},
					providers.OpenaiID: {
						ID:       providers.OpenaiID,
						Name:     "Openai",
						URL:      "https://api.openai.com",
						AuthType: config.ProviderAuthTypeBearer,
						Endpoints: struct {
							List     string
							Generate string
						}{},
					},
					providers.GoogleID: {
						ID:       providers.GoogleID,
						Name:     "Google",
						URL:      "https://generativelanguage.googleapis.com",
						AuthType: config.ProviderAuthTypeQuery,
						Endpoints: struct {
							List     string
							Generate string
						}{},
					},
					providers.CloudflareID: {
						ID:       providers.CloudflareID,
						Name:     "Cloudflare",
						URL:      "https://api.cloudflare.com/client/v4/accounts/{ACCOUNT_ID}",
						AuthType: config.ProviderAuthTypeBearer,
						Endpoints: struct {
							List     string
							Generate string
						}{},
					},
					providers.CohereID: {
						ID:       providers.CohereID,
						Name:     "Cohere",
						URL:      "https://api.cohere.com",
						AuthType: config.ProviderAuthTypeBearer,
						Endpoints: struct {
							List     string
							Generate string
						}{},
					},
					providers.AnthropicID: {
						ID:       providers.AnthropicID,
						Name:     "Anthropic",
						URL:      "https://api.anthropic.com",
						AuthType: config.ProviderAuthTypeXHeader,
						ExtraHeaders: map[string][]string{
							"anthropic-version": {"2023-06-01"},
						},
						Endpoints: struct {
							List     string
							Generate string
						}{},
					},
				},
			},
		},
		{
			name: "Success_AllEnvVariablesSet",
			env: map[string]string{
				"APPLICATION_NAME":     "test-app",
				"ENABLE_TELEMETRY":     "true",
				"ENVIRONMENT":          "development",
				"SERVER_HOST":          "localhost",
				"SERVER_PORT":          "9090",
				"SERVER_READ_TIMEOUT":  "60s",
				"SERVER_WRITE_TIMEOUT": "60s",
				"SERVER_IDLE_TIMEOUT":  "180s",
				"OLLAMA_API_URL":       "http://custom-ollama:8080",
				"GROQ_API_KEY":         "groq123",
				"OPENAI_API_KEY":       "openai123",
				"GOOGLE_API_KEY":       "google123",
			},
			expectedCfg: config.Config{
				ApplicationName: "test-app",
				EnableTelemetry: true,
				Environment:     "development",
				EnableAuth:      false,
				OIDC: &config.OIDC{
					IssuerURL:    "http://keycloak:8080/realms/inference-gateway-realm",
					ClientID:     "inference-gateway-client",
					ClientSecret: "",
				},
				Server: &config.ServerConfig{
					Host:         "localhost",
					Port:         "9090",
					ReadTimeout:  60 * time.Second,
					WriteTimeout: 60 * time.Second,
					IdleTimeout:  180 * time.Second,
				},
				Providers: map[string]*config.BaseProviderConfig{
					providers.OllamaID: {
						ID:       providers.OllamaID,
						Name:     "Ollama",
						URL:      "http://custom-ollama:8080",
						AuthType: config.ProviderAuthTypeNone,
						Endpoints: struct {
							List     string
							Generate string
						}{},
					},
					providers.GroqID: {
						ID:       providers.GroqID,
						Name:     "Groq",
						URL:      "https://api.groq.com",
						Token:    "groq123",
						AuthType: config.ProviderAuthTypeBearer,
						Endpoints: struct {
							List     string
							Generate string
						}{},
					},
					providers.OpenaiID: {
						ID:       providers.OpenaiID,
						Name:     "Openai",
						URL:      "https://api.openai.com",
						Token:    "openai123",
						AuthType: config.ProviderAuthTypeBearer,
						Endpoints: struct {
							List     string
							Generate string
						}{},
					},
					providers.GoogleID: {
						ID:       providers.GoogleID,
						Name:     "Google",
						URL:      "https://generativelanguage.googleapis.com",
						Token:    "google123",
						AuthType: config.ProviderAuthTypeQuery,
						Endpoints: struct {
							List     string
							Generate string
						}{},
					},
					providers.CloudflareID: {
						ID:       providers.CloudflareID,
						Name:     "Cloudflare",
						URL:      "https://api.cloudflare.com/client/v4/accounts/{ACCOUNT_ID}",
						AuthType: config.ProviderAuthTypeBearer,
						Endpoints: struct {
							List     string
							Generate string
						}{},
					},
					providers.CohereID: {
						ID:       providers.CohereID,
						Name:     "Cohere",
						URL:      "https://api.cohere.com",
						AuthType: config.ProviderAuthTypeBearer,
						Endpoints: struct {
							List     string
							Generate string
						}{},
					},
					providers.AnthropicID: {
						ID:       providers.AnthropicID,
						Name:     "Anthropic",
						URL:      "https://api.anthropic.com",
						AuthType: config.ProviderAuthTypeXHeader,
						ExtraHeaders: map[string][]string{
							"anthropic-version": {"2023-06-01"},
						},
						Endpoints: struct {
							List     string
							Generate string
						}{},
					},
				},
			},
		},
		{
			name: "Error_InvalidServerReadTimeout",
			env: map[string]string{
				"SERVER_READ_TIMEOUT": "invalid",
			},
			expectedError: "Server: ReadTimeout(\"invalid\"): time: invalid duration \"invalid\"",
		},
		{
			name: "Error_InvalidServerWriteTimeout",
			env: map[string]string{
				"SERVER_WRITE_TIMEOUT": "invalid",
			},
			expectedError: "Server: WriteTimeout(\"invalid\"): time: invalid duration \"invalid\"",
		},
		{
			name: "Error_InvalidServerIdleTimeout",
			env: map[string]string{
				"SERVER_IDLE_TIMEOUT": "invalid",
			},
			expectedError: "Server: IdleTimeout(\"invalid\"): time: invalid duration \"invalid\"",
		},
		{
			name: "PartialEnvVariables",
			env: map[string]string{
				"ENABLE_TELEMETRY": "true",
				"ENVIRONMENT":      "development",
				"OLLAMA_API_URL":   "http://custom-ollama:8080",
			},
			expectedCfg: config.Config{
				ApplicationName: "inference-gateway",
				EnableTelemetry: true,
				Environment:     "development",
				EnableAuth:      false,
				OIDC: &config.OIDC{
					IssuerURL:    "http://keycloak:8080/realms/inference-gateway-realm",
					ClientID:     "inference-gateway-client",
					ClientSecret: "",
				},
				Server: &config.ServerConfig{
					Host:         "0.0.0.0",
					Port:         "8080",
					ReadTimeout:  30 * time.Second,
					WriteTimeout: 30 * time.Second,
					IdleTimeout:  120 * time.Second,
				},
				Providers: map[string]*config.BaseProviderConfig{
					providers.OllamaID: {
						ID:       providers.OllamaID,
						Name:     "Ollama",
						URL:      "http://custom-ollama:8080",
						AuthType: "none",
						Endpoints: struct {
							List     string
							Generate string
						}{},
					},
					providers.GroqID: {
						ID:       providers.GroqID,
						Name:     "Groq",
						URL:      "https://api.groq.com",
						AuthType: config.ProviderAuthTypeBearer,
						Endpoints: struct {
							List     string
							Generate string
						}{},
					},
					providers.OpenaiID: {
						ID:       providers.OpenaiID,
						Name:     "Openai",
						URL:      "https://api.openai.com",
						AuthType: config.ProviderAuthTypeBearer,
						Endpoints: struct {
							List     string
							Generate string
						}{},
					},
					providers.GoogleID: {
						ID:       providers.GoogleID,
						Name:     "Google",
						URL:      "https://generativelanguage.googleapis.com",
						AuthType: "query",
						Endpoints: struct {
							List     string
							Generate string
						}{},
					},
					providers.CloudflareID: {
						ID:       providers.CloudflareID,
						Name:     "Cloudflare",
						URL:      "https://api.cloudflare.com/client/v4/accounts/{ACCOUNT_ID}",
						AuthType: config.ProviderAuthTypeBearer,
						Endpoints: struct {
							List     string
							Generate string
						}{},
					},
					providers.CohereID: {
						ID:       providers.CohereID,
						Name:     "Cohere",
						URL:      "https://api.cohere.com",
						AuthType: config.ProviderAuthTypeBearer,
						Endpoints: struct {
							List     string
							Generate string
						}{},
					},
					providers.AnthropicID: {
						ID:       providers.AnthropicID,
						Name:     "Anthropic",
						URL:      "https://api.anthropic.com",
						AuthType: "xheader",
						ExtraHeaders: map[string][]string{
							"anthropic-version": {"2023-06-01"},
						},
						Endpoints: struct {
							List     string
							Generate string
						}{},
					},
				},
			},
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
