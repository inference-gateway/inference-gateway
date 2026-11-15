// Package videos translates an OpenAI Videos request into the job a
// provider's API takes, ElevenLabs and Mirelo today, and reads the job back.
package videos

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

const (
	videoFieldPrompt    = "prompt"
	videoFieldSeconds   = "seconds"
	videoFieldSize      = "size"
	videoFieldReference = "input_reference"
	videoFieldAudio     = "audio"
	videoFieldRefImages = "reference_images"
	inlineMediaType     = "inline_base64"
	sizeSeparator       = "x"
	veoModelPrefix      = "veo-"
	veoRoleSubject      = "subject"
)

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

// referenceImageLimits is how many `images` each ElevenLabs video model
// accepts. Models missing here take no reference images.
var referenceImageLimits = map[string]int{
	"bytedance-seedance-v2":      9,
	"bytedance-seedance-v2-fast": 9,
	"bytedance-seedance-v2-mini": 9,
	"bytedance-seedance-v2.5":    30,
	"veo-3.1-generate-001":       3,
	"veo-3.1-fast-generate-001":  3,
}

var resolutions = map[int]string{480: "480p", 720: "720p", 1080: "1080p"}

var aspectRatios = map[[2]int]string{{16, 9}: "16:9", {9, 16}: "9:16", {1, 1}: "1:1"}

// InlineMedia is an uploaded image or clip carried inline as an
// inline_base64 reference, the media shape the ElevenLabs flows API takes.
type InlineMedia struct {
	Type          string `json:"type"`
	ContentBase64 string `json:"content_base64"`
	MimeType      string `json:"mime_type"`
}

// videoBody is the ElevenLabs POST /flows/video payload. Avatar models
// (creatify-aurora) take `image` and `audio` and no prompt; every other model
// takes a prompt with an optional `start_frame` or reference `images`. The two
// shapes are merged here and fields left nil are omitted.
type videoBody struct {
	ModelID      string       `json:"model_id"`
	Prompt       string       `json:"prompt,omitempty"`
	DurationSecs *int         `json:"duration_secs,omitempty"`
	Resolution   string       `json:"resolution,omitempty"`
	AspectRatio  string       `json:"aspect_ratio,omitempty"`
	StartFrame   *InlineMedia `json:"start_frame,omitempty"`
	Images       []any        `json:"images,omitempty"`
	Image        *InlineMedia `json:"image,omitempty"`
	Audio        *InlineMedia `json:"audio,omitempty"`
}

// veoImage is a Veo `images` entry: Veo wraps each image with the role it
// plays, where the other models take the bare image.
type veoImage struct {
	Image *InlineMedia `json:"image"`
	Role  string       `json:"role"`
}

// Video rewrites an OpenAI Videos multipart form into the ElevenLabs flows
// JSON shape. A form carrying `audio` is an avatar (lip-sync) request: the
// reference image becomes `image`, the clip becomes `audio`, and prompt,
// seconds and reference_images are dropped. Prompt-driven models refuse it.
// model is the request model with the provider prefix already stripped.
func Video(model string, form *multipart.Form) ([]byte, error) {
	fields := url.Values(form.Value)
	out := videoBody{ModelID: model}

	image, err := InlineFile(form, videoFieldReference)
	if err != nil {
		return nil, err
	}
	audio, err := InlineFile(form, videoFieldAudio)
	if err != nil {
		return nil, err
	}

	if size := fields.Get(videoFieldSize); size != "" {
		if out.Resolution, out.AspectRatio, err = parseSize(size); err != nil {
			return nil, err
		}
	}

	if audio != nil {
		if rendersFromPrompt(model) {
			return nil, fmt.Errorf("model %q does not lip-sync to 'audio': it renders from a prompt", model)
		}
		if image == nil {
			return nil, fmt.Errorf("the 'input_reference' image is required when 'audio' is provided")
		}
		out.Image, out.Audio = image, audio
		out.AspectRatio = ""
		return json.Marshal(out)
	}

	out.Prompt = fields.Get(videoFieldPrompt)
	if strings.TrimSpace(out.Prompt) == "" {
		return nil, fmt.Errorf("the 'prompt' field is required")
	}
	if out.Images, err = referenceImages(model, form); err != nil {
		return nil, err
	}
	if out.Images != nil && image != nil {
		return nil, fmt.Errorf("'reference_images' cannot be combined with 'input_reference'")
	}
	out.StartFrame = image
	if seconds := fields.Get(videoFieldSeconds); seconds != "" {
		n, err := strconv.Atoi(seconds)
		if err != nil || n <= 0 {
			return nil, fmt.Errorf("the 'seconds' field must be a positive integer")
		}
		out.DurationSecs = &n
	}
	return json.Marshal(out)
}

