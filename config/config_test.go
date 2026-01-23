package config_test

import (
	"testing"
	"time"

	"github.com/inference-gateway/inference-gateway/config"
	"github.com/inference-gateway/inference-gateway/providers/constants"
	"github.com/inference-gateway/inference-gateway/providers/registry"
	"github.com/inference-gateway/inference-gateway/providers/types"
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
				Providers: map[types.Provider]*registry.ProviderConfig{
					constants.AnthropicID: {
						ID:             constants.AnthropicID,
						Name:           constants.AnthropicDisplayName,
						URL:            constants.AnthropicDefaultBaseURL,
						AuthType:       constants.AuthTypeXheader,
						SupportsVision: true,
						ExtraHeaders: map[string][]string{
							"anthropic-version": {"2023-06-01"},
						},
						Endpoints: types.Endpoints{
							Models: constants.AnthropicModelsEndpoint,
							Chat:   constants.AnthropicChatEndpoint,
						},
					},
					constants.CloudflareID: {
						ID:             constants.CloudflareID,
						Name:           constants.CloudflareDisplayName,
						URL:            constants.CloudflareDefaultBaseURL,
						AuthType:       constants.AuthTypeBearer,
						SupportsVision: false,
						Endpoints: types.Endpoints{
							Models: constants.CloudflareModelsEndpoint,
							Chat:   constants.CloudflareChatEndpoint,
						},
					},
					constants.CohereID: {
						ID:             constants.CohereID,
						Name:           constants.CohereDisplayName,
						URL:            constants.CohereDefaultBaseURL,
						AuthType:       constants.AuthTypeBearer,
						SupportsVision: true,
						Endpoints: types.Endpoints{
							Models: constants.CohereModelsEndpoint,
							Chat:   constants.CohereChatEndpoint,
						},
					},
					constants.GroqID: {
						ID:             constants.GroqID,
						Name:           constants.GroqDisplayName,
						URL:            constants.GroqDefaultBaseURL,
						AuthType:       constants.AuthTypeBearer,
						SupportsVision: true,
						Endpoints: types.Endpoints{
							Models: constants.GroqModelsEndpoint,
							Chat:   constants.GroqChatEndpoint,
						},
					},
					constants.OllamaID: {
						ID:             constants.OllamaID,
						Name:           constants.OllamaDisplayName,
						URL:            constants.OllamaDefaultBaseURL,
						AuthType:       constants.AuthTypeNone,
						SupportsVision: true,
						Endpoints: types.Endpoints{
							Models: constants.OllamaModelsEndpoint,
							Chat:   constants.OllamaChatEndpoint,
						},
					},
					constants.OllamaCloudID: {
						ID:             constants.OllamaCloudID,
						Name:           constants.OllamaCloudDisplayName,
						URL:            constants.OllamaCloudDefaultBaseURL,
						AuthType:       constants.AuthTypeBearer,
						SupportsVision: true,
						Endpoints: types.Endpoints{
							Models: constants.OllamaCloudModelsEndpoint,
							Chat:   constants.OllamaCloudChatEndpoint,
						},
					},
					constants.OpenaiID: {
						ID:             constants.OpenaiID,
						Name:           constants.OpenaiDisplayName,
						URL:            constants.OpenaiDefaultBaseURL,
						AuthType:       constants.AuthTypeBearer,
						SupportsVision: true,
						Endpoints: types.Endpoints{
							Models: constants.OpenaiModelsEndpoint,
							Chat:   constants.OpenaiChatEndpoint,
						},
					},
					constants.DeepseekID: {
						ID:             constants.DeepseekID,
						Name:           constants.DeepseekDisplayName,
						URL:            constants.DeepseekDefaultBaseURL,
						AuthType:       constants.AuthTypeBearer,
						SupportsVision: false,
						Endpoints: types.Endpoints{
							Models: constants.DeepseekModelsEndpoint,
							Chat:   constants.DeepseekChatEndpoint,
						},
					},
					constants.GoogleID: {
						ID:             constants.GoogleID,
						Name:           constants.GoogleDisplayName,
						URL:            constants.GoogleDefaultBaseURL,
						AuthType:       constants.AuthTypeBearer,
						SupportsVision: true,
						Endpoints: types.Endpoints{
							Models: constants.GoogleModelsEndpoint,
							Chat:   constants.GoogleChatEndpoint,
						},
					},
					constants.MistralID: {
						ID:             constants.MistralID,
						Name:           constants.MistralDisplayName,
						URL:            constants.MistralDefaultBaseURL,
						AuthType:       constants.AuthTypeBearer,
						SupportsVision: true,
						Endpoints: types.Endpoints{
							Models: constants.MistralModelsEndpoint,
							Chat:   constants.MistralChatEndpoint,
						},
					},
					constants.MoonshotID: {
						ID:             constants.MoonshotID,
						Name:           constants.MoonshotDisplayName,
						URL:            constants.MoonshotDefaultBaseURL,
						AuthType:       constants.AuthTypeBearer,
						SupportsVision: false,
						Endpoints: types.Endpoints{
							Models: constants.MoonshotModelsEndpoint,
							Chat:   constants.MoonshotChatEndpoint,
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
				Providers: map[types.Provider]*registry.ProviderConfig{
					constants.OllamaID: {
						ID:             constants.OllamaID,
						Name:           constants.OllamaDisplayName,
						URL:            "http://custom-ollama:8080",
						AuthType:       constants.AuthTypeNone,
						SupportsVision: true,
						Endpoints: types.Endpoints{
							Models: constants.OllamaModelsEndpoint,
							Chat:   constants.OllamaChatEndpoint,
						},
					},
					constants.OllamaCloudID: {
						ID:             constants.OllamaCloudID,
						Name:           constants.OllamaCloudDisplayName,
						URL:            constants.OllamaCloudDefaultBaseURL,
						AuthType:       constants.AuthTypeBearer,
						SupportsVision: true,
						Endpoints: types.Endpoints{
							Models: constants.OllamaCloudModelsEndpoint,
							Chat:   constants.OllamaCloudChatEndpoint,
						},
					},
					constants.GroqID: {
						ID:             constants.GroqID,
						Name:           constants.GroqDisplayName,
						URL:            constants.GroqDefaultBaseURL,
						Token:          "groq123",
						AuthType:       constants.AuthTypeBearer,
						SupportsVision: true,
						Endpoints: types.Endpoints{
							Models: constants.GroqModelsEndpoint,
							Chat:   constants.GroqChatEndpoint,
						},
					},
					constants.OpenaiID: {
						ID:             constants.OpenaiID,
						Name:           constants.OpenaiDisplayName,
						URL:            constants.OpenaiDefaultBaseURL,
						Token:          "openai123",
						AuthType:       constants.AuthTypeBearer,
						SupportsVision: true,
						Endpoints: types.Endpoints{
							Models: constants.OpenaiModelsEndpoint,
							Chat:   constants.OpenaiChatEndpoint,
						},
					},
					constants.CloudflareID: {
						ID:             constants.CloudflareID,
						Name:           constants.CloudflareDisplayName,
						URL:            constants.CloudflareDefaultBaseURL,
						AuthType:       constants.AuthTypeBearer,
						SupportsVision: false,
						Endpoints: types.Endpoints{
							Models: constants.CloudflareModelsEndpoint,
							Chat:   constants.CloudflareChatEndpoint,
						},
					},
					constants.CohereID: {
						ID:             constants.CohereID,
						Name:           constants.CohereDisplayName,
						URL:            constants.CohereDefaultBaseURL,
						AuthType:       constants.AuthTypeBearer,
						SupportsVision: true,
						Endpoints: types.Endpoints{
							Models: constants.CohereModelsEndpoint,
							Chat:   constants.CohereChatEndpoint,
						},
					},
					constants.AnthropicID: {
						ID:             constants.AnthropicID,
						Name:           constants.AnthropicDisplayName,
						URL:            constants.AnthropicDefaultBaseURL,
						AuthType:       constants.AuthTypeXheader,
						SupportsVision: true,
						ExtraHeaders: map[string][]string{
							"anthropic-version": {"2023-06-01"},
						},
						Endpoints: types.Endpoints{
							Models: constants.AnthropicModelsEndpoint,
							Chat:   constants.AnthropicChatEndpoint,
						},
					},
					constants.DeepseekID: {
						ID:             constants.DeepseekID,
						Name:           constants.DeepseekDisplayName,
						URL:            constants.DeepseekDefaultBaseURL,
						AuthType:       constants.AuthTypeBearer,
						SupportsVision: false,
						Endpoints: types.Endpoints{
							Models: constants.DeepseekModelsEndpoint,
							Chat:   constants.DeepseekChatEndpoint,
						},
					},
					constants.GoogleID: {
						ID:             constants.GoogleID,
						Name:           constants.GoogleDisplayName,
						URL:            constants.GoogleDefaultBaseURL,
						AuthType:       constants.AuthTypeBearer,
						SupportsVision: true,
						Endpoints: types.Endpoints{
							Models: constants.GoogleModelsEndpoint,
							Chat:   constants.GoogleChatEndpoint,
						},
					},
					constants.MistralID: {
						ID:             constants.MistralID,
						Name:           constants.MistralDisplayName,
						URL:            constants.MistralDefaultBaseURL,
						AuthType:       constants.AuthTypeBearer,
						SupportsVision: true,
						Endpoints: types.Endpoints{
							Models: constants.MistralModelsEndpoint,
							Chat:   constants.MistralChatEndpoint,
						},
					},
					constants.MoonshotID: {
						ID:             constants.MoonshotID,
						Name:           constants.MoonshotDisplayName,
						URL:            constants.MoonshotDefaultBaseURL,
						AuthType:       constants.AuthTypeBearer,
						SupportsVision: false,
						Endpoints: types.Endpoints{
							Models: constants.MoonshotModelsEndpoint,
							Chat:   constants.MoonshotChatEndpoint,
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
				Providers: map[types.Provider]*registry.ProviderConfig{
					constants.OllamaID: {
						ID:             constants.OllamaID,
						Name:           constants.OllamaDisplayName,
						URL:            "http://custom-ollama:8080",
						AuthType:       constants.AuthTypeNone,
						SupportsVision: true,
						Endpoints: types.Endpoints{
							Models: constants.OllamaModelsEndpoint,
							Chat:   constants.OllamaChatEndpoint,
						},
					},
					constants.OllamaCloudID: {
						ID:             constants.OllamaCloudID,
						Name:           constants.OllamaCloudDisplayName,
						URL:            constants.OllamaCloudDefaultBaseURL,
						AuthType:       constants.AuthTypeBearer,
						SupportsVision: true,
						Endpoints: types.Endpoints{
							Models: constants.OllamaCloudModelsEndpoint,
							Chat:   constants.OllamaCloudChatEndpoint,
						},
					},
					constants.GroqID: {
						ID:             constants.GroqID,
						Name:           constants.GroqDisplayName,
						URL:            constants.GroqDefaultBaseURL,
						AuthType:       constants.AuthTypeBearer,
						SupportsVision: true,
						Endpoints: types.Endpoints{
							Models: constants.GroqModelsEndpoint,
							Chat:   constants.GroqChatEndpoint,
						},
					},
					constants.OpenaiID: {
						ID:             constants.OpenaiID,
						Name:           constants.OpenaiDisplayName,
						URL:            constants.OpenaiDefaultBaseURL,
						AuthType:       constants.AuthTypeBearer,
						SupportsVision: true,
						Endpoints: types.Endpoints{
							Models: constants.OpenaiModelsEndpoint,
							Chat:   constants.OpenaiChatEndpoint,
						},
					},
					constants.CloudflareID: {
						ID:             constants.CloudflareID,
						Name:           constants.CloudflareDisplayName,
						URL:            constants.CloudflareDefaultBaseURL,
						AuthType:       constants.AuthTypeBearer,
						SupportsVision: false,
						Endpoints: types.Endpoints{
							Models: constants.CloudflareModelsEndpoint,
							Chat:   constants.CloudflareChatEndpoint,
						},
					},
					constants.CohereID: {
						ID:             constants.CohereID,
						Name:           constants.CohereDisplayName,
						URL:            constants.CohereDefaultBaseURL,
						AuthType:       constants.AuthTypeBearer,
						SupportsVision: true,
						Endpoints: types.Endpoints{
							Models: constants.CohereModelsEndpoint,
							Chat:   constants.CohereChatEndpoint,
						},
					},
					constants.AnthropicID: {
						ID:             constants.AnthropicID,
						Name:           constants.AnthropicDisplayName,
						URL:            constants.AnthropicDefaultBaseURL,
						AuthType:       constants.AuthTypeXheader,
						SupportsVision: true,
						ExtraHeaders: map[string][]string{
							"anthropic-version": {"2023-06-01"},
						},
						Endpoints: types.Endpoints{
							Models: constants.AnthropicModelsEndpoint,
							Chat:   constants.AnthropicChatEndpoint,
						},
					},
					constants.DeepseekID: {
						ID:             constants.DeepseekID,
						Name:           constants.DeepseekDisplayName,
						URL:            constants.DeepseekDefaultBaseURL,
						AuthType:       constants.AuthTypeBearer,
						SupportsVision: false,
						Endpoints: types.Endpoints{
							Models: constants.DeepseekModelsEndpoint,
							Chat:   constants.DeepseekChatEndpoint,
						},
					},
					constants.GoogleID: {
						ID:             constants.GoogleID,
						Name:           constants.GoogleDisplayName,
						URL:            constants.GoogleDefaultBaseURL,
						AuthType:       constants.AuthTypeBearer,
						SupportsVision: true,
						Endpoints: types.Endpoints{
							Models: constants.GoogleModelsEndpoint,
							Chat:   constants.GoogleChatEndpoint,
						},
					},
					constants.MistralID: {
						ID:             constants.MistralID,
						Name:           constants.MistralDisplayName,
						URL:            constants.MistralDefaultBaseURL,
						AuthType:       constants.AuthTypeBearer,
						SupportsVision: true,
						Endpoints: types.Endpoints{
							Models: constants.MistralModelsEndpoint,
							Chat:   constants.MistralChatEndpoint,
						},
					},
					constants.MoonshotID: {
						ID:             constants.MoonshotID,
						Name:           constants.MoonshotDisplayName,
						URL:            constants.MoonshotDefaultBaseURL,
						AuthType:       constants.AuthTypeBearer,
						SupportsVision: false,
						Endpoints: types.Endpoints{
							Models: constants.MoonshotModelsEndpoint,
							Chat:   constants.MoonshotChatEndpoint,
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
