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
				EnableTelemetry: false,
				Environment:     "production",
				EnableAuth:      false,
				MCP: &config.MCPConfig{
					Enable:                false,
					Expose:                false,
					Servers:               "",
					ClientTimeout:         5 * time.Second,
					DialTimeout:           3 * time.Second,
					TlsHandshakeTimeout:   3 * time.Second,
					ResponseHeaderTimeout: 3 * time.Second,
					ExpectContinueTimeout: 1 * time.Second,
					RequestTimeout:        5 * time.Second,
				},
				A2A: &config.A2AConfig{
					Enable:        false,
					Agents:        "",
					ClientTimeout: 30 * time.Second,
				},
				OIDC: &config.OIDC{
					IssuerUrl:    "http://keycloak:8080/realms/inference-gateway-realm",
					ClientId:     "inference-gateway-client",
					ClientSecret: "",
				},
				Server: &config.ServerConfig{
					Host:         "0.0.0.0",
					Port:         "8080",
					ReadTimeout:  30 * time.Second,
					WriteTimeout: 30 * time.Second,
					IdleTimeout:  120 * time.Second,
				},
				Client: &config.ClientConfig{
					Timeout:               30 * time.Second,
					MaxIdleConns:          20,
					MaxIdleConnsPerHost:   20,
					IdleConnTimeout:       30 * time.Second,
					TlsMinVersion:         "TLS12",
					DisableCompression:    true,
					ResponseHeaderTimeout: 10 * time.Second,
					ExpectContinueTimeout: 1 * time.Second,
				},
				Providers: map[providers.Provider]*providers.Config{
					providers.AnthropicID: {
						ID:       providers.AnthropicID,
						Name:     providers.AnthropicDisplayName,
						URL:      providers.AnthropicDefaultBaseURL,
						AuthType: providers.AuthTypeXheader,
						ExtraHeaders: map[string][]string{
							"anthropic-version": {"2023-06-01"},
						},
						Endpoints: providers.Endpoints{
							Models: providers.AnthropicModelsEndpoint,
							Chat:   providers.AnthropicChatEndpoint,
						},
					},
					providers.CloudflareID: {
						ID:       providers.CloudflareID,
						Name:     providers.CloudflareDisplayName,
						URL:      providers.CloudflareDefaultBaseURL,
						AuthType: providers.AuthTypeBearer,
						Endpoints: providers.Endpoints{
							Models: providers.CloudflareModelsEndpoint,
							Chat:   providers.CloudflareChatEndpoint,
						},
					},
					providers.CohereID: {
						ID:       providers.CohereID,
						Name:     providers.CohereDisplayName,
						URL:      providers.CohereDefaultBaseURL,
						AuthType: providers.AuthTypeBearer,
						Endpoints: providers.Endpoints{
							Models: providers.CohereModelsEndpoint,
							Chat:   providers.CohereChatEndpoint,
						},
					},
					providers.GroqID: {
						ID:       providers.GroqID,
						Name:     providers.GroqDisplayName,
						URL:      providers.GroqDefaultBaseURL,
						AuthType: providers.AuthTypeBearer,
						Endpoints: providers.Endpoints{
							Models: providers.GroqModelsEndpoint,
							Chat:   providers.GroqChatEndpoint,
						},
					},
					providers.OllamaID: {
						ID:       providers.OllamaID,
						Name:     providers.OllamaDisplayName,
						URL:      providers.OllamaDefaultBaseURL,
						AuthType: providers.AuthTypeNone,
						Endpoints: providers.Endpoints{
							Models: providers.OllamaModelsEndpoint,
							Chat:   providers.OllamaChatEndpoint,
						},
					},
					providers.OpenaiID: {
						ID:       providers.OpenaiID,
						Name:     providers.OpenaiDisplayName,
						URL:      providers.OpenaiDefaultBaseURL,
						AuthType: providers.AuthTypeBearer,
						Endpoints: providers.Endpoints{
							Models: providers.OpenaiModelsEndpoint,
							Chat:   providers.OpenaiChatEndpoint,
						},
					},
					providers.DeepseekID: {
						ID:       providers.DeepseekID,
						Name:     providers.DeepseekDisplayName,
						URL:      providers.DeepseekDefaultBaseURL,
						AuthType: providers.AuthTypeBearer,
						Endpoints: providers.Endpoints{
							Models: providers.DeepseekModelsEndpoint,
							Chat:   providers.DeepseekChatEndpoint,
						},
					},
				},
			},
		},
		{
			name: "Success_AllEnvVariablesSet",
			env: map[string]string{
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
			},
			expectedCfg: config.Config{
				EnableTelemetry: true,
				Environment:     "development",
				EnableAuth:      false,
				MCP: &config.MCPConfig{
					Enable:                false,
					Expose:                false,
					Servers:               "",
					ClientTimeout:         5 * time.Second,
					DialTimeout:           3 * time.Second,
					TlsHandshakeTimeout:   3 * time.Second,
					ResponseHeaderTimeout: 3 * time.Second,
					ExpectContinueTimeout: 1 * time.Second,
					RequestTimeout:        5 * time.Second,
				},
				A2A: &config.A2AConfig{
					Enable:        false,
					Agents:        "",
					ClientTimeout: 30 * time.Second,
				},
				OIDC: &config.OIDC{
					IssuerUrl:    "http://keycloak:8080/realms/inference-gateway-realm",
					ClientId:     "inference-gateway-client",
					ClientSecret: "",
				},
				Server: &config.ServerConfig{
					Host:         "localhost",
					Port:         "9090",
					ReadTimeout:  60 * time.Second,
					WriteTimeout: 60 * time.Second,
					IdleTimeout:  180 * time.Second,
				},
				Client: &config.ClientConfig{
					Timeout:               30 * time.Second,
					MaxIdleConns:          20,
					MaxIdleConnsPerHost:   20,
					IdleConnTimeout:       30 * time.Second,
					TlsMinVersion:         "TLS12",
					DisableCompression:    true,
					ResponseHeaderTimeout: 10 * time.Second,
					ExpectContinueTimeout: 1 * time.Second,
				},
				Providers: map[providers.Provider]*providers.Config{
					providers.OllamaID: {
						ID:       providers.OllamaID,
						Name:     providers.OllamaDisplayName,
						URL:      "http://custom-ollama:8080",
						AuthType: providers.AuthTypeNone,
						Endpoints: providers.Endpoints{
							Models: providers.OllamaModelsEndpoint,
							Chat:   providers.OllamaChatEndpoint,
						},
					},
					providers.GroqID: {
						ID:       providers.GroqID,
						Name:     providers.GroqDisplayName,
						URL:      providers.GroqDefaultBaseURL,
						Token:    "groq123",
						AuthType: providers.AuthTypeBearer,
						Endpoints: providers.Endpoints{
							Models: providers.GroqModelsEndpoint,
							Chat:   providers.GroqChatEndpoint,
						},
					},
					providers.OpenaiID: {
						ID:       providers.OpenaiID,
						Name:     providers.OpenaiDisplayName,
						URL:      providers.OpenaiDefaultBaseURL,
						Token:    "openai123",
						AuthType: providers.AuthTypeBearer,
						Endpoints: providers.Endpoints{
							Models: providers.OpenaiModelsEndpoint,
							Chat:   providers.OpenaiChatEndpoint,
						},
					},
					providers.CloudflareID: {
						ID:       providers.CloudflareID,
						Name:     providers.CloudflareDisplayName,
						URL:      providers.CloudflareDefaultBaseURL,
						AuthType: providers.AuthTypeBearer,
						Endpoints: providers.Endpoints{
							Models: providers.CloudflareModelsEndpoint,
							Chat:   providers.CloudflareChatEndpoint,
						},
					},
					providers.CohereID: {
						ID:       providers.CohereID,
						Name:     providers.CohereDisplayName,
						URL:      providers.CohereDefaultBaseURL,
						AuthType: providers.AuthTypeBearer,
						Endpoints: providers.Endpoints{
							Models: providers.CohereModelsEndpoint,
							Chat:   providers.CohereChatEndpoint,
						},
					},
					providers.AnthropicID: {
						ID:       providers.AnthropicID,
						Name:     providers.AnthropicDisplayName,
						URL:      providers.AnthropicDefaultBaseURL,
						AuthType: providers.AuthTypeXheader,
						ExtraHeaders: map[string][]string{
							"anthropic-version": {"2023-06-01"},
						},
						Endpoints: providers.Endpoints{
							Models: providers.AnthropicModelsEndpoint,
							Chat:   providers.AnthropicChatEndpoint,
						},
					},
					providers.DeepseekID: {
						ID:       providers.DeepseekID,
						Name:     providers.DeepseekDisplayName,
						URL:      providers.DeepseekDefaultBaseURL,
						AuthType: providers.AuthTypeBearer,
						Endpoints: providers.Endpoints{
							Models: providers.DeepseekModelsEndpoint,
							Chat:   providers.DeepseekChatEndpoint,
						},
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
				EnableTelemetry: true,
				Environment:     "development",
				EnableAuth:      false,
				MCP: &config.MCPConfig{
					Enable:                false,
					Expose:                false,
					Servers:               "",
					ClientTimeout:         5 * time.Second,
					DialTimeout:           3 * time.Second,
					TlsHandshakeTimeout:   3 * time.Second,
					ResponseHeaderTimeout: 3 * time.Second,
					ExpectContinueTimeout: 1 * time.Second,
					RequestTimeout:        5 * time.Second,
				},
				A2A: &config.A2AConfig{
					Enable:        false,
					Agents:        "",
					ClientTimeout: 30 * time.Second,
				},
				OIDC: &config.OIDC{
					IssuerUrl:    "http://keycloak:8080/realms/inference-gateway-realm",
					ClientId:     "inference-gateway-client",
					ClientSecret: "",
				},
				Server: &config.ServerConfig{
					Host:         "0.0.0.0",
					Port:         "8080",
					ReadTimeout:  30 * time.Second,
					WriteTimeout: 30 * time.Second,
					IdleTimeout:  120 * time.Second,
				},
				Client: &config.ClientConfig{
					Timeout:               30 * time.Second,
					MaxIdleConns:          20,
					MaxIdleConnsPerHost:   20,
					IdleConnTimeout:       30 * time.Second,
					TlsMinVersion:         "TLS12",
					DisableCompression:    true,
					ResponseHeaderTimeout: 10 * time.Second,
					ExpectContinueTimeout: 1 * time.Second,
				},
				Providers: map[providers.Provider]*providers.Config{
					providers.OllamaID: {
						ID:       providers.OllamaID,
						Name:     providers.OllamaDisplayName,
						URL:      "http://custom-ollama:8080",
						AuthType: providers.AuthTypeNone,
						Endpoints: providers.Endpoints{
							Models: providers.OllamaModelsEndpoint,
							Chat:   providers.OllamaChatEndpoint,
						},
					},
					providers.GroqID: {
						ID:       providers.GroqID,
						Name:     providers.GroqDisplayName,
						URL:      providers.GroqDefaultBaseURL,
						AuthType: providers.AuthTypeBearer,
						Endpoints: providers.Endpoints{
							Models: providers.GroqModelsEndpoint,
							Chat:   providers.GroqChatEndpoint,
						},
					},
					providers.OpenaiID: {
						ID:       providers.OpenaiID,
						Name:     providers.OpenaiDisplayName,
						URL:      providers.OpenaiDefaultBaseURL,
						AuthType: providers.AuthTypeBearer,
						Endpoints: providers.Endpoints{
							Models: providers.OpenaiModelsEndpoint,
							Chat:   providers.OpenaiChatEndpoint,
						},
					},
					providers.CloudflareID: {
						ID:       providers.CloudflareID,
						Name:     providers.CloudflareDisplayName,
						URL:      providers.CloudflareDefaultBaseURL,
						AuthType: providers.AuthTypeBearer,
						Endpoints: providers.Endpoints{
							Models: providers.CloudflareModelsEndpoint,
							Chat:   providers.CloudflareChatEndpoint,
						},
					},
					providers.CohereID: {
						ID:       providers.CohereID,
						Name:     providers.CohereDisplayName,
						URL:      providers.CohereDefaultBaseURL,
						AuthType: providers.AuthTypeBearer,
						Endpoints: providers.Endpoints{
							Models: providers.CohereModelsEndpoint,
							Chat:   providers.CohereChatEndpoint,
						},
					},
					providers.AnthropicID: {
						ID:       providers.AnthropicID,
						Name:     providers.AnthropicDisplayName,
						URL:      providers.AnthropicDefaultBaseURL,
						AuthType: providers.AuthTypeXheader,
						ExtraHeaders: map[string][]string{
							"anthropic-version": {"2023-06-01"},
						},
						Endpoints: providers.Endpoints{
							Models: providers.AnthropicModelsEndpoint,
							Chat:   providers.AnthropicChatEndpoint,
						},
					},
					providers.DeepseekID: {
						ID:       providers.DeepseekID,
						Name:     providers.DeepseekDisplayName,
						URL:      providers.DeepseekDefaultBaseURL,
						AuthType: providers.AuthTypeBearer,
						Endpoints: providers.Endpoints{
							Models: providers.DeepseekModelsEndpoint,
							Chat:   providers.DeepseekChatEndpoint,
						},
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
