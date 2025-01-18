package config

import (
	"context"
	"time"

	"github.com/sethvargo/go-envconfig"
)

// Config holds the configuration for the Inference Gateway.
//
//go:generate go run ../cmd/generate/main.go -type=Env -output=../examples/docker-compose/.env.example
//go:generate go run ../cmd/generate/main.go -type=ConfigMap -output=../examples/kubernetes/basic/inference-gateway/configmap.yaml
//go:generate go run ../cmd/generate/main.go -type=Secret -output=../examples/kubernetes/basic/inference-gateway/secret.yaml
//go:generate go run ../cmd/generate/main.go -type=ConfigMap -output=../examples/kubernetes/hybrid/inference-gateway/configmap.yaml
//go:generate go run ../cmd/generate/main.go -type=Secret -output=../examples/kubernetes/hybrid/inference-gateway/secret.yaml
//go:generate go run ../cmd/generate/main.go -type=ConfigMap -output=../examples/kubernetes/authentication/inference-gateway/configmap.yaml
//go:generate go run ../cmd/generate/main.go -type=Secret -output=../examples/kubernetes/authentication/inference-gateway/secret.yaml
//go:generate go run ../cmd/generate/main.go -type=ConfigMap -output=../examples/kubernetes/agent/inference-gateway/configmap.yaml
//go:generate go run ../cmd/generate/main.go -type=Secret -output=../examples/kubernetes/agent/inference-gateway/secret.yaml
//go:generate go run ../cmd/generate/main.go -type=MD -output=../Configurations.md
type Config struct {
	// General settings
	ApplicationName  string `env:"APPLICATION_NAME, default=inference-gateway" description:"The name of the application"`
	EnableTelemetry  bool   `env:"ENABLE_TELEMETRY, default=false" description:"Enable telemetry for the server"`
	Environment      string `env:"ENVIRONMENT, default=production" description:"The environment in which the application is running"`
	EnableAuth       bool   `env:"ENABLE_AUTH, default=false" description:"Enable authentication"`
	OIDCIssuerURL    string `env:"OIDC_ISSUER_URL, default=http://keycloak:8080/realms/inference-gateway-realm" description:"The OIDC issuer URL"`
	OIDCClientID     string `env:"OIDC_CLIENT_ID, default=inference-gateway-client" type:"secret" description:"The OIDC client ID"`
	OIDCClientSecret string `env:"OIDC_CLIENT_SECRET" type:"secret" description:"The OIDC client secret"`

	// Server settings
	ServerHost         string        `env:"SERVER_HOST, default=0.0.0.0" description:"The host address for the server"`
	ServerPort         string        `env:"SERVER_PORT, default=8080" description:"The port on which the server will listen"`
	ServerReadTimeout  time.Duration `env:"SERVER_READ_TIMEOUT, default=30s" description:"The server read timeout"`
	ServerWriteTimeout time.Duration `env:"SERVER_WRITE_TIMEOUT, default=30s" description:"The server write timeout"`
	ServerIdleTimeout  time.Duration `env:"SERVER_IDLE_TIMEOUT, default=120s" description:"The server idle timeout"`
	ServerTLSCertPath  string        `env:"SERVER_TLS_CERT_PATH" description:"The path to the TLS certificate"`
	ServerTLSKeyPath   string        `env:"SERVER_TLS_KEY_PATH" description:"The path to the TLS key"`

	// API URLs and keys
	OllamaAPIURL      string `env:"OLLAMA_API_URL, default=http://ollama:8080" description:"The URL for Ollama API"`
	GroqAPIURL        string `env:"GROQ_API_URL, default=https://api.groq.com" description:"The URL for Groq Cloud API"`
	GroqAPIKey        string `env:"GROQ_API_KEY" type:"secret" description:"The Access token for Groq Cloud API"`
	OpenaiAPIURL      string `env:"OPENAI_API_URL, default=https://api.openai.com" description:"The URL for OpenAI API"`
	OpenaiAPIKey      string `env:"OPENAI_API_KEY" type:"secret" description:"The Access token for OpenAI API"`
	GoogleAIStudioURL string `env:"GOOGLE_AISTUDIO_API_URL, default=https://generativelanguage.googleapis.com" description:"The URL for Google AI Studio API"`
	GoogleAIStudioKey string `env:"GOOGLE_AISTUDIO_API_KEY" type:"secret" description:"The Access token for Google AI Studio API"`
	CloudflareAPIURL  string `env:"CLOUDFLARE_API_URL, default=https://api.cloudflare.com/client/v4/accounts/{ACCOUNT_ID}" description:"The URL for Cloudflare API"`
	CloudflareAPIKey  string `env:"CLOUDFLARE_API_KEY" type:"secret" description:"The Access token for Cloudflare API"`
	CohereAPIURL      string `env:"COHERE_API_URL, default=https://api.cohere.com" description:"The URL for Cohere API"`
	CohereAPIKey      string `env:"COHERE_API_KEY" type:"secret" description:"The Access token for Cohere API"`
}

// Load loads the configuration from environment variables.
func (cfg *Config) Load() (Config, error) {
	if err := envconfig.Process(context.Background(), cfg); err != nil {
		return Config{}, err
	}
	return *cfg, nil
}
