package config

import (
	"context"
	"fmt"
	"log"
	"net/url"
	"strconv"
	"strings"
	"time"

	envconfig "github.com/sethvargo/go-envconfig"

	endpoints "github.com/inference-gateway/inference-gateway/internal/platform/endpoints"
	constants "github.com/inference-gateway/inference-gateway/providers/constants"
	registry "github.com/inference-gateway/inference-gateway/providers/registry"
	types "github.com/inference-gateway/inference-gateway/providers/types"
)

const (
	// EnvImagesEnabled is the current environment variable for the Images API toggle.
	EnvImagesEnabled = "IMAGES_ENABLED"
	// EnvImagesEnabledDeprecated is the retired name for EnvImagesEnabled. It is
	// still honoured when EnvImagesEnabled is unset; remove in the next major release.
	EnvImagesEnabledDeprecated = "ENABLE_IMAGES"
	// EnvA2AResourceURL and EnvMCPResourceURL are served without authentication,
	// on the gateway agent card and the RFC 9728 documents.
	EnvA2AResourceURL = "A2A_RESOURCE_URL"
	EnvMCPResourceURL = "MCP_RESOURCE_URL"
)

// Load configuration
func (cfg *Config) Load(lookuper envconfig.Lookuper) (Config, error) {
	if err := envconfig.ProcessWith(context.Background(), &envconfig.Config{
		Target:   cfg,
		Lookuper: lookuper,
	}); err != nil {
		return Config{}, err
	}

	if _, set := lookuper.Lookup(EnvImagesEnabled); !set {
		if legacy, ok := lookuper.Lookup(EnvImagesEnabledDeprecated); ok {
			enabled, err := strconv.ParseBool(legacy)
			if err != nil {
				return Config{}, fmt.Errorf("parsing %s: %w", EnvImagesEnabledDeprecated, err)
			}
			cfg.ImagesEnabled = enabled
			t := time.Now().UTC().Format(time.RFC3339)
			log.SetFlags(0)
			log.Printf("{\"level\":\"warn\",\"timestamp\":\"%s\",\"caller\":\"config/load.go\",\"msg\":\"%s is deprecated, use %s instead\"}", t, EnvImagesEnabledDeprecated, EnvImagesEnabled)
		}
	}

	if cfg.A2A != nil {
		if err := rejectCredentials(EnvA2AResourceURL, cfg.A2A.ResourceUrl); err != nil {
			return Config{}, err
		}
	}
	if cfg.MCP != nil {
		if err := rejectCredentials(EnvMCPResourceURL, cfg.MCP.ResourceUrl); err != nil {
			return Config{}, err
		}
	}

	if cfg.Providers == nil {
		cfg.Providers = make(map[types.Provider]*registry.ProviderConfig)
	}

	for id, defaults := range registry.Registry {
		if _, exists := cfg.Providers[id]; !exists {
			cp := *defaults
			providerCfg := &cp
			url, ok := lookuper.Lookup(strings.ToUpper(string(id)) + "_API_URL")
			if ok {
				providerCfg.URL = url
			}

			token, ok := lookuper.Lookup(strings.ToUpper(string(id)) + "_API_KEY")
			if (!ok || token == "") && defaults.AuthType != constants.AuthTypeNone {
				t := time.Now().UTC().Format(time.RFC3339)
				log.SetFlags(0)
				log.Printf("{\"level\":\"notice\",\"timestamp\":\"%s\",\"caller\":\"config/load.go\",\"msg\":\"provider is not configured\",\"provider\":\"%s\"}", t, string(id))
			}
			providerCfg.Token = token
			cfg.Providers[id] = providerCfg
		}
	}

	return *cfg, nil
}

// rejectCredentials refuses a published URL that carries credentials. The
// error never echoes the URL, since that would log the credential it guards.
func rejectCredentials(env, rawURL string) error {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("%s is not a valid url", env)
	}
	if parsed.User != nil {
		return fmt.Errorf("%s must not carry credentials: it is published without authentication", env)
	}
	return nil
}

// String renders the MCP config with the passwords in MCP_SERVERS masked, so
// logging the config never prints a server credential.
func (cfg MCPConfig) String() string {
	type plain MCPConfig
	cfg.Servers = endpoints.Redact(cfg.Servers)
	return fmt.Sprintf("%+v", plain(cfg))
}

// String renders the A2A config with the passwords in A2A_AGENTS masked, so
// logging the config never prints an agent credential.
func (cfg A2AConfig) String() string {
	type plain A2AConfig
	cfg.Agents = endpoints.Redact(cfg.Agents)
	return fmt.Sprintf("%+v", plain(cfg))
}

// The string representation of Config
func (cfg *Config) String() string {
	return fmt.Sprintf(
		"Config{ApplicationName:%s, Version:%s Environment:%s, Telemetry:%+v, "+
			"MCP:%+v, A2A:%+v, Auth:%+v, Server:%+v, Routing:%+v, Client:%+v, Providers:%+v}",
		APPLICATION_NAME,
		VERSION,
		cfg.Environment,
		cfg.Telemetry,
		cfg.MCP,
		cfg.A2A,
		cfg.Auth,
		cfg.Server,
		cfg.Routing,
		cfg.Client,
		cfg.Providers,
	)
}
