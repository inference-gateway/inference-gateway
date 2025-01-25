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
	ApplicationName string `env:"APPLICATION_NAME, default=inference-gateway" description:"The name of the application"`
	Environment     string `env:"ENVIRONMENT, default=production" description:"The environment in which the application is running"`
	EnableTelemetry bool   `env:"ENABLE_TELEMETRY, default=false" description:"Enable telemetry for the server"`
	EnableAuth      bool   `env:"ENABLE_AUTH, default=false" description:"Enable authentication"`

	// Auth settings
	OIDC *OIDC `env:", prefix=OIDC_" description:"The OIDC configuration"`

	// Server settings
	Server *ServerConfig `env:", prefix=SERVER_" description:"The configuration for the server"`

	// Providers settings
	Anthropic  *AnthropicConfig  `env:", prefix=ANTHROPIC_" id:"anthropic" name:"Anthropic" url:"https://api.anthropic.com" auth_type:"xheader"`
	Cloudflare *CloudflareConfig `env:", prefix=CLOUDFLARE_" id:"cloudflare" name:"Cloudflare" url:"https://api.cloudflare.com/client/v4/accounts/{ACCOUNT_ID}" auth_type:"bearer"`
	Cohere     *CohereConfig     `env:", prefix=COHERE_" id:"cohere" name:"Cohere" url:"https://api.cohere.com" auth_type:"bearer"`
	Google     *GoogleConfig     `env:", prefix=GOOGLE_" id:"google" name:"Google" url:"https://generativelanguage.googleapis.com" auth_type:"query"`
	Groq       *GroqConfig       `env:", prefix=GROQ_" id:"groq" name:"Groq" url:"https://api.groq.com" auth_type:"bearer"`
	Ollama     *OllamaConfig     `env:", prefix=OLLAMA_" id:"ollama" name:"Ollama" url:"http://ollama:8080" auth_type:"none"`
	Openai     *OpenaiConfig     `env:", prefix=OPENAI_" id:"openai" name:"Openai" url:"https://api.openai.com" auth_type:"bearer"`
}

// OIDC holds the configuration for the OIDC provider
type OIDC struct {
	IssuerURL    string `env:"ISSUER_URL, default=http://keycloak:8080/realms/inference-gateway-realm" description:"The OIDC issuer URL"`
	ClientID     string `env:"CLIENT_ID, default=inference-gateway-client" type:"secret" description:"The OIDC client ID"`
	ClientSecret string `env:"CLIENT_SECRET" type:"secret" description:"The OIDC client secret"`
}

// ServerConfig holds the configuration for the server
type ServerConfig struct {
	Host         string        `env:"HOST, default=0.0.0.0" description:"The host address for the server"`
	Port         string        `env:"PORT, default=8080" description:"The port on which the server will listen"`
	ReadTimeout  time.Duration `env:"READ_TIMEOUT, default=30s" description:"The server read timeout"`
	WriteTimeout time.Duration `env:"WRITE_TIMEOUT, default=30s" description:"The server write timeout"`
	IdleTimeout  time.Duration `env:"IDLE_TIMEOUT, default=120s" description:"The server idle timeout"`
	TLSCertPath  string        `env:"TLS_CERT_PATH" description:"The path to the TLS certificate"`
	TLSKeyPath   string        `env:"TLS_KEY_PATH" description:"The path to the TLS key"`
}

// AnthropicConfig holds the specific provider config
type AnthropicConfig struct {
	ID           string              `env:"ID, default=anthropic" description:"The provider ID"`
	Name         string              `env:"NAME, default=Anthropic" description:"The provider name"`
	URL          string              `env:"API_URL, default=https://api.anthropic.com" description:"The provider API URL"`
	Token        string              `env:"API_KEY" type:"secret" description:"The provider API key"`
	AuthType     string              `env:"AUTH_TYPE, default=xheader" description:"The provider auth type"`
	ExtraHeaders map[string][]string `env:"EXTRA_HEADERS, default=anthropic-version:2023-06-01" description:"Extra headers for provider requests"`
	Endpoints    struct {
		List     string
		Generate string
	}
}

