// Hand-maintained: ElevenLabs' models endpoint is not OpenAI-compatible
// (see .openapi-ignore).
package transformers

import (
	"bytes"
	"encoding/json"

	constants "github.com/inference-gateway/inference-gateway/providers/constants"
	types "github.com/inference-gateway/inference-gateway/providers/types"
)

// elevenlabsModel is one entry of the bare array GET /v1/models returns.
type elevenlabsModel struct {
	ModelID              string `json:"model_id"`
	Name                 string `json:"name"`
	CanDoTextToSpeech    bool   `json:"can_do_text_to_speech"`
	CanDoVoiceConversion bool   `json:"can_do_voice_conversion"`
}

// modalities derives the model's modalities from ElevenLabs' capability
// flags: text-to-speech models take text and return audio, speech-to-speech
// (voice conversion) models take audio and return audio. Models with neither
// flag stay nil so the community table can fill them in.
func (m elevenlabsModel) modalities() *types.ModelModalities {
	switch {
	case m.CanDoTextToSpeech:
		return &types.ModelModalities{Input: []types.Modality{types.ModalityText}, Output: []types.Modality{types.ModalityAudio}}
	case m.CanDoVoiceConversion:
		return &types.ModelModalities{Input: []types.Modality{types.ModalityAudio}, Output: []types.Modality{types.ModalityAudio}}
	}
	return nil
}

// ListModelsResponseElevenlabs decodes ElevenLabs' models listing, a JSON
// array of objects keyed by model_id rather than an OpenAI {object, data}
// envelope. Only speech models are listed there; the sound-effect and video
// models are addressed by id directly and never appear in the listing.
type ListModelsResponseElevenlabs struct {
	Data []elevenlabsModel
}

// UnmarshalJSON accepts the bare array the real API returns and, for
// OpenAI-shaped mocks, an {object, data} envelope whose entries carry `id`.
func (l *ListModelsResponseElevenlabs) UnmarshalJSON(b []byte) error {
	if bytes.HasPrefix(bytes.TrimSpace(b), []byte("[")) {
		return json.Unmarshal(b, &l.Data)
	}
	var envelope struct {
		Data []types.Model `json:"data"`
	}
	if err := json.Unmarshal(b, &envelope); err != nil {
		return err
	}
	l.Data = make([]elevenlabsModel, len(envelope.Data))
	for i, m := range envelope.Data {
		l.Data[i] = elevenlabsModel{ModelID: m.ID}
	}
	return nil
}

// unlistedModels are the sound-effect and video models the gateway can
// route to but ElevenLabs does not include in GET /v1/models. They are
// appended to the listing so clients can discover them and filter on their
// modalities. Kept in step with the video create endpoint's model_id enum:
// https://elevenlabs.io/docs/api-reference/flows/video/create
var unlistedModels = []struct {
	id         string
	modalities types.ModelModalities
}{
	{"eleven_text_to_sound_v2", types.ModelModalities{Input: []types.Modality{types.ModalityText}, Output: []types.Modality{types.ModalityAudio}}},
	{"creatify-aurora", types.ModelModalities{Input: []types.Modality{types.ModalityImage, types.ModalityAudio}, Output: []types.Modality{types.ModalityVideo}}},
	{"veo-3.1-generate-001", types.ModelModalities{Input: []types.Modality{types.ModalityText, types.ModalityImage}, Output: []types.Modality{types.ModalityVideo}}},
	{"veo-3.1-fast-generate-001", types.ModelModalities{Input: []types.Modality{types.ModalityText, types.ModalityImage}, Output: []types.Modality{types.ModalityVideo}}},
	{"bytedance-seedance-v2", types.ModelModalities{Input: []types.Modality{types.ModalityText, types.ModalityImage}, Output: []types.Modality{types.ModalityVideo}}},
	{"bytedance-seedance-v2-fast", types.ModelModalities{Input: []types.Modality{types.ModalityText, types.ModalityImage}, Output: []types.Modality{types.ModalityVideo}}},
	{"bytedance-seedance-v2-mini", types.ModelModalities{Input: []types.Modality{types.ModalityText, types.ModalityImage}, Output: []types.Modality{types.ModalityVideo}}},
	{"bytedance-seedance-v2.5", types.ModelModalities{Input: []types.Modality{types.ModalityText, types.ModalityImage}, Output: []types.Modality{types.ModalityVideo}}},
}

func (l *ListModelsResponseElevenlabs) Transform() types.ListModelsResponse {
	provider := constants.ElevenlabsID
	models := make([]types.Model, 0, len(l.Data)+len(unlistedModels))
	for _, m := range l.Data {
		models = append(models, types.Model{
			ID:         string(provider) + "/" + m.ModelID,
			Object:     "model",
			OwnedBy:    string(provider),
			ServedBy:   provider,
			Modalities: m.modalities(),
		})
	}
	for _, m := range unlistedModels {
		mods := m.modalities
		models = append(models, types.Model{
			ID:         string(provider) + "/" + m.id,
			Object:     "model",
			OwnedBy:    string(provider),
			ServedBy:   provider,
			Modalities: &mods,
		})
	}

	return types.ListModelsResponse{
		Provider: &provider,
		Object:   "list",
		Data:     models,
	}
}
