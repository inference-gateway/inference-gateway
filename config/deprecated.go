package config

import (
	"log"

	envconfig "github.com/sethvargo/go-envconfig"
)

// deprecatedEnvAliases maps a current env var name to the deprecated name that
// still works for one release. The schemas repo renamed the boolean toggles
// from `_ENABLE` to `_ENABLED` (see issue #515); these fallbacks keep existing
// deployments working until the next breaking release.
var deprecatedEnvAliases = map[string]string{
	"TELEMETRY_ENABLED":              "TELEMETRY_ENABLE",
	"TELEMETRY_METRICS_PUSH_ENABLED": "TELEMETRY_METRICS_PUSH_ENABLE",
	"TELEMETRY_TRACING_ENABLED":      "TELEMETRY_TRACING_ENABLE",
	"MCP_ENABLED":                    "MCP_ENABLE",
	"MCP_POLLING_ENABLED":            "MCP_POLLING_ENABLE",
	"AUTH_ENABLED":                   "AUTH_ENABLE",
}

// deprecatedAliasLookuper wraps a Lookuper so that when a renamed env var is
// unset, the old `_ENABLE` name is used as a fallback with a deprecation warning.
type deprecatedAliasLookuper struct {
	base envconfig.Lookuper
}

// WithDeprecatedAliases wraps base so deprecated `_ENABLE` env vars still resolve.
func WithDeprecatedAliases(base envconfig.Lookuper) envconfig.Lookuper {
	return &deprecatedAliasLookuper{base: base}
}

func (l *deprecatedAliasLookuper) Lookup(key string) (string, bool) {
	if v, ok := l.base.Lookup(key); ok {
		return v, true
	}
	old, ok := deprecatedEnvAliases[key]
	if !ok {
		return "", false
	}
	if v, ok := l.base.Lookup(old); ok {
		log.Printf("{\"level\":\"warn\",\"caller\":\"config/deprecated.go\",\"msg\":\"env var %s is deprecated, use %s instead\"}", old, key)
		return v, true
	}
	return "", false
}
