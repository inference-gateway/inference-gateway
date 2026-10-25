// Hand-maintained: Cloudflare's models endpoint is not OpenAI-compatible
// (see .openapi-ignore).
package transformers

import (
	"encoding/json"
	"time"

	constants "github.com/inference-gateway/inference-gateway/providers/constants"
	types "github.com/inference-gateway/inference-gateway/providers/types"
)

// cloudflareFinetune is one entry of Cloudflare's GET
// .../ai/finetunes/public response. `name` is the public fine-tune's id;
// `created_at` is the only timestamp the listing publishes.
type cloudflareFinetune struct {
	Name      string `json:"name"`
	CreatedAt string `json:"created_at"`
}

// ListModelsResponseCloudflare decodes Cloudflare's native models listing,
// {"result":[...],"success":true}, rather than the OpenAI {object, data}
// envelope, and renders each public fine-tune as an OpenAI-shaped model.
type ListModelsResponseCloudflare struct {
	Object string
	Data   []types.Model
}

// UnmarshalJSON accepts the {"result":[...]} envelope the real API returns and,
// for OpenAI-shaped mocks, an {object, data} envelope whose entries carry `id`.
func (l *ListModelsResponseCloudflare) UnmarshalJSON(b []byte) error {
	var envelope struct {
		Object string               `json:"object"`
		Data   []types.Model        `json:"data"`
		Result []cloudflareFinetune `json:"result"`
	}
	if err := json.Unmarshal(b, &envelope); err != nil {
		return err
	}
	l.Object = envelope.Object
	if envelope.Data != nil {
		l.Data = envelope.Data
		return nil
	}

	l.Data = make([]types.Model, 0, len(envelope.Result))
	for _, finetune := range envelope.Result {
		if finetune.Name == "" {
			continue
		}
		l.Data = append(l.Data, types.Model{
			ID:      finetune.Name,
			Object:  "model",
			OwnedBy: string(constants.CloudflareID),
			Created: cloudflareCreatedAt(finetune.CreatedAt),
		})
	}
	return nil
}

// cloudflareCreatedAt converts the listing's creation timestamp to the Unix
// seconds an OpenAI model object carries. Cloudflare has published both a
// space-separated datetime and RFC 3339; an unparsable value stays zero.
func cloudflareCreatedAt(value string) int64 {
	for _, layout := range []string{time.RFC3339Nano, "2006-01-02 15:04:05.999"} {
		if parsed, err := time.Parse(layout, value); err == nil {
			return parsed.Unix()
		}
	}
	return 0
}

func (l *ListModelsResponseCloudflare) Transform() types.ListModelsResponse {
	provider := constants.CloudflareID
	models := make([]types.Model, len(l.Data))
	for i, model := range l.Data {
		model.ServedBy = provider
		model.ID = string(provider) + "/" + model.ID
		models[i] = model
	}

	return types.ListModelsResponse{
		Provider: &provider,
		Object:   l.Object,
		Data:     models,
	}
}
