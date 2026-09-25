// Code generated from OpenAPI schema. DO NOT EDIT.
package config

import (
	"time"

	client "github.com/inference-gateway/inference-gateway/providers/client"
	registry "github.com/inference-gateway/inference-gateway/providers/registry"
	types "github.com/inference-gateway/inference-gateway/providers/types"
)

// Config holds the configuration for the Inference Gateway
type Config struct {
	// General settings
	Environment               string `env:"ENVIRONMENT, default=production" description:"The environment"`
	AllowedModels             string `env:"ALLOWED_MODELS" description:"Comma-separated list of models to allow. If empty, all models will be available"`
	DisallowedModels          string `env:"DISALLOWED_MODELS" description:"Comma-separated list of models to disallow. If empty, no models will be blocked. Takes lower precedence than ALLOWED_MODELS"`
	VisionEnabled             bool   `env:"VISION_ENABLED, default=false" description:"Enable vision/multimodal handling for all providers. When enabled, image content is stripped from requests to models known to accept only non-image input; unknown models are passed through. When disabled, image content is forwarded to the provider untouched"`
	ImagesEnabled             bool   `env:"IMAGES_ENABLED, default=false" description:"Enable the Images API (POST /v1/images/generations, /v1/images/edits). When disabled, the endpoints return a 404. Only providers with images support (currently openai) can serve these endpoints"`
	AudioEnabled              bool   `env:"AUDIO_ENABLED, default=false" description:"Enable the Audio API (POST /v1/audio/speech, POST /v1/audio/sfx, POST /v1/audio/music). When disabled, the endpoints return a 404. Served by providers with speech, sound-effect or music support (currently openai and elevenlabs) or by the local llama-tts engine"`
	VideosEnabled             bool   `env:"VIDEOS_ENABLED, default=false" description:"Enable the Videos API (POST /v1/videos, GET /v1/videos/{video_id}, GET /v1/videos/{video_id}/content). When disabled, the endpoints return a 404. Only providers with video support (currently elevenlabs) can serve these endpoints"`
	AudioLocalAutoDownload    bool   `env:"AUDIO_LOCAL_AUTO_DOWNLOAD, default=true" description:"Allow downloading the llama-tts binary and GGUF models on first use. Anything already present is never re-downloaded: the cache is checked first, whether populated by an earlier run, the CLI, a mounted volume, or a pre-baked image layer. When false, the gateway serves only from the existing cache or PATH and returns an actionable error when assets are missing"`
	AudioLocalMaxConcurrency  int    `env:"AUDIO_LOCAL_MAX_CONCURRENCY, default=2" description:"Maximum concurrent local speech syntheses; requests beyond the limit queue"`
	AudioLocalTimeout         int    `env:"AUDIO_LOCAL_TIMEOUT, default=300" description:"Timeout in seconds for a single local speech synthesis"`
	DebugContentTruncateWords int    `env:"DEBUG_CONTENT_TRUNCATE_WORDS, default=10" description:"Number of words to truncate per content section in debug logs (development mode only)"`
	DebugMaxMessages          int    `env:"DEBUG_MAX_MESSAGES, default=100" description:"Maximum number of messages to show in debug logs (development mode only)"`
	// Telemetry settings
	Telemetry *TelemetryConfig `env:", prefix=TELEMETRY_" description:"Telemetry configuration"`
	// MCP settings
	MCP *MCPConfig `env:", prefix=MCP_" description:"MCP configuration"`
	// Authentication settings
	Auth *AuthConfig `env:", prefix=AUTH_" description:"Authentication configuration"`
	// Guardrails settings
	Guardrails *GuardrailsConfig `env:", prefix=GUARDRAILS_" description:"Guardrails configuration"`
	// Server settings
	Server *ServerConfig `env:", prefix=SERVER_" description:"Server configuration"`
	// Client settings
	Client *client.ClientConfig `description:"Client configuration"`
	// Routing settings
	Routing *RoutingConfig `env:", prefix=ROUTING_" description:"Routing configuration"`

	// Providers map
	Providers map[types.Provider]*registry.ProviderConfig
}

