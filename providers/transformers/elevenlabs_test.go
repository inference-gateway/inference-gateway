package transformers

import (
	"encoding/json"
	"testing"

	require "github.com/stretchr/testify/require"

	types "github.com/inference-gateway/inference-gateway/providers/types"
)

func TestListModelsResponseElevenlabs_DecodesBareArray(t *testing.T) {
	var l ListModelsResponseElevenlabs
	require.NoError(t, json.Unmarshal([]byte(`[{"model_id":"eleven_v3","name":"Eleven v3","can_do_text_to_speech":true}]`), &l))
	resp := l.Transform()
	require.Len(t, resp.Data, 1+len(unlistedModels))
	require.Equal(t, "elevenlabs/eleven_v3", resp.Data[0].ID)
	require.Equal(t, "list", resp.Object)
	require.Equal(t, []types.Modality{types.ModalityText}, resp.Data[0].Modalities.Input)
	require.Equal(t, []types.Modality{types.ModalityAudio}, resp.Data[0].Modalities.Output)
	require.Equal(t, "elevenlabs/music_v2", resp.Data[2].ID)
	require.Equal(t, []types.Modality{types.ModalityAudio}, resp.Data[2].Modalities.Output)
	require.Equal(t, "elevenlabs/creatify-aurora", resp.Data[4].ID)
	require.Equal(t, []types.Modality{types.ModalityVideo}, resp.Data[4].Modalities.Output)
}
