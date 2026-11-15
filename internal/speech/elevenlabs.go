// Package speech translates an OpenAI speech request into the shape a
// provider's text-to-speech API takes, ElevenLabs today, and speaks it in a
// voice cloned from a reference sample.
package speech

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	types "github.com/inference-gateway/inference-gateway/providers/types"
)

const VoicePathPlaceholder = "{voice}"

var outputFormats = map[string]string{
	string(types.CreateSpeechRequestResponseFormatMp3):  "mp3_44100_128",
	string(types.CreateSpeechRequestResponseFormatOpus): "opus_48000_128",
	string(types.CreateSpeechRequestResponseFormatPcm):  "pcm_24000",
}

const SupportedFormats = "mp3, opus, pcm"

// OutputFormat maps an OpenAI response_format onto an ElevenLabs
// output_format. An empty format defaults to mp3; anything ElevenLabs cannot
// produce is an error.
func OutputFormat(responseFormat string) (string, error) {
	if responseFormat == "" {
		responseFormat = string(types.CreateSpeechRequestResponseFormatMp3)
	}
	format, ok := outputFormats[responseFormat]
	if !ok {
		return "", fmt.Errorf("elevenlabs does not support response_format %q, supported formats: %s", responseFormat, SupportedFormats)
	}
	return format, nil
}

// SpeechWavFormat is the ElevenLabs output_format served for response_format wav.
const SpeechWavFormat = "wav_48000"

// speechFormat maps an OpenAI response_format onto an ElevenLabs
// text-to-speech output_format. Unlike sound effects and music, speech can be
// a WAV.
func speechFormat(responseFormat string) (string, error) {
	if responseFormat == string(types.CreateSpeechRequestResponseFormatWav) {
		return SpeechWavFormat, nil
	}
	if format, err := OutputFormat(responseFormat); err == nil {
		return format, nil
	}
	return "", fmt.Errorf("elevenlabs does not support response_format %q for speech, supported formats: %s, wav", responseFormat, SupportedFormats)
}

// speechBody is the ElevenLabs POST /text-to-speech/{voice_id} payload.
type speechBody struct {
	Text          string         `json:"text"`
	ModelID       string         `json:"model_id"`
	LanguageCode  *string        `json:"language_code,omitempty"`
	VoiceSettings *voiceSettings `json:"voice_settings,omitempty"`
}

type voiceSettings struct {
	Speed float32 `json:"speed"`
}

// Speech rewrites an OpenAI CreateSpeechRequest into the ElevenLabs
// text-to-speech shape. endpoint is the registry's speech endpoint template;
// the returned path has the voice substituted in and the returned query
// carries output_format. model is the request model with the provider prefix
// already stripped.
func Speech(endpoint, model string, req types.CreateSpeechRequest) (path, query string, body []byte, err error) {
	if strings.TrimSpace(req.Voice) == "" {
		return "", "", nil, fmt.Errorf("elevenlabs needs a voice: set 'voice' to an elevenlabs voice id or send a 'reference_audio' sample")
	}

	format, err := speechFormat(derefFormat(req.ResponseFormat))
	if err != nil {
		return "", "", nil, err
	}

	out := speechBody{Text: req.Input, ModelID: model, LanguageCode: req.Language}
	if req.Speed != nil {
		out.VoiceSettings = &voiceSettings{Speed: *req.Speed}
	}
	body, err = json.Marshal(out)
	if err != nil {
		return "", "", nil, err
	}

	return strings.ReplaceAll(endpoint, VoicePathPlaceholder, url.PathEscape(req.Voice)), "output_format=" + format, body, nil
}

func derefFormat(f *types.CreateSpeechRequestResponseFormat) string {
	if f == nil {
		return ""
	}
	return string(*f)
}
