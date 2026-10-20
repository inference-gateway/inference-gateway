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
	assert.Contains(t, content, "AuthType: constants.AuthTypeNone")
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
	assert.Contains(t, content, "Providers map[types.Provider]*registry.ProviderConfig")
	assert.NotContains(t, content, "func (cfg *Config) Load")
	assert.NotContains(t, content, "acme")
	assert.NotContains(t, content, "ollama")
}

func TestGenerateProvidersRemovesStaleTransformer(t *testing.T) {
	dir := t.TempDir()
	stale := filepath.Join(dir, "gone.go")
	handwritten := filepath.Join(dir, "custom.go")
	require.NoError(t, os.WriteFile(stale, []byte("// Code generated from OpenAPI schema. DO NOT EDIT.\npackage transformers\n"), 0o644))
	require.NoError(t, os.WriteFile(handwritten, []byte("package transformers\n"), 0o644))

	require.NoError(t, GenerateProviders(dir, fixture))

	_, err := os.Stat(stale)
	assert.True(t, os.IsNotExist(err), "stale generated transformer should be removed")
	_, err = os.Stat(handwritten)
	assert.NoError(t, err, "hand-written file without the generated header must be kept")
}