// CloudflareConfig holds the specific provider config
type CloudflareConfig struct {
	ID        string `env:"ID, default=cloudflare" description:"The provider ID"`
	Name      string `env:"NAME, default=Cloudflare" description:"The provider name"`
	URL       string `env:"API_URL, default=https://api.cloudflare.com/client/v4/accounts/{ACCOUNT_ID}" description:"The provider API URL"`
	Token     string `env:"API_KEY" type:"secret" description:"The provider API key"`
	AuthType  string `env:"AUTH_TYPE, default=bearer" description:"The provider auth type"`
	Endpoints struct {
		List     string
		Generate string
	}
}

// CohereConfig holds the specific provider config
type CohereConfig struct {
	ID        string `env:"ID, default=cohere" description:"The provider ID"`
	Name      string `env:"NAME, default=Cohere" description:"The provider name"`
	URL       string `env:"API_URL, default=https://api.cohere.com" description:"The provider API URL"`
	Token     string `env:"API_KEY" type:"secret" description:"The provider API key"`
	AuthType  string `env:"AUTH_TYPE, default=bearer" description:"The provider auth type"`
	Endpoints struct {
		List     string
		Generate string
	}
}

// GoogleConfig holds the specific provider config
type GoogleConfig struct {
	ID        string `env:"ID, default=google" description:"The provider ID"`
	Name      string `env:"NAME, default=Google" description:"The provider name"`
	URL       string `env:"API_URL, default=https://generativelanguage.googleapis.com" description:"The provider API URL"`
	Token     string `env:"API_KEY" type:"secret" description:"The provider API key"`
	AuthType  string `env:"AUTH_TYPE, default=query" description:"The provider auth type"`
	Endpoints struct {
		List     string
		Generate string
	}
}

// GroqConfig holds the specific provider config
type GroqConfig struct {
	ID        string `env:"ID, default=groq" description:"The provider ID"`
	Name      string `env:"NAME, default=Groq" description:"The provider name"`
	URL       string `env:"API_URL, default=https://api.groq.com" description:"The provider API URL"`
	Token     string `env:"API_KEY" type:"secret" description:"The provider API key"`
	AuthType  string `env:"AUTH_TYPE, default=bearer" description:"The provider auth type"`
	Endpoints struct {
		List     string
		Generate string
	}
}

// OllamaConfig holds the specific provider config
type OllamaConfig struct {
	ID        string `env:"ID, default=ollama" description:"The provider ID"`
	Name      string `env:"NAME, default=Ollama" description:"The provider name"`
	URL       string `env:"API_URL, default=http://ollama:8080" description:"The provider API URL"`
	Token     string `env:"API_KEY" type:"secret" description:"The provider API key"`
	AuthType  string `env:"AUTH_TYPE, default=none" description:"The provider auth type"`
	Endpoints struct {
		List     string
		Generate string
	}
}

// OpenaiConfig holds the specific provider config
type OpenaiConfig struct {
	ID        string `env:"ID, default=openai" description:"The provider ID"`
	Name      string `env:"NAME, default=Openai" description:"The provider name"`
	URL       string `env:"API_URL, default=https://api.openai.com" description:"The provider API URL"`
	Token     string `env:"API_KEY" type:"secret" description:"The provider API key"`
	AuthType  string `env:"AUTH_TYPE, default=bearer" description:"The provider auth type"`
	Endpoints struct {
		List     string
		Generate string
	}
}

// Load loads the configuration from environment variables
func (cfg *Config) Load(lookuper envconfig.Lookuper) (Config, error) {
	if err := envconfig.ProcessWith(context.Background(), &envconfig.Config{
		Target:   cfg,
		Lookuper: lookuper,
	}); err != nil {
		return Config{}, err
	}
	return *cfg, nil
}