// Telemetry configuration
type TelemetryConfig struct {
	Enabled             bool   `env:"ENABLED, default=false" description:"Enable telemetry"`
	MetricsPushEnabled  bool   `env:"METRICS_PUSH_ENABLED, default=false" description:"Enable the OTLP metrics push endpoint (POST /v1/metrics)"`
	MetricsPort         string `env:"METRICS_PORT, default=9464" description:"Port for telemetry metrics server"`
	TracingEnabled      bool   `env:"TRACING_ENABLED, default=false" description:"Enable OpenTelemetry tracing spans (requires TELEMETRY_ENABLED)"`
	TracingOtlpEndpoint string `env:"TRACING_OTLP_ENDPOINT, default=http://localhost:4318" description:"OTLP HTTP endpoint for trace export"`
}

// MCP configuration
type MCPConfig struct {
	Enabled                bool          `env:"ENABLED, default=false" description:"Enable MCP"`
	Expose                 bool          `env:"EXPOSE, default=false" description:"Expose the gateway as an MCP server on POST /mcp (requires MCP_ENABLED)"`
	ResourceUrl            string        `env:"RESOURCE_URL" description:"Canonical public URL of POST /mcp, e.g. https://gateway.example.com/mcp. Published as the resource of the OAuth 2.0 Protected Resource Metadata (RFC 9728) document served at /.well-known/oauth-protected-resource/mcp, and pointed at by the resource_metadata of every 401 challenge on /mcp. Defaults to the request scheme (honouring X-Forwarded-Proto) and Host with /mcp appended; set it behind an ingress that rewrites either"`
	Servers                string        `env:"SERVERS" description:"Comma-separated list of MCP servers as alias=url, e.g. deepwiki=https://mcp.deepwiki.com/mcp,http://mcp-time-server:8081/mcp. Without alias= the alias is derived from the URL host. Aliases must match ^[a-z0-9_-]+$, be unique, must not be the reserved alias tools, and namespace the tools as mcp_<alias>_<tool>"`
	ToolMode               string        `env:"TOOL_MODE, default=selector" description:"How MCP tools are exposed to the model. selector injects two meta-tools for discovery and dispatch; direct injects every tool schema"`
	IncludeTools           string        `env:"INCLUDE_TOOLS" description:"Comma-separated list of MCP tool names to inject, matched against either the bare tool name or the namespaced <alias>_<tool name> form. If empty, all tools are injected. Takes precedence over MCP_EXCLUDE_TOOLS"`
	ExcludeTools           string        `env:"EXCLUDE_TOOLS" description:"Comma-separated list of MCP tool names to skip injecting, matched against either the bare tool name or the namespaced <alias>_<tool name> form. If empty, no tools are excluded. Takes lower precedence than MCP_INCLUDE_TOOLS"`
	ClientTimeout          time.Duration `env:"CLIENT_TIMEOUT, default=5s" description:"MCP client HTTP timeout"`
	DialTimeout            time.Duration `env:"DIAL_TIMEOUT, default=3s" description:"MCP client dial timeout"`
	TlsHandshakeTimeout    time.Duration `env:"TLS_HANDSHAKE_TIMEOUT, default=3s" description:"MCP client TLS handshake timeout"`
	ResponseHeaderTimeout  time.Duration `env:"RESPONSE_HEADER_TIMEOUT, default=3s" description:"MCP client response header timeout"`
	ExpectContinueTimeout  time.Duration `env:"EXPECT_CONTINUE_TIMEOUT, default=1s" description:"MCP client expect continue timeout"`
	RequestTimeout         time.Duration `env:"REQUEST_TIMEOUT, default=5s" description:"MCP client request timeout for initialize and tool calls"`
	MaxRetries             int           `env:"MAX_RETRIES, default=3" description:"Maximum number of connection retry attempts"`
	RetryInterval          time.Duration `env:"RETRY_INTERVAL, default=5s" description:"Interval between connection retry attempts"`
	InitialBackoff         time.Duration `env:"INITIAL_BACKOFF, default=1s" description:"Initial backoff duration for exponential backoff retry"`
	EnableReconnect        bool          `env:"ENABLE_RECONNECT, default=true" description:"Enable automatic reconnection for failed servers"`
	ReconnectInterval      time.Duration `env:"RECONNECT_INTERVAL, default=30s" description:"Interval between reconnection attempts"`
	PollingEnabled         bool          `env:"POLLING_ENABLED, default=true" description:"Enable health check polling"`
	PollingInterval        time.Duration `env:"POLLING_INTERVAL, default=30s" description:"Interval between health check polling requests"`
	PollingTimeout         time.Duration `env:"POLLING_TIMEOUT, default=5s" description:"Timeout for individual health check requests"`
	DisableHealthcheckLogs bool          `env:"DISABLE_HEALTHCHECK_LOGS, default=true" description:"Disable health check log messages to reduce noise"`
}

