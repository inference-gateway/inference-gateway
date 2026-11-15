package gen

import (
	"os"
	"path/filepath"
	"testing"

	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

func TestGenerateEnvExampleOverridesServerHost(t *testing.T) {
	dest := filepath.Join(t.TempDir(), ".env.example")
	require.NoError(t, EnvExample(dest, fixture))

	out, err := os.ReadFile(dest)
	require.NoError(t, err)
	content := string(out)

	assert.Contains(t, content, serverHostEnv+"="+serverHostInDocker)
	assert.NotContains(t, content, serverHostEnv+"=127.0.0.1")
	assert.Contains(t, content, "CLIENT_TIMEOUT=30s")
}
