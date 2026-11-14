// Package sfx translates a gateway sound-effect request, a text prompt or a
// video to score, into the job a provider's API takes, Mirelo today, and reads
// the job back.
package sfx

import (
	"cmp"
	"encoding/json"
	"fmt"
	"maps"
	"math"
	"mime/multipart"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"

	types "github.com/inference-gateway/inference-gateway/providers/types"
)

const (
	// defaultDurationSeconds is the clip length sent when the request omits
	// duration_seconds - Mirelo requires duration_ms and has no default.
	defaultDurationSeconds = 5

	// jobBudget is how long one request waits for its job in all, across the
	// create call and the long-polls that follow it.
	jobBudget = 150 * time.Second

	maxWaitSeconds    = 25
	minWaitSeconds    = 1
	waitMarginSeconds = 5
	waitQueryPrefix   = "wait="
	millisPerSecond   = 1000
	formatSeparator   = "_"
	statusSucceeded   = "succeeded"
	unknownJobError   = "no error was reported"
	mediaTypeAsset    = "asset"
	mediaTypeURL      = "url"
	minVideoSeconds   = 1
	minOffsetSeconds  = 0

	// FieldOutput is the request field naming the track a video-to-sfx
	// request wants back, one of OutputMix, OutputSFX or OutputSpeech.
	FieldOutput = "output"

	// The remaining fields of a sound-effect request, named in the errors of
	// the provider translators that live outside this package.
	FieldPrompt             = "prompt"
	FieldVideo              = "video"
	FieldDurationSeconds    = "duration_seconds"
	FieldStartOffsetSeconds = "start_offset_seconds"
	FieldPreserveSpeech     = "preserve_speech"
	FieldResponseFormat     = "response_format"
	FieldLoop               = "loop"
	FieldPromptInfluence    = "prompt_influence"

	OutputMix    = "mix"
	OutputSFX    = "sfx"
	OutputSpeech = "speech"
)

var outputs = map[string]bool{OutputMix: true, OutputSFX: true, OutputSpeech: true}

var supportedOutputs = strings.Join(slices.Sorted(maps.Keys(outputs)), ", ")

var outputFormats = map[string]string{
	string(types.CreateSFXRequestResponseFormatMp3):  "mp3_128",
	string(types.CreateSFXRequestResponseFormatFlac): "flac",
}

var supportedFormats = strings.Join(slices.Sorted(maps.Keys(outputFormats)), ", ")

var finalStatuses = map[string]bool{
	statusSucceeded:       true,
	"partially_succeeded": true,
	"failed":              true,
	"canceled":            true,
	"expired":             true,
}

var contentTypes = map[string]string{
	"mp3":  "audio/mpeg",
	"wav":  "audio/wav",
	"flac": "audio/flac",
}

// outputFormat maps a gateway response_format onto a Mirelo output.format. An
// empty format defaults to mp3 and anything Mirelo cannot produce is an error.
func outputFormat(responseFormat string) (string, error) {
	if responseFormat == "" {
		responseFormat = string(types.CreateSFXRequestResponseFormatMp3)
	}
	format, ok := outputFormats[responseFormat]
	if !ok {
		return "", fmt.Errorf("mirelo does not support response_format %q, supported formats: %s", responseFormat, supportedFormats)
	}
	return format, nil
}

type sfxBody struct {
	Model      string    `json:"model"`
	DurationMs int       `json:"duration_ms"`
	Input      sfxInput  `json:"input"`
	Controls   *controls `json:"controls,omitempty"`
	Output     sfxOutput `json:"output"`
}

type sfxInput struct {
	Prompt string `json:"prompt"`
}

type controls struct {
	Loop bool `json:"loop"`
}

type sfxOutput struct {
	Media  string `json:"media,omitempty"`
	Format string `json:"format,omitempty"`
}

type videoBody struct {
	Model         string         `json:"model"`
	DurationMs    *int           `json:"duration_ms,omitempty"`
	StartOffsetMs *int           `json:"start_offset_ms,omitempty"`
	Input         videoInput     `json:"input"`
	Controls      *videoControls `json:"controls,omitempty"`
	Output        sfxOutput      `json:"output"`
}

type videoInput struct {
	Video  MediaRef `json:"video"`
	Prompt string   `json:"prompt,omitempty"`
}

// MediaRef points Mirelo at the clip to score: an asset uploaded through a
// ticket, or a public URL Mirelo fetches itself.
type MediaRef struct {
	Type string `json:"type"`
	ID   string `json:"id,omitempty"`
	URL  string `json:"url,omitempty"`
}

// AssetRef references a clip uploaded as the asset id.
func AssetRef(id string) MediaRef {
	return MediaRef{Type: mediaTypeAsset, ID: id}
}

// URLRef references a clip Mirelo downloads from u, with no upload.
func URLRef(u string) MediaRef {
	return MediaRef{Type: mediaTypeURL, URL: u}
}

type videoControls struct {
	PreserveSpeech *bool `json:"preserve_speech,omitempty"`
	SeparateSpeech bool  `json:"separate_speech,omitempty"`
}

