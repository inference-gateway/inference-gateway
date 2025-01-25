package config_test

import (
	"testing"
	"time"

	"github.com/inference-gateway/inference-gateway/config"
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
				Ollama: &config.OllamaConfig{
					ID:       "ollama",
					Name:     "Ollama",
					URL:      "http://ollama:8080",
					AuthType: "none",
					Endpoints: struct {
						List     string
						Generate string
					}{},
				},
				Groq: &config.GroqConfig{
					ID:       "groq",
					Name:     "Groq",
					URL:      "https://api.groq.com",
					AuthType: "bearer",
					Endpoints: struct {
						List     string
						Generate string
					}{},
				},
				Openai: &config.OpenaiConfig{
					ID:       "openai",
					Name:     "Openai",
					URL:      "https://api.openai.com",
					AuthType: "bearer",
					Endpoints: struct {
						List     string
						Generate string
					}{},
				},
				Google: &config.GoogleConfig{
					ID:       "google",
					Name:     "Google",
					URL:      "https://generativelanguage.googleapis.com",
					AuthType: "query",
					Endpoints: struct {
						List     string
						Generate string
					}{},
				},
				Cloudflare: &config.CloudflareConfig{
					ID:       "cloudflare",
					Name:     "Cloudflare",
					URL:      "https://api.cloudflare.com/client/v4/accounts/{ACCOUNT_ID}",
					AuthType: "bearer",
					Endpoints: struct {
						List     string
						Generate string
					}{},
				},
				Cohere: &config.CohereConfig{
					ID:       "cohere",
					Name:     "Cohere",
					URL:      "https://api.cohere.com",
					AuthType: "bearer",
					Endpoints: struct {
						List     string
						Generate string
					}{},
				},
				Anthropic: &config.AnthropicConfig{
					ID:       "anthropic",
					Name:     "Anthropic",
					URL:      "https://api.anthropic.com",
					AuthType: "xheader",
					Endpoints: struct {
						List     string
						Generate string
					}{},
					ExtraHeaders: map[string][]string{
						"anthropic-version": {"2023-06-01"},
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
				Ollama: &config.OllamaConfig{
					ID:       "ollama",
					Name:     "Ollama",
					URL:      "http://custom-ollama:8080",
					Token:    "",
					AuthType: "none",
				},
				Groq: &config.GroqConfig{
					ID:       "groq",
					Name:     "Groq",
					URL:      "https://api.groq.com",
					Token:    "groq123",
					AuthType: "bearer",
				},
				Openai: &config.OpenaiConfig{
					ID:       "openai",
					Name:     "Openai",
					URL:      "https://api.openai.com",
					Token:    "openai123",
					AuthType: "bearer",
				},
				Google: &config.GoogleConfig{
					ID:       "google",
					Name:     "Google",
					URL:      "https://generativelanguage.googleapis.com",
					Token:    "google123",
					AuthType: "query",
				},
				Cloudflare: &config.CloudflareConfig{
					ID:       "cloudflare",
					Name:     "Cloudflare",
					URL:      "https://api.cloudflare.com/client/v4/accounts/{ACCOUNT_ID}",
					Token:    "",
					AuthType: "bearer",
				},
				Cohere: &config.CohereConfig{
					ID:       "cohere",
					Name:     "Cohere",
					URL:      "https://api.cohere.com",
					Token:    "",
					AuthType: "bearer",
				},
				Anthropic: &config.AnthropicConfig{
					ID:       "anthropic",
					Name:     "Anthropic",
					URL:      "https://api.anthropic.com",
					Token:    "",
					AuthType: "xheader",
					ExtraHeaders: map[string][]string{
						"anthropic-version": {"2023-06-01"},
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
				Ollama: &config.OllamaConfig{
					ID:       "ollama",
					Name:     "Ollama",
					URL:      "http://custom-ollama:8080",
					AuthType: "none",
					Endpoints: struct {
						List     string
						Generate string
					}{},
				},
				Groq: &config.GroqConfig{
					ID:       "groq",
					Name:     "Groq",
					AuthType: "bearer",
					URL:      "https://api.groq.com",
					Endpoints: struct {
						List     string
						Generate string
					}{},
				},
				Openai: &config.OpenaiConfig{
					ID:       "openai",
					Name:     "Openai",
					AuthType: "bearer",
					URL:      "https://api.openai.com",
					Endpoints: struct {
						List     string
						Generate string
					}{},
				},
				Google: &config.GoogleConfig{
					ID:       "google",
					Name:     "Google",
					AuthType: "query",
					URL:      "https://generativelanguage.googleapis.com",
					Endpoints: struct {
						List     string
						Generate string
					}{},
				},
				Cloudflare: &config.CloudflareConfig{
					ID:       "cloudflare",
					Name:     "Cloudflare",
					AuthType: "bearer",
					URL:      "https://api.cloudflare.com/client/v4/accounts/{ACCOUNT_ID}",
					Endpoints: struct {
						List     string
						Generate string
					}{},
				},
				Cohere: &config.CohereConfig{
					ID:       "cohere",
					Name:     "Cohere",
					AuthType: "bearer",
					URL:      "https://api.cohere.com",
					Endpoints: struct {
						List     string
						Generate string
					}{},
				},
				Anthropic: &config.AnthropicConfig{
					ID:       "anthropic",
					Name:     "Anthropic",
					AuthType: "xheader",
					URL:      "https://api.anthropic.com",
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
