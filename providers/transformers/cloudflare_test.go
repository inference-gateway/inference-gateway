package transformers

import (
	"encoding/json"
	"testing"

	require "github.com/stretchr/testify/require"
)

func TestListModelsResponseCloudflare_DecodesNativeEnvelope(t *testing.T) {
	var l ListModelsResponseCloudflare
	require.NoError(t, json.Unmarshal([]byte(`{"success":true,"result":[{"name":"cf-public-magicoder","created_at":"2025-09-01 12:34:56.789"},{"name":"cf-public-cnn-summarization","created_at":"2025-09-01T12:34:56Z"}]}`), &l))

	resp := l.Transform()
	require.Len(t, resp.Data, 2)
	require.Equal(t, "cloudflare/cf-public-magicoder", resp.Data[0].ID)
	require.Equal(t, int64(1756730096), resp.Data[0].Created)
	require.Equal(t, "cloudflare/cf-public-cnn-summarization", resp.Data[1].ID)
	require.Equal(t, int64(1756730096), resp.Data[1].Created)
}

func TestListModelsResponseCloudflare_AcceptsOpenAIShape(t *testing.T) {
	var l ListModelsResponseCloudflare
	require.NoError(t, json.Unmarshal([]byte(`{"object":"list","data":[{"id":"m1"}]}`), &l))

	resp := l.Transform()
	require.Len(t, resp.Data, 1)
	require.Equal(t, "cloudflare/m1", resp.Data[0].ID)
	require.Equal(t, "list", resp.Object)
}
