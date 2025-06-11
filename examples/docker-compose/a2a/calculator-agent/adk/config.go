package adk

import "time"

// Config holds all application configuration
type Config struct {
	AgentName                     string              `env:"AGENT_NAME,default=helloworld-agent"`
	AgentDescription              string              `env:"AGENT_DESCRIPTION,default=A simple greeting agent that provides personalized greetings using the A2A protocol"`
	AgentURL                      string              `env:"AGENT_URL,default=http://helloworld-agent:8080"`
	AgentVersion                  string              `env:"AGENT_VERSION,default=1.0.0"`
	Debug                         bool                `env:"DEBUG,default=false"`
	Port                          string              `env:"PORT,default=8080"`
	InferenceGatewayURL           string              `env:"INFERENCE_GATEWAY_URL,required"`
	LLMProvider                   string              `env:"LLM_PROVIDER,default=deepseek"`
	LLMModel                      string              `env:"LLM_MODEL,default=deepseek-chat"`
	MaxChatCompletionIterations   int                 `env:"MAX_CHAT_COMPLETION_ITERATIONS,default=10"`
	StreamingStatusUpdateInterval time.Duration       `env:"STREAMING_STATUS_UPDATE_INTERVAL,default=1s"`
	CapabilitiesConfig            *CapabilitiesConfig `env:",prefix=CAPABILITIES_"`
	TLSConfig                     *TLSConfig          `env:",prefix=TLS_"`
	AuthConfig                    *AuthConfig         `env:",prefix=AUTH_"`
	QueueConfig                   *QueueConfig        `env:",prefix=QUEUE_"`
	ServerConfig                  *ServerConfig       `env:",prefix=SERVER_"`
}

// CapabilitiesConfig defines agent capabilities
type CapabilitiesConfig struct {
	Streaming              bool `env:"STREAMING,default=true" description:"Enable streaming support"`
	PushNotifications      bool `env:"PUSH_NOTIFICATIONS,default=true" description:"Enable push notifications"`
	StateTransitionHistory bool `env:"STATE_TRANSITION_HISTORY,default=false" description:"Enable state transition history"`
}

// TLSConfig holds TLS configuration
type TLSConfig struct {
	Enable   bool   `env:"ENABLE,default=false"`
	CertPath string `env:"CERT_PATH,default=" description:"TLS certificate path"`
	KeyPath  string `env:"KEY_PATH,default=" description:"TLS key path"`
}

// AuthConfig holds authentication configuration
type AuthConfig struct {
	Enable       bool   `env:"ENABLE,default=false"`
	IssuerURL    string `env:"ISSUER_URL,default=http://keycloak:8080/realms/inference-gateway-realm"`
	ClientID     string `env:"CLIENT_ID,default=inference-gateway-client"`
	ClientSecret string `env:"CLIENT_SECRET"`
}

// QueueConfig holds task queue configuration
type QueueConfig struct {
	MaxSize         int           `env:"MAX_SIZE,default=100"`
	CleanupInterval time.Duration `env:"CLEANUP_INTERVAL,default=30s"`
}

// ServerConfig holds HTTP server configuration
type ServerConfig struct {
	ReadTimeout  time.Duration `env:"READ_TIMEOUT,default=120s" description:"HTTP server read timeout"`
	WriteTimeout time.Duration `env:"WRITE_TIMEOUT,default=120s" description:"HTTP server write timeout"`
	IdleTimeout  time.Duration `env:"IDLE_TIMEOUT,default=120s" description:"HTTP server idle timeout"`
}
