package api

import (
	"encoding/json"
	"fmt"
	"net/http"

	elevenlabs "github.com/inference-gateway/inference-gateway/internal/elevenlabs"
	constants "github.com/inference-gateway/inference-gateway/providers/constants"
	types "github.com/inference-gateway/inference-gateway/providers/types"
)

// MusicHandler implements POST /v1/audio/music, the gateway extension that
// composes a music clip from a text prompt. OpenAI has no music endpoint, so
// the request mirrors /audio/sfx (JSON in, raw audio bytes out) and only
// providers that carry a Music endpoint (currently elevenlabs) can serve it;
// the rest receive a 400, mirroring the schema's MusicNotSupported response.
//
// The endpoint shares the AUDIO_ENABLED toggle with /audio/speech.
func (router *RouterImpl) MusicHandler(w http.ResponseWriter, r *http.Request) {
	if !router.audioEnabled(w) {
		return
	}

	router.proxyJSONBody(w, r, jsonProxy{
		apiName:      "Music generation",
		exampleModel: "elevenlabs/music_v2",
		endpointOf:   func(e types.Endpoints) *string { return e.Music },
		notSupported: "Music generation is not supported by this provider yet.",
		translators:  map[types.Provider]jsonTranslator{constants.ElevenlabsID: elevenlabsMusic},
	})
}

// elevenlabsMusic rewrites a gateway CreateMusicRequest into the ElevenLabs
// music shape, whose audio container rides in an output_format query parameter.
func elevenlabsMusic(endpoint, model string, body []byte) (string, string, []byte, error) {
	var req types.CreateMusicRequest
	if err := json.Unmarshal(body, &req); err != nil {
		return "", "", nil, fmt.Errorf("failed to decode request: %w", err)
	}
	query, out, err := elevenlabs.Music(model, req)
	if err != nil {
		return "", "", nil, err
	}
	return endpoint, query, out, nil
}