// rendersFromPrompt reports whether model is a known prompt-driven video model,
// which takes no driving audio. Unknown models are left to ElevenLabs.
func rendersFromPrompt(model string) bool {
	_, known := referenceImageLimits[model]
	return known
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

// referenceImages inlines every `reference_images` part into an ElevenLabs
// `images` list, or returns nil when none were sent. The count is checked
// against the model's limit before any file is read.
func referenceImages(model string, form *multipart.Form) ([]any, error) {
	headers := form.File[videoFieldRefImages]
	if len(headers) == 0 {
		return nil, nil
	}
	limit, ok := referenceImageLimits[model]
	if !ok {
		return nil, fmt.Errorf("model %q does not accept 'reference_images'", model)
	}
	if len(headers) > limit {
		return nil, fmt.Errorf("model %q accepts at most %d 'reference_images', got %d", model, limit, len(headers))
	}
	images := make([]any, len(headers))
	for i, fh := range headers {
		img, err := InlineHeader(fh, videoFieldRefImages)
		if err != nil {
			return nil, err
		}
		if strings.HasPrefix(model, veoModelPrefix) {
			images[i] = veoImage{Image: img, Role: veoRoleSubject}
		} else {
			images[i] = img
		}
	}
	return images, nil
}

// InlineFile reads the first uploaded file for field into an inline media
// reference, or returns nil when the field is absent.
func InlineFile(form *multipart.Form, field string) (*InlineMedia, error) {
	headers := form.File[field]
	if len(headers) == 0 {
		return nil, nil
	}
	return InlineHeader(headers[0], field)
}

// InlineHeader reads one uploaded file into an inline media reference. The
// mime type comes from the part header, sniffed from the bytes when the
// client sent none.
func InlineHeader(fh *multipart.FileHeader, field string) (*InlineMedia, error) {
	f, err := fh.Open()
	if err != nil {
		return nil, fmt.Errorf("failed to open uploaded %s: %w", field, err)
	}
	defer f.Close()
	data, err := io.ReadAll(f)
	if err != nil {
		return nil, fmt.Errorf("failed to read uploaded %s: %w", field, err)
	}
	mimeType := fh.Header.Get("Content-Type")
	if mimeType == "" || mimeType == "application/octet-stream" {
		mimeType = http.DetectContentType(data)
	}
	return &InlineMedia{Type: inlineMediaType, ContentBase64: base64.StdEncoding.EncodeToString(data), MimeType: acceptedMimeType(mimeType)}, nil
}

// acceptedMimeTypes maps the other common names of an audio type onto the one
// ElevenLabs accepts. Go sniffs a WAV as audio/wave, which it refuses.
var acceptedMimeTypes = map[string]string{
	"audio/wave":     "audio/wav",
	"audio/x-wav":    "audio/wav",
	"audio/vnd.wave": "audio/wav",
	"audio/mp3":      "audio/mpeg",
}

func acceptedMimeType(mimeType string) string {
	if accepted, renamed := acceptedMimeTypes[mimeType]; renamed {
		return accepted
	}
	return mimeType
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
