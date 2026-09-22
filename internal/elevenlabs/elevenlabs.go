package elevenlabs

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"strconv"
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

	return strings.ReplaceAll(endpoint, VoicePathPlaceholder, url.PathEscape(req.Voice)), "output_format=" + format, body, nil
}

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
// stripped; the response_format becomes an output_format query parameter.
func Music(model string, req types.CreateMusicRequest) (string, []byte, error) {
	if strings.TrimSpace(req.Prompt) == "" {
		return "", nil, fmt.Errorf("the 'prompt' field is required")
	}

	format, err := OutputFormat(derefMusicFormat(req.ResponseFormat))
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
	ContentURL   string `json:"content_url"`
	Error        string `json:"error"`
	ErrorMessage string `json:"error_message"`
}

var statuses = map[string]types.VideoJobStatus{
	"queued":      types.VideoJobStatusQueued,
	"pending":     types.VideoJobStatusQueued,
	"processing":  types.VideoJobStatusInProgress,
	"in_progress": types.VideoJobStatusInProgress,
	"generating":  types.VideoJobStatusInProgress,
	"completed":   types.VideoJobStatusCompleted,
	"done":        types.VideoJobStatusCompleted,
	"succeeded":   types.VideoJobStatusCompleted,
	"failed":      types.VideoJobStatusFailed,
	"error":       types.VideoJobStatusFailed,
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
	return job, firstNonEmpty(p.ContentURL, p.MediaURL, p.VideoURL, p.DownloadURL), nil
}

const (
	videoFieldPrompt    = "prompt"
	videoFieldSeconds   = "seconds"
	videoFieldSize      = "size"
	videoFieldReference = "input_reference"
	videoFieldAudio     = "audio"
	inlineMediaType     = "inline_base64"
	sizeSeparator       = "x"
)

var resolutions = map[int]string{480: "480p", 720: "720p", 1080: "1080p"}

var aspectRatios = map[[2]int]string{{16, 9}: "16:9", {9, 16}: "9:16", {1, 1}: "1:1"}

type inlineMedia struct {
	Type          string `json:"type"`
	ContentBase64 string `json:"content_base64"`
	MimeType      string `json:"mime_type"`
}

// videoBody is the ElevenLabs POST /flows/video payload. Avatar models
// (creatify-aurora) take `image` and `audio` and no prompt; every other model
// takes a prompt with an optional `start_frame`. The two shapes are merged
// here and fields left nil are omitted.
type videoBody struct {
	ModelID      string       `json:"model_id"`
	Prompt       string       `json:"prompt,omitempty"`
	DurationSecs *int         `json:"duration_secs,omitempty"`
	Resolution   string       `json:"resolution,omitempty"`
	AspectRatio  string       `json:"aspect_ratio,omitempty"`
	StartFrame   *inlineMedia `json:"start_frame,omitempty"`
	Image        *inlineMedia `json:"image,omitempty"`
	Audio        *inlineMedia `json:"audio,omitempty"`
}

// Video rewrites an OpenAI Videos multipart form into the ElevenLabs flows
// JSON shape. A form carrying `audio` is treated as an avatar (lip-sync)
// request: the reference image becomes `image`, the clip becomes `audio`,
// and prompt/seconds are dropped because those models reject them. model is
// the request model with the provider prefix already stripped.
func Video(model string, form *multipart.Form) ([]byte, error) {
	out := videoBody{ModelID: model}

	image, err := inlineFile(form, videoFieldReference)
	if err != nil {
		return nil, err
	}
	audio, err := inlineFile(form, videoFieldAudio)
	if err != nil {
		return nil, err
	}

	if size := formValue(form, videoFieldSize); size != "" {
		if out.Resolution, out.AspectRatio, err = parseSize(size); err != nil {
			return nil, err
		}
	}

	if audio != nil {
		if image == nil {
			return nil, fmt.Errorf("the 'input_reference' image is required when 'audio' is provided")
		}
		out.Image, out.Audio = image, audio
		out.AspectRatio = ""
		return json.Marshal(out)
	}

	out.Prompt = formValue(form, videoFieldPrompt)
	if strings.TrimSpace(out.Prompt) == "" {
		return nil, fmt.Errorf("the 'prompt' field is required")
	}
	out.StartFrame = image
	if seconds := formValue(form, videoFieldSeconds); seconds != "" {
		n, err := strconv.Atoi(seconds)
		if err != nil || n <= 0 {
			return nil, fmt.Errorf("the 'seconds' field must be a positive integer")
		}
		out.DurationSecs = &n
	}
	return json.Marshal(out)
}

// parseSize turns an OpenAI `size` ("720x1280") into an ElevenLabs
// resolution and aspect_ratio. The resolution is taken from the shorter side.
func parseSize(size string) (resolution, aspect string, err error) {
	w, h, ok := strings.Cut(size, sizeSeparator)
	width, werr := strconv.Atoi(w)
	height, herr := strconv.Atoi(h)
	if !ok || werr != nil || herr != nil || width <= 0 || height <= 0 {
		return "", "", fmt.Errorf("the 'size' field must be WIDTHxHEIGHT, e.g. 1280x720")
	}
	resolution, ok = resolutions[min(width, height)]
	if !ok {
		return "", "", fmt.Errorf("elevenlabs does not support size %q, the shorter side must be 480, 720 or 1080", size)
	}
	g := gcd(width, height)
	return resolution, aspectRatios[[2]int{width / g, height / g}], nil
}

func gcd(a, b int) int {
	for b != 0 {
		a, b = b, a%b
	}
	return a
}

// inlineFile reads the first uploaded file for field into an inline media
// reference, or returns nil when the field is absent. The mime type comes
// from the part header, sniffed from the bytes when the client sent none.
func inlineFile(form *multipart.Form, field string) (*inlineMedia, error) {
	headers := form.File[field]
	if len(headers) == 0 {
		return nil, nil
	}
	f, err := headers[0].Open()
	if err != nil {
		return nil, fmt.Errorf("failed to open uploaded %s: %w", field, err)
	}
	defer f.Close()
	data, err := io.ReadAll(f)
	if err != nil {
		return nil, fmt.Errorf("failed to read uploaded %s: %w", field, err)
	}
	mimeType := headers[0].Header.Get("Content-Type")
	if mimeType == "" || mimeType == "application/octet-stream" {
		mimeType = http.DetectContentType(data)
	}
	return &inlineMedia{Type: inlineMediaType, ContentBase64: base64.StdEncoding.EncodeToString(data), MimeType: mimeType}, nil
}

func formValue(form *multipart.Form, key string) string {
	if vs := form.Value[key]; len(vs) > 0 {
		return vs[0]
	}
	return ""
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
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

func derefMusicFormat(f *types.CreateMusicRequestResponseFormat) string {
	if f == nil {
		return ""
	}
	return string(*f)
}
