// Package elevenlabs translates between the gateway's OpenAI-compatible audio
// and video payloads and ElevenLabs' native API shapes.
//
// Every other provider the gateway speaks to is OpenAI-compatible, so their
// requests are proxied byte-for-byte. ElevenLabs is not: the voice lives in
// the URL path rather than the body, the text field is `text` instead of
// `input`/`prompt`, the model is `model_id`, and the audio container is a
// single `output_format` string that encodes sample rate and bitrate. The
// functions here are pure so they can be tested without a server.
package elevenlabs

import (
	"encoding/json"
	"fmt"
	"strings"

	types "github.com/inference-gateway/inference-gateway/providers/types"
)

// VoicePathPlaceholder is the token in the speech endpoint that carries the
// voice id (ElevenlabsSpeechEndpoint is "/text-to-speech/{voice}").
const VoicePathPlaceholder = "{voice}"

// JobIDPathPlaceholder is the token in the video retrieve endpoint that
// carries the upstream generation id.
const JobIDPathPlaceholder = "{generation_id}"

// Default ElevenLabs output formats per OpenAI response_format. ElevenLabs
// takes a single string encoding container, sample rate and bitrate, and has
// no aac, flac or wav variant at all - those are rejected rather than
// silently downgraded, so a caller never gets mp3 bytes labelled as wav.
var outputFormats = map[string]string{
	string(types.CreateSpeechRequestResponseFormatMp3):  "mp3_44100_128",
	string(types.CreateSpeechRequestResponseFormatOpus): "opus_48000_128",
	string(types.CreateSpeechRequestResponseFormatPcm):  "pcm_24000",
}

// SupportedFormats lists the response_format values ElevenLabs can serve, for
// use in client-facing error messages.
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
		return "", "", nil, fmt.Errorf("the 'voice' field is required for elevenlabs and must be an elevenlabs voice id")
	}

	format, err := OutputFormat(derefFormat(req.ResponseFormat))
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

	// The voice id is user input landing in a URL path; escaping it keeps a
	// crafted value from rewriting the upstream path.
	return strings.ReplaceAll(endpoint, VoicePathPlaceholder, pathEscape(req.Voice)), "output_format=" + format, body, nil
}

// sfxBody is the ElevenLabs POST /sound-generation payload.
type sfxBody struct {
	Text            string   `json:"text"`
	ModelID         string   `json:"model_id"`
	OutputFormat    string   `json:"output_format"`
	DurationSeconds *float32 `json:"duration_seconds,omitempty"`
	PromptInfluence *float32 `json:"prompt_influence,omitempty"`
	Loop            *bool    `json:"loop,omitempty"`
}

// SFX rewrites a gateway CreateSFXRequest into the ElevenLabs sound-generation
// shape. model is the request model with the provider prefix already stripped.
func SFX(model string, req types.CreateSFXRequest) ([]byte, error) {
	if strings.TrimSpace(req.Prompt) == "" {
		return nil, fmt.Errorf("the 'prompt' field is required")
	}

	format, err := OutputFormat(derefSFXFormat(req.ResponseFormat))
	if err != nil {
		return nil, err
	}

	return json.Marshal(sfxBody{
		Text:            req.Prompt,
		ModelID:         model,
		OutputFormat:    format,
		DurationSeconds: req.DurationSeconds,
		PromptInfluence: req.PromptInfluence,
		Loop:            req.Loop,
	})
}

// videoPayload is the subset of an ElevenLabs video generation response the
// gateway maps onto a VideoJob. ElevenLabs names these fields differently
// across its flow endpoints, so each one accepts the aliases seen in the wild
// rather than failing the whole mapping on a rename.
type videoPayload struct {
	GenerationID string `json:"generation_id"`
	ID           string `json:"id"`
	Status       string `json:"status"`
	State        string `json:"state"`
	Progress     *int   `json:"progress"`
	ModelID      string `json:"model_id"`
	CreatedAt    *int   `json:"created_at_unix"`
	CompletedAt  *int   `json:"completed_at_unix"`
	MediaURL     string `json:"media_url"`
	VideoURL     string `json:"video_url"`
	DownloadURL  string `json:"download_url"`
	Error        string `json:"error"`
	ErrorMessage string `json:"error_message"`
}

// statuses maps ElevenLabs job states onto the VideoJob status enum. Anything
// unrecognized is reported as in_progress so a client keeps polling instead of
// treating a new upstream state as a terminal failure.
var statuses = map[string]types.VideoJobStatus{
	"queued":     types.VideoJobStatusQueued,
	"pending":    types.VideoJobStatusQueued,
	"processing": types.VideoJobStatusInProgress,
	"in_progress": types.VideoJobStatusInProgress,
	"generating": types.VideoJobStatusInProgress,
	"completed":  types.VideoJobStatusCompleted,
	"done":       types.VideoJobStatusCompleted,
	"succeeded":  types.VideoJobStatusCompleted,
	"failed":     types.VideoJobStatusFailed,
	"error":      types.VideoJobStatusFailed,
}

// Job maps an ElevenLabs video generation payload onto the gateway's VideoJob
// and returns the URL the rendered video can be downloaded from, empty while
// the render is not finished. fallbackModel is used when the payload does not
// echo the model back.
func Job(raw []byte, fallbackModel string, createdAt int) (types.VideoJob, string, error) {
	var p videoPayload
	if err := json.Unmarshal(raw, &p); err != nil {
		return types.VideoJob{}, "", fmt.Errorf("failed to decode elevenlabs video response: %w", err)
	}

	id := firstNonEmpty(p.GenerationID, p.ID)
	if id == "" {
		return types.VideoJob{}, "", fmt.Errorf("elevenlabs video response carries no generation id")
	}

	status, ok := statuses[strings.ToLower(firstNonEmpty(p.Status, p.State))]
	if !ok {
		status = types.VideoJobStatusInProgress
	}

	job := types.VideoJob{
		ID:        id,
		Object:    types.VideoJobObjectVideo,
		Model:     firstNonEmpty(p.ModelID, fallbackModel),
		Status:    status,
		Progress:  p.Progress,
		CreatedAt: createdAt,
	}
	if p.CreatedAt != nil {
		job.CreatedAt = *p.CreatedAt
	}
	if status == types.VideoJobStatusCompleted || status == types.VideoJobStatusFailed {
		job.CompletedAt = p.CompletedAt
	}
	if status == types.VideoJobStatusFailed {
		message := firstNonEmpty(p.ErrorMessage, p.Error, "the video generation job failed")
		job.Error = &struct {
			Code    *string `json:"code,omitempty"`
			Message *string `json:"message,omitempty"`
		}{Message: &message}
	}

	if status != types.VideoJobStatusCompleted {
		return job, "", nil
	}
	return job, firstNonEmpty(p.MediaURL, p.VideoURL, p.DownloadURL), nil
}

// RetrievePath substitutes the upstream job id into the retrieve endpoint
// template.
func RetrievePath(endpoint, jobID string) string {
	return strings.ReplaceAll(endpoint, JobIDPathPlaceholder, pathEscape(jobID))
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

// pathEscape strips the characters that would let a caller-supplied id or
// voice escape its path segment.
func pathEscape(v string) string {
	return strings.NewReplacer("/", "", "?", "", "#", "", "..", "").Replace(v)
}

func derefFormat(f *types.CreateSpeechRequestResponseFormat) string {
	if f == nil {
		return ""
	}
	return string(*f)
}

func derefSFXFormat(f *types.CreateSFXRequestResponseFormat) string {
	if f == nil {
		return ""
	}
	return string(*f)
}