// FromText rewrites a gateway CreateSFXRequest into the Mirelo text-to-sfx shape.
// model is the request model with the provider prefix already stripped.
func FromText(model string, req types.CreateSFXRequest) ([]byte, error) {
	if strings.TrimSpace(req.Prompt) == "" {
		return nil, fmt.Errorf("the 'prompt' field is required")
	}
	if req.PromptInfluence != nil {
		return nil, fmt.Errorf("mirelo does not support 'prompt_influence', omit the field")
	}

	format, err := outputFormat(responseFormatOf(req))
	if err != nil {
		return nil, err
	}

	seconds := float64(defaultDurationSeconds)
	if req.DurationSeconds != nil {
		seconds = float64(*req.DurationSeconds)
	}

	body := sfxBody{
		Model:      model,
		DurationMs: int(math.Round(seconds * millisPerSecond)),
		Input:      sfxInput{Prompt: req.Prompt},
		Output:     sfxOutput{Format: format},
	}
	if req.Loop != nil {
		body.Controls = &controls{Loop: *req.Loop}
	}
	return json.Marshal(body)
}

// VideoScore is a validated Mirelo video-to-sfx job still missing the clip it
// scores, so a request Mirelo cannot serve is refused before any upload.
type VideoScore struct {
	body videoBody
}

// FromVideo validates a gateway CreateSFXRequest that sets `video` and maps it
// onto the Mirelo video-to-sfx shape. Omitting the duration scores the rest of
// the clip. model is the request model with the provider prefix stripped.
func FromVideo(model string, req types.CreateSFXRequest) (VideoScore, error) {
	if field := TextOnlyField(req); field != "" {
		return VideoScore{}, fmt.Errorf("mirelo does not support '%s' when scoring a video, omit the field", field)
	}
	format, err := outputFormat(responseFormatOf(req))
	if err != nil {
		return VideoScore{}, err
	}
	output := Output(req)
	if !outputs[output] {
		return VideoScore{}, fmt.Errorf("the '%s' field must be one of: %s", FieldOutput, supportedOutputs)
	}

	body := videoBody{
		Model:    model,
		Input:    videoInput{Prompt: req.Prompt},
		Controls: videoControlsFor(req.PreserveSpeech, output),
		Output:   sfxOutput{Format: format},
	}
	if body.DurationMs, err = windowMillis(FieldDurationSeconds, req.DurationSeconds, minVideoSeconds); err != nil {
		return VideoScore{}, err
	}
	if body.StartOffsetMs, err = windowMillis(FieldStartOffsetSeconds, req.StartOffsetSeconds, minOffsetSeconds); err != nil {
		return VideoScore{}, err
	}
	return VideoScore{body: body}, nil
}

// Body is the job request scoring the clip video points at.
func (s VideoScore) Body(video MediaRef) []byte {
	s.body.Input.Video = video
	out, _ := json.Marshal(s.body)
	return out
}

// Output is the track a video-to-sfx request wants back, the mix when unset.
func Output(req types.CreateSFXRequest) string {
	if req.Output == nil {
		return OutputMix
	}
	return string(*req.Output)
}

// TextOnlyField names the first text-to-sfx field a video request sets, ""
// when none. Scoring a video has no counterpart for them, so they are refused
// rather than dropped and the caller never gets audio that ignores them.
func TextOnlyField(req types.CreateSFXRequest) string {
	switch {
	case req.Loop != nil:
		return FieldLoop
	case req.PromptInfluence != nil:
		return FieldPromptInfluence
	}
	return ""
}

func responseFormatOf(req types.CreateSFXRequest) string {
	if req.ResponseFormat == nil {
		return ""
	}
	return string(*req.ResponseFormat)
}

// windowMillis converts an optional window bound in seconds to milliseconds,
// nil when absent.
func windowMillis(field string, seconds *float32, minSeconds float64) (*int, error) {
	if seconds == nil {
		return nil, nil
	}
	return millisAtLeast(field, float64(*seconds), minSeconds)
}

// optionalMillis reads a multipart seconds field into milliseconds, nil when
// absent.
func optionalMillis(form *multipart.Form, field string, minSeconds float64) (*int, error) {
	raw := formValue(form, field)
	if raw == "" {
		return nil, nil
	}
	seconds, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return nil, windowError(field, minSeconds)
	}
	return millisAtLeast(field, seconds, minSeconds)
}

func millisAtLeast(field string, seconds, minSeconds float64) (*int, error) {
	if seconds < minSeconds {
		return nil, windowError(field, minSeconds)
	}
	millis := int(math.Round(seconds * millisPerSecond))
	return &millis, nil
}

func windowError(field string, minSeconds float64) error {
	return fmt.Errorf("the '%s' field must be a number of at least %g", field, minSeconds)
}

// videoControlsFor derives Mirelo's speech controls from the wanted track:
// any track but the mix needs the speech separated, and the sfx track only
// exists when speech is not preserved, so that is the default for it.
func videoControlsFor(preserve *bool, output string) *videoControls {
	controls := &videoControls{PreserveSpeech: preserve, SeparateSpeech: output != OutputMix}
	if output == OutputSFX && controls.PreserveSpeech == nil {
		keep := false
		controls.PreserveSpeech = &keep
	}
	if controls.PreserveSpeech == nil && !controls.SeparateSpeech {
		return nil
	}
	return controls
}