// Authentication configuration
type AuthConfig struct {
	Enabled      bool   `env:"ENABLED, default=false" description:"Enable authentication"`
	OidcIssuer   string `env:"OIDC_ISSUER" description:"OIDC issuer URL. Discovery runs once at startup against {issuer}/.well-known/openid-configuration"`
	OidcClientId string `env:"OIDC_CLIENT_ID" description:"OIDC client ID. Used as the expected token audience when AUTH_OIDC_AUDIENCE is empty"`
	OidcAudience string `env:"OIDC_AUDIENCE" description:"Comma-separated list of accepted aud values, for example an API identifier. Defaults to AUTH_OIDC_CLIENT_ID"`
}

// Guardrails configuration
type GuardrailsConfig struct {
	Enabled         bool          `env:"ENABLED, default=false" description:"Enable gateway guardrails (OPA/Rego policy enforcement)"`
	PolicyDir       string        `env:"POLICY_DIR" description:"Directory of .rego files compiled at startup"`
	FailMode        string        `env:"FAIL_MODE, default=closed" description:"closed or open: behavior on policy/external error or timeout"`
	ExternalUrl     string        `env:"EXTERNAL_URL" description:"Optional external HTTP guardrail service"`
	ExternalTimeout time.Duration `env:"EXTERNAL_TIMEOUT, default=5s" description:"Timeout for the external guardrail service"`
}

// Server configuration
type ServerConfig struct {
	Host               string        `env:"HOST, default=127.0.0.1" description:"Server host"`
	Port               string        `env:"PORT, default=8080" description:"Server port"`
	ReadTimeout        time.Duration `env:"READ_TIMEOUT, default=30s" description:"Read timeout"`
	WriteTimeout       time.Duration `env:"WRITE_TIMEOUT, default=30s" description:"Write timeout"`
	IdleTimeout        time.Duration `env:"IDLE_TIMEOUT, default=120s" description:"Idle timeout"`
	MaxRequestBodySize int           `env:"MAX_REQUEST_BODY_SIZE, default=10485760" description:"Maximum request body size in bytes (10 MiB)"`
	TlsCertPath        string        `env:"TLS_CERT_PATH" description:"TLS certificate path"`
	TlsKeyPath         string        `env:"TLS_KEY_PATH" description:"TLS key path"`
}

// Routing configuration
type RoutingConfig struct {
	Enabled    bool   `env:"ENABLED, default=false" description:"Enable gateway-native model routing: logical model aliases backed by a pool of upstream provider deployments, selected round-robin per replica. Opt-in; when disabled, direct provider/model routing is unchanged"`
	ConfigPath string `env:"CONFIG_PATH" description:"Path to a YAML file mapping logical model aliases to their upstream deployment pools. Required when ROUTING_ENABLED is true"`
}
