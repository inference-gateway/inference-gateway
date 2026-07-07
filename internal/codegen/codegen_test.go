package codegen

import (
	"os"
	"path/filepath"
	"testing"

	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

const fixture = "testdata/fixture-openapi.yaml"

func TestGenerateProviderRegistryFromFixture(t *testing.T) {
	dest := filepath.Join(t.TempDir(), "registry.go")
	require.NoError(t, GenerateProviderRegistry(dest, fixture))

	out, err := os.ReadFile(dest)
	require.NoError(t, err)
	content := string(out)

	assert.Contains(t, content, "constants.AcmeID: {")
	assert.Contains(t, content, "constants.LocalID: {")
	assert.Contains(t, content, `"x-acme-version": {"v1"}`)
	assert.Contains(t, content, "AuthType:       constants.AuthTypeNone")
	assert.NotContains(t, content, "anthropic")
}

func TestGenerateProvidersFromFixture(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, GenerateProviders(dir, fixture))

	for _, file := range []string{"acme.go", "local.go", "transformers.go"} {
		_, err := os.Stat(filepath.Join(dir, file))
		assert.NoError(t, err, file)
	}

	out, err := os.ReadFile(filepath.Join(dir, "transformers.go"))
	require.NoError(t, err)
	content := string(out)

	assert.Contains(t, content, "case constants.AcmeID:")
	assert.Contains(t, content, "return &ListModelsResponseAcme{}")
	assert.Contains(t, content, "case constants.LocalID:")
	assert.Contains(t, content, "return &ListModelsResponseOpenai{}")
}

func TestGenerateConfigFromFixture(t *testing.T) {
	dest := filepath.Join(t.TempDir(), "config.go")
	require.NoError(t, GenerateConfig(dest, fixture))

	out, err := os.ReadFile(dest)
	require.NoError(t, err)
	content := string(out)

	assert.Contains(t, content, "Client *client.ClientConfig")
	assert.Contains(t, content, "defaults.AuthType != constants.AuthTypeNone")
	assert.Contains(t, content, "cp := *defaults")
	assert.NotContains(t, content, "acme")
	assert.NotContains(t, content, "ollama")
}