// videoControlsFrom reads the speech choice off a multipart form and derives
// the controls for the wanted track as videoControlsFor does.
func videoControlsFrom(form *multipart.Form, output string) (*videoControls, error) {
	raw := formValue(form, FieldPreserveSpeech)
	if raw == "" {
		return videoControlsFor(nil, output), nil
	}
	preserve, err := strconv.ParseBool(raw)
	if err != nil {
		return nil, fmt.Errorf("the '%s' field must be true or false", FieldPreserveSpeech)
	}
	return videoControlsFor(&preserve, output), nil
}

func formValue(form *multipart.Form, key string) string {
	if values := form.Value[key]; len(values) > 0 {
		return values[0]
	}
	return ""
}

// Polling is how a job is awaited: the ?wait= query every request carries and
// how many requests the budget allows.
type Polling struct {
	Query    string
	Requests int
}

// Plan sizes the long-poll: Mirelo holds a request open for up to 25 seconds,
// shortened to stay inside the gateway's upstream timeout and the transport's
// response-header timeout (zero means none), and the requests fill the budget.
func Plan(upstreamTimeout, headerTimeout time.Duration) Polling {
	limit := upstreamTimeout
	if headerTimeout > 0 && headerTimeout < limit {
		limit = headerTimeout
	}
	seconds := min(max(int(limit.Seconds())-waitMarginSeconds, minWaitSeconds), maxWaitSeconds)
	return Polling{
		Query:    waitQueryPrefix + strconv.Itoa(seconds),
		Requests: int(math.Ceil(jobBudget.Seconds() / float64(seconds))),
	}
}

// JobPath is the path a job created at createPath is polled on. It is built
// from the job id rather than taken from `urls.self` so the API key is only
// ever sent to the configured Mirelo base URL.
func JobPath(createPath, jobID string) string {
	return createPath + "/" + url.PathEscape(jobID)
}

// Job is the subset of a Mirelo generation job the gateway reads.
type Job struct {
	ID     string `json:"id"`
	Model  string `json:"model"`
	Status string `json:"status"`
	Errors []struct {
		Message string `json:"message"`
	} `json:"errors"`
	Result *struct {
		Output  track   `json:"output"`
		Outputs []track `json:"outputs"`
	} `json:"result"`
}

// track is one audio output of a job: the mix, or a separated stem such as
// the clip's speech.
type track struct {
	Type     string `json:"type"`
	Variants []struct {
		Files *struct {
			Audio *struct {
				URL    string `json:"url"`
				Format string `json:"format"`
			} `json:"audio"`
			Video *struct {
				URL string `json:"url"`
			} `json:"video"`
		} `json:"files"`
	} `json:"variants"`
}

// ParseJob decodes a Mirelo job envelope.
func ParseJob(raw []byte) (Job, error) {
	var job Job
	if err := json.Unmarshal(raw, &job); err != nil {
		return Job{}, fmt.Errorf("failed to decode mirelo job response: %w", err)
	}
	if job.ID == "" {
		return Job{}, fmt.Errorf("mirelo job response carries no id")
	}
	return job, nil
}

// Pending reports whether the job has yet to reach a final state. Mirelo's
// status list is open, so an unknown status counts as still running.
func (j Job) Pending() bool {
	return !finalStatuses[j.Status]
}

// Audio returns the download URL and Content-Type of the first audio file of
// the wanted track, the mix when output is empty, so a partially_succeeded job
// still relays what it produced. A job without that file is an error carrying
// Mirelo's own message.
func (j Job) Audio(output string) (audioURL, contentType string, err error) {
	output = cmp.Or(output, OutputMix)
	for _, t := range j.tracks() {
		if t.Type != "" && t.Type != output {
			continue
		}
		for _, variant := range t.Variants {
			if variant.Files != nil && variant.Files.Audio != nil && variant.Files.Audio.URL != "" {
				container, _, _ := strings.Cut(variant.Files.Audio.Format, formatSeparator)
				return variant.Files.Audio.URL, contentTypes[container], nil
			}
		}
	}
	if j.Status != statusSucceeded {
		return "", "", fmt.Errorf("mirelo sound effect generation %s: %s", j.Status, j.errorMessage())
	}
	return "", "", fmt.Errorf("mirelo sound effect generation succeeded without a %s track", output)
}

// tracks lists the job's outputs, which Mirelo reports as one `output` or,
// when speech is separated, several `outputs`.
func (j Job) tracks() []track {
	if j.Result == nil {
		return nil
	}
	if len(j.Result.Outputs) > 0 {
		return j.Result.Outputs
	}
	return []track{j.Result.Output}
}

func (j Job) errorMessage() string {
	messages := make([]string, 0, len(j.Errors))
	for _, e := range j.Errors {
		if e.Message != "" {
			messages = append(messages, e.Message)
		}
	}
	if len(messages) == 0 {
		return unknownJobError
	}
	return strings.Join(messages, ", ")
}
