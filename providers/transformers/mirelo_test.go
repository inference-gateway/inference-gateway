package transformers

import (
	"encoding/json"
	"testing"

	require "github.com/stretchr/testify/require"

	types "github.com/inference-gateway/inference-gateway/providers/types"
)

func TestListModelsResponseMirelo_ListsTextToAudioModels(t *testing.T) {
	var l ListModelsResponseMirelo
	require.NoError(t, json.Unmarshal([]byte(`{"data":[
		{"id":"sfx-2.0","status":"general_availability","credits_per_second":15},
		{"id":"sfx-1.6","status":"general_availability","credits_per_second":10}
	]}`), &l))

	resp := l.Transform()

	require.Equal(t, listObject, resp.Object)
	require.Len(t, resp.Data, 2)
	require.Equal(t, "mirelo/sfx-2.0", resp.Data[0].ID)
	require.Equal(t, "mirelo/sfx-1.6", resp.Data[1].ID)
	for _, m := range resp.Data {
		require.Equal(t, []types.Modality{types.ModalityText}, m.Modalities.Input)
		require.Equal(t, []types.Modality{types.ModalityAudio}, m.Modalities.Output)
	}
}
