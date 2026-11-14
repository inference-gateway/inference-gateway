// Package sfx translates a gateway sound-effect request into the job a
// provider's text-to-sfx API takes, Mirelo today, and reads the job back.
package sfx

import (
	"encoding/json"
	"fmt"
	"maps"
	"math"
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

	// MaxJobRequests bounds the create call plus the long-polls that follow it.
	MaxJobRequests = 6

	maxWaitSeconds    = 25
	minWaitSeconds    = 1
	waitMarginSeconds = 5
	waitQueryPrefix   = "wait="
	millisPerSecond   = 1000
	formatSeparator   = "_"
	statusSucceeded   = "succeeded"
	unknownJobError   = "no error was reported"
)

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

// WaitQuery is the long-poll query Mirelo holds a job request open for: its
// 25 second ceiling, shortened to stay inside the gateway's upstream timeout.
func WaitQuery(upstreamTimeout time.Duration) string {
	seconds := int(upstreamTimeout.Seconds()) - waitMarginSeconds
	return waitQueryPrefix + strconv.Itoa(min(max(seconds, minWaitSeconds), maxWaitSeconds))
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
		Output struct {
			Variants []struct {
				Files *struct {
					Audio *struct {
						URL    string `json:"url"`
						Format string `json:"format"`
					} `json:"audio"`
				} `json:"files"`
			} `json:"variants"`
		} `json:"output"`
	} `json:"result"`
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

// Audio returns the download URL and Content-Type of the first audio file the
// job rendered, so a partially_succeeded job still relays what it produced. A
// job without a file is an error carrying Mirelo's own message.
func (j Job) Audio() (audioURL, contentType string, err error) {
	if j.Result != nil {
		for _, variant := range j.Result.Output.Variants {
			if variant.Files != nil && variant.Files.Audio != nil && variant.Files.Audio.URL != "" {
				container, _, _ := strings.Cut(variant.Files.Audio.Format, formatSeparator)
				return variant.Files.Audio.URL, contentTypes[container], nil
			}
		}
	}
	if j.Status != statusSucceeded {
		return "", "", fmt.Errorf("mirelo sound effect generation %s: %s", j.Status, j.errorMessage())
	}
	return "", "", fmt.Errorf("mirelo sound effect generation succeeded without an audio file")
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
