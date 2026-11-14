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
	minVideoSeconds   = 1
	minOffsetSeconds  = 0

	// FieldOutput is the multipart field naming the track a video-to-sfx
	// request wants back, one of OutputMix, OutputSFX or OutputSpeech.
	FieldOutput = "output"

	fieldPrompt             = "prompt"
	fieldDurationSeconds    = "duration_seconds"
	fieldStartOffsetSeconds = "start_offset_seconds"
	fieldPreserveSpeech     = "preserve_speech"
	fieldResponseFormat     = "response_format"

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
	Format string `json:"format"`
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
	Video  mediaRef `json:"video"`
	Prompt string   `json:"prompt,omitempty"`
}

type mediaRef struct {
	Type string `json:"type"`
	ID   string `json:"id"`
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

	responseFormat := ""
	if req.ResponseFormat != nil {
		responseFormat = string(*req.ResponseFormat)
	}
	format, err := outputFormat(responseFormat)
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

// FromVideo rewrites the multipart video-to-sfx form into the Mirelo shape,
// scoring the asset the upload produced. Omitting the duration scores the rest
// of the clip. model is the request model with the provider prefix stripped.
func FromVideo(model string, form *multipart.Form, assetID string) ([]byte, error) {
	format, err := outputFormat(formValue(form, fieldResponseFormat))
	if err != nil {
		return nil, err
	}
	output := cmp.Or(formValue(form, FieldOutput), OutputMix)
	if !outputs[output] {
		return nil, fmt.Errorf("the '%s' field must be one of: %s", FieldOutput, supportedOutputs)
	}

	body := videoBody{
		Model:  model,
		Input:  videoInput{Video: mediaRef{Type: mediaTypeAsset, ID: assetID}, Prompt: formValue(form, fieldPrompt)},
		Output: sfxOutput{Format: format},
	}
	if body.DurationMs, err = optionalMillis(form, fieldDurationSeconds, minVideoSeconds); err != nil {
		return nil, err
	}
	if body.StartOffsetMs, err = optionalMillis(form, fieldStartOffsetSeconds, minOffsetSeconds); err != nil {
		return nil, err
	}
	if body.Controls, err = videoControlsFrom(form, output); err != nil {
		return nil, err
	}
	return json.Marshal(body)
}

// optionalMillis reads a seconds field into milliseconds, nil when absent.
func optionalMillis(form *multipart.Form, field string, minSeconds float64) (*int, error) {
	raw := formValue(form, field)
	if raw == "" {
		return nil, nil
	}
	seconds, err := strconv.ParseFloat(raw, 64)
	if err != nil || seconds < minSeconds {
		return nil, fmt.Errorf("the '%s' field must be a number of at least %g", field, minSeconds)
	}
	millis := int(math.Round(seconds * millisPerSecond))
	return &millis, nil
}

// videoControlsFrom derives Mirelo's speech controls from the wanted track:
// any track but the mix needs the speech separated, and the sfx track only
// exists when speech is not preserved, so that is the default for it.
func videoControlsFrom(form *multipart.Form, output string) (*videoControls, error) {
	controls := &videoControls{SeparateSpeech: output != OutputMix}
	if raw := formValue(form, fieldPreserveSpeech); raw != "" {
		preserve, err := strconv.ParseBool(raw)
		if err != nil {
			return nil, fmt.Errorf("the '%s' field must be true or false", fieldPreserveSpeech)
		}
		controls.PreserveSpeech = &preserve
	}
	if output == OutputSFX && controls.PreserveSpeech == nil {
		preserve := false
		controls.PreserveSpeech = &preserve
	}
	if controls.PreserveSpeech == nil && !controls.SeparateSpeech {
		return nil, nil
	}
	return controls, nil
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
