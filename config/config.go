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

	// Providers
	Providers map[string][]string `env:"" description:"The configuration for the providers"`
}

// OIDC holds the configuration for the OIDC provider.
type OIDC struct {
	OIDCIssuerURL    string `env:"ISSUER_URL, default=http://keycloak:8080/realms/inference-gateway-realm" description:"The OIDC issuer URL"`
	OIDCClientID     string `env:"CLIENT_ID, default=inference-gateway-client" type:"secret" description:"The OIDC client ID"`
	OIDCClientSecret string `env:"CLIENT_SECRET" type:"secret" description:"The OIDC client secret"`
}

// ServerConfig holds the configuration for the server.
type ServerConfig struct {
	Host         string        `env:"HOST, default=0.0.0.0" description:"The host address for the server"`
	Port         string        `env:"PORT, default=8080" description:"The port on which the server will listen"`
	ReadTimeout  time.Duration `env:"READ_TIMEOUT, default=30s" description:"The server read timeout"`
	WriteTimeout time.Duration `env:"WRITE_TIMEOUT, default=30s" description:"The server write timeout"`
	IdleTimeout  time.Duration `env:"IDLE_TIMEOUT, default=120s" description:"The server idle timeout"`
	TLSCertPath  string        `env:"TLS_CERT_PATH" description:"The path to the TLS certificate"`
	TLSKeyPath   string        `env:"TLS_KEY_PATH" description:"The path to the TLS key"`
}

// Load loads the configuration from environment variables.
func (cfg *Config) Load(lookuper envconfig.Lookuper) (Config, error) {
	if err := envconfig.ProcessWith(context.Background(), &envconfig.Config{
		Target:   cfg,
		Lookuper: lookuper,
	}); err != nil {
		return Config{}, err
	}

	return *cfg, nil
}
