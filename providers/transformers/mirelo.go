package transformers

import (
	constants "github.com/inference-gateway/inference-gateway/providers/constants"
	types "github.com/inference-gateway/inference-gateway/providers/types"
)

const listObject = "list"

// ListModelsResponseMirelo decodes Mirelo's GET /v3/models, a {data} envelope
// with no `object` whose entries describe sound-effect releases. Hand-maintained
// (see .openapi-ignore) because the listing publishes no modalities.
type ListModelsResponseMirelo struct {
	modelListing
}

// Transform lists every Mirelo release as a text-to-audio model, which is what
// POST /v1/audio/sfx can ask of it.
func (l *ListModelsResponseMirelo) Transform() types.ListModelsResponse {
	provider := constants.MireloID
	for i := range l.Data {
		l.Data[i].Modalities = &types.ModelModalities{
			Input:  []types.Modality{types.ModalityText},
			Output: []types.Modality{types.ModalityAudio},
		}
	}
	return types.ListModelsResponse{
		Provider: &provider,
		Object:   listObject,
		Data:     l.models(provider),
	}
}
