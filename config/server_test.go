package config_test

import (
	"testing"

	assert "github.com/stretchr/testify/assert"

	config "github.com/inference-gateway/inference-gateway/config"
)

// TestResolveMaxRequestBodySize verifies the configured limit is honored and the
// default kicks in for unset/non-positive values (e.g. a config built without env parsing).
func TestResolveMaxRequestBodySize(t *testing.T) {
	const defaultSize = 10 << 20 // 10 MiB
	assert.Equal(t, defaultSize, (&config.ServerConfig{MaxRequestBodySize: 0}).ResolveMaxRequestBodySize(), "unset falls back to default")
	assert.Equal(t, defaultSize, (&config.ServerConfig{MaxRequestBodySize: -1}).ResolveMaxRequestBodySize(), "negative falls back to default")
	assert.Equal(t, 1234, (&config.ServerConfig{MaxRequestBodySize: 1234}).ResolveMaxRequestBodySize(), "configured value is honored")
	assert.Equal(t, defaultSize, (*config.ServerConfig)(nil).ResolveMaxRequestBodySize(), "nil receiver falls back to default")
}
