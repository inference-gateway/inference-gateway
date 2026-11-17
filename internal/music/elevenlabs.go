package music

import (
	"encoding/json"
	"fmt"
	"strings"

	speech "github.com/inference-gateway/inference-gateway/internal/speech"
	types "github.com/inference-gateway/inference-gateway/providers/types"
)

// musicBody is the ElevenLabs POST /music payload. The clip length is
// expressed in milliseconds and the no-vocals guarantee as a force flag.
type musicBody struct {
	Prompt            string `json:"prompt"`
	ModelID           string `json:"model_id"`
	MusicLengthMs     *int64 `json:"music_length_ms,omitempty"`
	ForceInstrumental *bool  `json:"force_instrumental,omitempty"`
}

// Music rewrites a gateway CreateMusicRequest into the ElevenLabs music
// shape. model is the request model with the provider prefix already
// stripped - the response_format becomes an output_format query parameter.
func Music(model string, req types.CreateMusicRequest) (string, []byte, error) {
	if strings.TrimSpace(req.Prompt) == "" {
		return "", nil, fmt.Errorf("the 'prompt' field is required")
	}

	format, err := speech.OutputFormat(derefMusicFormat(req.ResponseFormat))
	if err != nil {
		return "", nil, err
	}

	var lengthMs *int64
	if req.DurationSeconds != nil {
		ms := int64(*req.DurationSeconds * 1000)
		lengthMs = &ms
	}

	body, err := json.Marshal(musicBody{
		Prompt:            req.Prompt,
		ModelID:           model,
		MusicLengthMs:     lengthMs,
		ForceInstrumental: req.Instrumental,
	})
	if err != nil {
		return "", nil, err
	}
	return "output_format=" + format, body, nil
}

func derefMusicFormat(f *types.CreateMusicRequestResponseFormat) string {
	if f == nil {
		return ""
	}
	return string(*f)
}
