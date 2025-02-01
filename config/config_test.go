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
				Providers: map[string]*providers.Config{
					providers.AnthropicID: {
						ID:       providers.AnthropicID,
						Name:     providers.AnthropicDisplayName,
						URL:      providers.AnthropicDefaultBaseURL,
						AuthType: providers.AuthTypeXheader,
						ExtraHeaders: map[string][]string{
							"anthropic-version": {"2023-06-01"},
						},
						Endpoints: providers.Endpoints{
							List:     providers.AnthropicListEndpoint,
							Generate: providers.AnthropicGenerateEndpoint,
						},
					},
					providers.CloudflareID: {
						ID:       providers.CloudflareID,
						Name:     providers.CloudflareDisplayName,
						URL:      providers.CloudflareDefaultBaseURL,
						AuthType: providers.AuthTypeBearer,
						Endpoints: providers.Endpoints{
							List:     providers.CloudflareListEndpoint,
							Generate: providers.CloudflareGenerateEndpoint,
						},
					},
					providers.CohereID: {
						ID:       providers.CohereID,
						Name:     providers.CohereDisplayName,
						URL:      providers.CohereDefaultBaseURL,
						AuthType: providers.AuthTypeBearer,
						Endpoints: providers.Endpoints{
							List:     providers.CohereListEndpoint,
							Generate: providers.CohereGenerateEndpoint,
						},
					},
					providers.GroqID: {
						ID:       providers.GroqID,
						Name:     providers.GroqDisplayName,
						URL:      providers.GroqDefaultBaseURL,
						AuthType: providers.AuthTypeBearer,
						Endpoints: providers.Endpoints{
							List:     providers.GroqListEndpoint,
							Generate: providers.GroqGenerateEndpoint,
						},
					},
					providers.OllamaID: {
						ID:       providers.OllamaID,
						Name:     providers.OllamaDisplayName,
						URL:      providers.OllamaDefaultBaseURL,
						AuthType: providers.AuthTypeNone,
						Endpoints: providers.Endpoints{
							List:     providers.OllamaListEndpoint,
							Generate: providers.OllamaGenerateEndpoint,
						},
					},
					providers.OpenaiID: {
						ID:       providers.OpenaiID,
						Name:     providers.OpenaiDisplayName,
						URL:      providers.OpenaiDefaultBaseURL,
						AuthType: providers.AuthTypeBearer,
						Endpoints: providers.Endpoints{
							List:     providers.OpenAIListEndpoint,
							Generate: providers.OpenAIGenerateEndpoint,
						},
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
			},
			expectedCfg: config.Config{
				ApplicationName: "test-app",
				EnableTelemetry: true,
				Environment:     "development",
				EnableAuth:      false,
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
				Providers: map[string]*providers.Config{
					providers.OllamaID: {
						ID:       providers.OllamaID,
						Name:     providers.OllamaDisplayName,
						URL:      "http://custom-ollama:8080",
						AuthType: providers.AuthTypeNone,
						Endpoints: providers.Endpoints{
							List:     providers.OllamaListEndpoint,
							Generate: providers.OllamaGenerateEndpoint,
						},
					},
					providers.GroqID: {
						ID:       providers.GroqID,
						Name:     providers.GroqDisplayName,
						URL:      providers.GroqDefaultBaseURL,
						Token:    "groq123",
						AuthType: providers.AuthTypeBearer,
						Endpoints: providers.Endpoints{
							List:     providers.GroqListEndpoint,
							Generate: providers.GroqGenerateEndpoint,
						},
					},
					providers.OpenaiID: {
						ID:       providers.OpenaiID,
						Name:     providers.OpenaiDisplayName,
						URL:      providers.OpenaiDefaultBaseURL,
						Token:    "openai123",
						AuthType: providers.AuthTypeBearer,
						Endpoints: providers.Endpoints{
							List:     providers.OpenAIListEndpoint,
							Generate: providers.OpenAIGenerateEndpoint,
						},
					},
					providers.CloudflareID: {
						ID:       providers.CloudflareID,
						Name:     providers.CloudflareDisplayName,
						URL:      providers.CloudflareDefaultBaseURL,
						AuthType: providers.AuthTypeBearer,
						Endpoints: providers.Endpoints{
							List:     providers.CloudflareListEndpoint,
							Generate: providers.CloudflareGenerateEndpoint,
						},
					},
					providers.CohereID: {
						ID:       providers.CohereID,
						Name:     providers.CohereDisplayName,
						URL:      providers.CohereDefaultBaseURL,
						AuthType: providers.AuthTypeBearer,
						Endpoints: providers.Endpoints{
							List:     providers.CohereListEndpoint,
							Generate: providers.CohereGenerateEndpoint,
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
							List:     providers.AnthropicListEndpoint,
							Generate: providers.AnthropicGenerateEndpoint,
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
				ApplicationName: "inference-gateway",
				EnableTelemetry: true,
				Environment:     "development",
				EnableAuth:      false,
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
				Providers: map[string]*providers.Config{
					providers.OllamaID: {
						ID:       providers.OllamaID,
						Name:     providers.OllamaDisplayName,
						URL:      "http://custom-ollama:8080",
						AuthType: providers.AuthTypeNone,
						Endpoints: providers.Endpoints{
							List:     providers.OllamaListEndpoint,
							Generate: providers.OllamaGenerateEndpoint,
						},
					},
					providers.GroqID: {
						ID:       providers.GroqID,
						Name:     providers.GroqDisplayName,
						URL:      providers.GroqDefaultBaseURL,
						AuthType: providers.AuthTypeBearer,
						Endpoints: providers.Endpoints{
							List:     providers.GroqListEndpoint,
							Generate: providers.GroqGenerateEndpoint,
						},
					},
					providers.OpenaiID: {
						ID:       providers.OpenaiID,
						Name:     providers.OpenaiDisplayName,
						URL:      providers.OpenaiDefaultBaseURL,
						AuthType: providers.AuthTypeBearer,
						Endpoints: providers.Endpoints{
							List:     providers.OpenAIListEndpoint,
							Generate: providers.OpenAIGenerateEndpoint,
						},
					},
					providers.CloudflareID: {
						ID:       providers.CloudflareID,
						Name:     providers.CloudflareDisplayName,
						URL:      providers.CloudflareDefaultBaseURL,
						AuthType: providers.AuthTypeBearer,
						Endpoints: providers.Endpoints{
							List:     providers.CloudflareListEndpoint,
							Generate: providers.CloudflareGenerateEndpoint,
						},
					},
					providers.CohereID: {
						ID:       providers.CohereID,
						Name:     providers.CohereDisplayName,
						URL:      providers.CohereDefaultBaseURL,
						AuthType: providers.AuthTypeBearer,
						Endpoints: providers.Endpoints{
							List:     providers.CohereListEndpoint,
							Generate: providers.CohereGenerateEndpoint,
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
							List:     providers.AnthropicListEndpoint,
							Generate: providers.AnthropicGenerateEndpoint,
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
