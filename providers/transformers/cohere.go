// Hand-maintained: Cohere's models endpoint is not OpenAI-compatible
// (see .openapi-ignore).
package transformers

import (
	"encoding/json"

	constants "github.com/inference-gateway/inference-gateway/providers/constants"
	types "github.com/inference-gateway/inference-gateway/providers/types"
)

// cohereModel is one entry of Cohere's GET /v1/models response. `name` is the
// id a chat request must send in its `model` field; the listing has no `id`,
// `object` or `created` field. context_length is a JSON number, decoded as
// float64 like every other provider-published model number.
type cohereModel struct {
	Name          string  `json:"name"`
	ContextLength float64 `json:"context_length"`
}

// ListModelsResponseCohere decodes Cohere's models listing, {"models":[...]},
// rather than the OpenAI {object, data} envelope, and renders each entry as an
// OpenAI-shaped model. The entry's context_length becomes the provider-sourced
// context window directly.
type ListModelsResponseCohere struct {
	Data []cohereModel
}

// UnmarshalJSON accepts the {"models":[...]} envelope the real API returns and,
// for OpenAI-shaped mocks, an {object, data} envelope whose entries carry `id`.
func (l *ListModelsResponseCohere) UnmarshalJSON(b []byte) error {
	var envelope struct {
		Models []cohereModel `json:"models"`
		Data   []types.Model `json:"data"`
	}
	if err := json.Unmarshal(b, &envelope); err != nil {
		return err
	}
	if envelope.Models != nil {
		l.Data = envelope.Models
		return nil
	}
	l.Data = make([]cohereModel, len(envelope.Data))
	for i, model := range envelope.Data {
		l.Data[i] = cohereModel{Name: model.ID}
	}
	return nil
}

func (l *ListModelsResponseCohere) Transform() types.ListModelsResponse {
	provider := constants.CohereID
	models := make([]types.Model, len(l.Data))
	for i, m := range l.Data {
		models[i] = listedModel{Model: types.Model{ID: m.Name}, ContextLength: m.ContextLength}.toModel(provider)
	}

	return types.ListModelsResponse{
		Provider: &provider,
		Object:   "list",
		Data:     models,
	}
}
