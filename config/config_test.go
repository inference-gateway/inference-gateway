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
						ID:             providers.AnthropicID,
						Name:           providers.AnthropicDisplayName,
						URL:            providers.AnthropicDefaultBaseURL,
						AuthType:       providers.AuthTypeXheader,
						SupportsVision: true,
						ExtraHeaders: map[string][]string{
							"anthropic-version": {"2023-06-01"},
						},
						Endpoints: providers.Endpoints{
							Models: providers.AnthropicModelsEndpoint,
							Chat:   providers.AnthropicChatEndpoint,
						},
					},
					providers.CloudflareID: {
						ID:             providers.CloudflareID,
						Name:           providers.CloudflareDisplayName,
						URL:            providers.CloudflareDefaultBaseURL,
						AuthType:       providers.AuthTypeBearer,
						SupportsVision: false,
						Endpoints: providers.Endpoints{
							Models: providers.CloudflareModelsEndpoint,
							Chat:   providers.CloudflareChatEndpoint,
						},
					},
					providers.CohereID: {
						ID:             providers.CohereID,
						Name:           providers.CohereDisplayName,
						URL:            providers.CohereDefaultBaseURL,
						AuthType:       providers.AuthTypeBearer,
						SupportsVision: true,
						Endpoints: providers.Endpoints{
							Models: providers.CohereModelsEndpoint,
							Chat:   providers.CohereChatEndpoint,
						},
					},
					providers.GroqID: {
						ID:             providers.GroqID,
						Name:           providers.GroqDisplayName,
						URL:            providers.GroqDefaultBaseURL,
						AuthType:       providers.AuthTypeBearer,
						SupportsVision: true,
						Endpoints: providers.Endpoints{
							Models: providers.GroqModelsEndpoint,
							Chat:   providers.GroqChatEndpoint,
						},
					},
					providers.OllamaID: {
						ID:             providers.OllamaID,
						Name:           providers.OllamaDisplayName,
						URL:            providers.OllamaDefaultBaseURL,
						AuthType:       providers.AuthTypeNone,
						SupportsVision: true,
						Endpoints: providers.Endpoints{
							Models: providers.OllamaModelsEndpoint,
							Chat:   providers.OllamaChatEndpoint,
						},
					},
					providers.OllamaCloudID: {
						ID:             providers.OllamaCloudID,
						Name:           providers.OllamaCloudDisplayName,
						URL:            providers.OllamaCloudDefaultBaseURL,
						AuthType:       providers.AuthTypeBearer,
						SupportsVision: true,
						Endpoints: providers.Endpoints{
							Models: providers.OllamaCloudModelsEndpoint,
							Chat:   providers.OllamaCloudChatEndpoint,
						},
					},
					providers.OpenaiID: {
						ID:             providers.OpenaiID,
						Name:           providers.OpenaiDisplayName,
						URL:            providers.OpenaiDefaultBaseURL,
						AuthType:       providers.AuthTypeBearer,
						SupportsVision: true,
						Endpoints: providers.Endpoints{
							Models: providers.OpenaiModelsEndpoint,
							Chat:   providers.OpenaiChatEndpoint,
						},
					},
					providers.DeepseekID: {
						ID:             providers.DeepseekID,
						Name:           providers.DeepseekDisplayName,
						URL:            providers.DeepseekDefaultBaseURL,
						AuthType:       providers.AuthTypeBearer,
						SupportsVision: false,
						Endpoints: providers.Endpoints{
							Models: providers.DeepseekModelsEndpoint,
							Chat:   providers.DeepseekChatEndpoint,
						},
					},
					providers.GoogleID: {
						ID:             providers.GoogleID,
						Name:           providers.GoogleDisplayName,
						URL:            providers.GoogleDefaultBaseURL,
						AuthType:       providers.AuthTypeBearer,
						SupportsVision: true,
						Endpoints: providers.Endpoints{
							Models: providers.GoogleModelsEndpoint,
							Chat:   providers.GoogleChatEndpoint,
						},
					},
					providers.MistralID: {
						ID:             providers.MistralID,
						Name:           providers.MistralDisplayName,
						URL:            providers.MistralDefaultBaseURL,
						AuthType:       providers.AuthTypeBearer,
						SupportsVision: true,
						Endpoints: providers.Endpoints{
							Models: providers.MistralModelsEndpoint,
							Chat:   providers.MistralChatEndpoint,
						},
					},
					providers.MoonshotID: {
						ID:             providers.MoonshotID,
						Name:           providers.MoonshotDisplayName,
						URL:            providers.MoonshotDefaultBaseURL,
						AuthType:       providers.AuthTypeBearer,
						SupportsVision: false,
						Endpoints: providers.Endpoints{
							Models: providers.MoonshotModelsEndpoint,
							Chat:   providers.MoonshotChatEndpoint,
						},
					},
				},
			},
		},
		{
			name: "Success_AllEnvVariablesSet",
			env: map[string]string{
				"TELEMETRY_ENABLE":     "true",
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
				Environment:               "development",
				AllowedModels:             "",
				DebugContentTruncateWords: 10,
				DebugMaxMessages:          100,
				Telemetry: &config.TelemetryConfig{
					Enable:      true,
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
						ID:             providers.OllamaID,
						Name:           providers.OllamaDisplayName,
						URL:            "http://custom-ollama:8080",
						AuthType:       providers.AuthTypeNone,
						SupportsVision: true,
						Endpoints: providers.Endpoints{
							Models: providers.OllamaModelsEndpoint,
							Chat:   providers.OllamaChatEndpoint,
						},
					},
					providers.OllamaCloudID: {
						ID:             providers.OllamaCloudID,
						Name:           providers.OllamaCloudDisplayName,
						URL:            providers.OllamaCloudDefaultBaseURL,
						AuthType:       providers.AuthTypeBearer,
						SupportsVision: true,
						Endpoints: providers.Endpoints{
							Models: providers.OllamaCloudModelsEndpoint,
							Chat:   providers.OllamaCloudChatEndpoint,
						},
					},
					providers.GroqID: {
						ID:             providers.GroqID,
						Name:           providers.GroqDisplayName,
						URL:            providers.GroqDefaultBaseURL,
						Token:          "groq123",
						AuthType:       providers.AuthTypeBearer,
						SupportsVision: true,
						Endpoints: providers.Endpoints{
							Models: providers.GroqModelsEndpoint,
							Chat:   providers.GroqChatEndpoint,
						},
					},
					providers.OpenaiID: {
						ID:             providers.OpenaiID,
						Name:           providers.OpenaiDisplayName,
						URL:            providers.OpenaiDefaultBaseURL,
						Token:          "openai123",
						AuthType:       providers.AuthTypeBearer,
						SupportsVision: true,
						Endpoints: providers.Endpoints{
							Models: providers.OpenaiModelsEndpoint,
							Chat:   providers.OpenaiChatEndpoint,
						},
					},
					providers.CloudflareID: {
						ID:             providers.CloudflareID,
						Name:           providers.CloudflareDisplayName,
						URL:            providers.CloudflareDefaultBaseURL,
						AuthType:       providers.AuthTypeBearer,
						SupportsVision: false,
						Endpoints: providers.Endpoints{
							Models: providers.CloudflareModelsEndpoint,
							Chat:   providers.CloudflareChatEndpoint,
						},
					},
					providers.CohereID: {
						ID:             providers.CohereID,
						Name:           providers.CohereDisplayName,
						URL:            providers.CohereDefaultBaseURL,
						AuthType:       providers.AuthTypeBearer,
						SupportsVision: true,
						Endpoints: providers.Endpoints{
							Models: providers.CohereModelsEndpoint,
							Chat:   providers.CohereChatEndpoint,
						},
					},
					providers.AnthropicID: {
						ID:             providers.AnthropicID,
						Name:           providers.AnthropicDisplayName,
						URL:            providers.AnthropicDefaultBaseURL,
						AuthType:       providers.AuthTypeXheader,
						SupportsVision: true,
						ExtraHeaders: map[string][]string{
							"anthropic-version": {"2023-06-01"},
						},
						Endpoints: providers.Endpoints{
							Models: providers.AnthropicModelsEndpoint,
							Chat:   providers.AnthropicChatEndpoint,
						},
					},
					providers.DeepseekID: {
						ID:             providers.DeepseekID,
						Name:           providers.DeepseekDisplayName,
						URL:            providers.DeepseekDefaultBaseURL,
						AuthType:       providers.AuthTypeBearer,
						SupportsVision: false,
						Endpoints: providers.Endpoints{
							Models: providers.DeepseekModelsEndpoint,
							Chat:   providers.DeepseekChatEndpoint,
						},
					},
					providers.GoogleID: {
						ID:             providers.GoogleID,
						Name:           providers.GoogleDisplayName,
						URL:            providers.GoogleDefaultBaseURL,
						AuthType:       providers.AuthTypeBearer,
						SupportsVision: true,
						Endpoints: providers.Endpoints{
							Models: providers.GoogleModelsEndpoint,
							Chat:   providers.GoogleChatEndpoint,
						},
					},
					providers.MistralID: {
						ID:             providers.MistralID,
						Name:           providers.MistralDisplayName,
						URL:            providers.MistralDefaultBaseURL,
						AuthType:       providers.AuthTypeBearer,
						SupportsVision: true,
						Endpoints: providers.Endpoints{
							Models: providers.MistralModelsEndpoint,
							Chat:   providers.MistralChatEndpoint,
						},
					},
					providers.MoonshotID: {
						ID:             providers.MoonshotID,
						Name:           providers.MoonshotDisplayName,
						URL:            providers.MoonshotDefaultBaseURL,
						AuthType:       providers.AuthTypeBearer,
						SupportsVision: false,
						Endpoints: providers.Endpoints{
							Models: providers.MoonshotModelsEndpoint,
							Chat:   providers.MoonshotChatEndpoint,
						},
					},
				},
			},
		},
		{
			name: "PartialEnvVariables",
			env: map[string]string{
				"TELEMETRY_ENABLE": "true",
				"ENVIRONMENT":      "development",
				"OLLAMA_API_URL":   "http://custom-ollama:8080",
			},
			expectedCfg: config.Config{
				Environment:               "development",
				AllowedModels:             "",
				DebugContentTruncateWords: 10,
				DebugMaxMessages:          100,
				Telemetry: &config.TelemetryConfig{
					Enable:      true,
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
						ID:             providers.OllamaID,
						Name:           providers.OllamaDisplayName,
						URL:            "http://custom-ollama:8080",
						AuthType:       providers.AuthTypeNone,
						SupportsVision: true,
						Endpoints: providers.Endpoints{
							Models: providers.OllamaModelsEndpoint,
							Chat:   providers.OllamaChatEndpoint,
						},
					},
					providers.OllamaCloudID: {
						ID:             providers.OllamaCloudID,
						Name:           providers.OllamaCloudDisplayName,
						URL:            providers.OllamaCloudDefaultBaseURL,
						AuthType:       providers.AuthTypeBearer,
						SupportsVision: true,
						Endpoints: providers.Endpoints{
							Models: providers.OllamaCloudModelsEndpoint,
							Chat:   providers.OllamaCloudChatEndpoint,
						},
					},
					providers.GroqID: {
						ID:             providers.GroqID,
						Name:           providers.GroqDisplayName,
						URL:            providers.GroqDefaultBaseURL,
						AuthType:       providers.AuthTypeBearer,
						SupportsVision: true,
						Endpoints: providers.Endpoints{
							Models: providers.GroqModelsEndpoint,
							Chat:   providers.GroqChatEndpoint,
						},
					},
					providers.OpenaiID: {
						ID:             providers.OpenaiID,
						Name:           providers.OpenaiDisplayName,
						URL:            providers.OpenaiDefaultBaseURL,
						AuthType:       providers.AuthTypeBearer,
						SupportsVision: true,
						Endpoints: providers.Endpoints{
							Models: providers.OpenaiModelsEndpoint,
							Chat:   providers.OpenaiChatEndpoint,
						},
					},
					providers.CloudflareID: {
						ID:             providers.CloudflareID,
						Name:           providers.CloudflareDisplayName,
						URL:            providers.CloudflareDefaultBaseURL,
						AuthType:       providers.AuthTypeBearer,
						SupportsVision: false,
						Endpoints: providers.Endpoints{
							Models: providers.CloudflareModelsEndpoint,
							Chat:   providers.CloudflareChatEndpoint,
						},
					},
					providers.CohereID: {
						ID:             providers.CohereID,
						Name:           providers.CohereDisplayName,
						URL:            providers.CohereDefaultBaseURL,
						AuthType:       providers.AuthTypeBearer,
						SupportsVision: true,
						Endpoints: providers.Endpoints{
							Models: providers.CohereModelsEndpoint,
							Chat:   providers.CohereChatEndpoint,
						},
					},
					providers.AnthropicID: {
						ID:             providers.AnthropicID,
						Name:           providers.AnthropicDisplayName,
						URL:            providers.AnthropicDefaultBaseURL,
						AuthType:       providers.AuthTypeXheader,
						SupportsVision: true,
						ExtraHeaders: map[string][]string{
							"anthropic-version": {"2023-06-01"},
						},
						Endpoints: providers.Endpoints{
							Models: providers.AnthropicModelsEndpoint,
							Chat:   providers.AnthropicChatEndpoint,
						},
					},
					providers.DeepseekID: {
						ID:             providers.DeepseekID,
						Name:           providers.DeepseekDisplayName,
						URL:            providers.DeepseekDefaultBaseURL,
						AuthType:       providers.AuthTypeBearer,
						SupportsVision: false,
						Endpoints: providers.Endpoints{
							Models: providers.DeepseekModelsEndpoint,
							Chat:   providers.DeepseekChatEndpoint,
						},
					},
					providers.GoogleID: {
						ID:             providers.GoogleID,
						Name:           providers.GoogleDisplayName,
						URL:            providers.GoogleDefaultBaseURL,
						AuthType:       providers.AuthTypeBearer,
						SupportsVision: true,
						Endpoints: providers.Endpoints{
							Models: providers.GoogleModelsEndpoint,
							Chat:   providers.GoogleChatEndpoint,
						},
					},
					providers.MistralID: {
						ID:             providers.MistralID,
						Name:           providers.MistralDisplayName,
						URL:            providers.MistralDefaultBaseURL,
						AuthType:       providers.AuthTypeBearer,
						SupportsVision: true,
						Endpoints: providers.Endpoints{
							Models: providers.MistralModelsEndpoint,
							Chat:   providers.MistralChatEndpoint,
						},
					},
					providers.MoonshotID: {
						ID:             providers.MoonshotID,
						Name:           providers.MoonshotDisplayName,
						URL:            providers.MoonshotDefaultBaseURL,
						AuthType:       providers.AuthTypeBearer,
						SupportsVision: false,
						Endpoints: providers.Endpoints{
							Models: providers.MoonshotModelsEndpoint,
							Chat:   providers.MoonshotChatEndpoint,
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
			expectedError: "Client: Timeout: time: invalid duration \"invalid\"",
		},
		{
			name: "Error_InvalidClientIdleConnTimeout",
			env: map[string]string{
				"CLIENT_IDLE_CONN_TIMEOUT": "invalid",
			},
			expectedError: "Client: IdleConnTimeout: time: invalid duration \"invalid\"",
		},
		{
			name: "Error_InvalidClientResponseHeaderTimeout",
			env: map[string]string{
				"CLIENT_RESPONSE_HEADER_TIMEOUT": "invalid",
			},
			expectedError: "Client: ResponseHeaderTimeout: time: invalid duration \"invalid\"",
		},
		{
			name: "Error_InvalidClientExpectContinueTimeout",
			env: map[string]string{
				"CLIENT_EXPECT_CONTINUE_TIMEOUT": "invalid",
			},
			expectedError: "Client: ExpectContinueTimeout: time: invalid duration \"invalid\"",
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
