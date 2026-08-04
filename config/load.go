package config

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	envconfig "github.com/sethvargo/go-envconfig"

	constants "github.com/inference-gateway/inference-gateway/providers/constants"
	registry "github.com/inference-gateway/inference-gateway/providers/registry"
	types "github.com/inference-gateway/inference-gateway/providers/types"
)

// Load configuration
func (cfg *Config) Load(lookuper envconfig.Lookuper) (Config, error) {
	if err := envconfig.ProcessWith(context.Background(), &envconfig.Config{
		Target:   cfg,
		Lookuper: lookuper,
	}); err != nil {
		return Config{}, err
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

// The string representation of Config
func (cfg *Config) String() string {
	return fmt.Sprintf(
		"Config{ApplicationName:%s, Version:%s Environment:%s, Telemetry:%+v, "+
			"MCP:%+v, Auth:%+v, Server:%+v, Routing:%+v, Client:%+v, Providers:%+v}",
		APPLICATION_NAME,
		VERSION,
		cfg.Environment,
		cfg.Telemetry,
		cfg.MCP,
		cfg.Auth,
		cfg.Server,
		cfg.Routing,
		cfg.Client,
		cfg.Providers,
	)
}
