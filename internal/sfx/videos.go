package sfx

import (
	"cmp"
	"encoding/json"
	"mime/multipart"

	types "github.com/inference-gateway/inference-gateway/providers/types"
)

const (
	// mediaVideo asks Mirelo to re-render the clip with the generated sound as
	// its new soundtrack instead of answering with the audio alone.
	mediaVideo = "video"

	// fieldSeconds is the Videos API duration field, the window to score.
	fieldSeconds = "seconds"
)

// videoJobStatuses maps a Mirelo job status onto the gateway's VideoJob status.
// Mirelo's status list is open, so an unknown status counts as still running.
var videoJobStatuses = map[string]types.VideoJobStatus{
	"queued":              types.VideoJobStatusInProgress,
	"running":             types.VideoJobStatusInProgress,
	statusSucceeded:       types.VideoJobStatusCompleted,
	"partially_succeeded": types.VideoJobStatusCompleted,
	"failed":              types.VideoJobStatusFailed,
	"canceled":            types.VideoJobStatusFailed,
	"expired":             types.VideoJobStatusFailed,
}

// Dub rewrites an OpenAI Videos multipart form into the Mirelo video-to-sfx
// shape that answers with the dubbed clip. Only the generated mix is laid onto
// the clip - the separated tracks are audio and stay on /audio/sfx - so the
// form picks the window and whether the clip's own speech is preserved.
// model is the request model with the provider prefix already stripped.
func Dub(model string, form *multipart.Form, assetID string) ([]byte, error) {
	body := videoBody{
		Model:  model,
		Input:  videoInput{Video: mediaRef{Type: mediaTypeAsset, ID: assetID}, Prompt: formValue(form, FieldPrompt)},
		Output: sfxOutput{Media: mediaVideo},
	}

	var err error
	if body.DurationMs, err = optionalMillis(form, fieldSeconds, minVideoSeconds); err != nil {
		return nil, err
	}
	if body.StartOffsetMs, err = optionalMillis(form, FieldStartOffsetSeconds, minOffsetSeconds); err != nil {
		return nil, err
	}
	if body.Controls, err = videoControlsFrom(form, OutputMix); err != nil {
		return nil, err
	}
	return json.Marshal(body)
}

// VideoJob maps a Mirelo generation job onto the gateway's VideoJob and returns
// the URL the dubbed clip can be downloaded from, empty until it is rendered.
// fallbackModel is used when the job does not echo the model back.
func VideoJob(raw []byte, fallbackModel string, createdAt int) (types.VideoJob, string, error) {
	job, err := ParseJob(raw)
	if err != nil {
		return types.VideoJob{}, "", err
	}

	status, known := videoJobStatuses[job.Status]
	if !known {
		status = types.VideoJobStatusInProgress
	}

	mapped := types.VideoJob{
		ID:        job.ID,
		Object:    types.VideoJobObjectVideo,
		Model:     cmp.Or(job.Model, fallbackModel),
		Status:    status,
		CreatedAt: createdAt,
	}
	if status == types.VideoJobStatusFailed {
		message := job.errorMessage()
		mapped.Error = &struct {
			Code    *string `json:"code,omitempty"`
			Message *string `json:"message,omitempty"`
		}{Message: &message}
		return mapped, "", nil
	}
	if status != types.VideoJobStatusCompleted {
		return mapped, "", nil
	}
	return mapped, job.videoURL(), nil
}

// videoURL is the download URL of the first rendered video the job carries.
func (j Job) videoURL() string {
	for _, t := range j.tracks() {
		for _, variant := range t.Variants {
			if variant.Files != nil && variant.Files.Video != nil && variant.Files.Video.URL != "" {
				return variant.Files.Video.URL
			}
		}
	}
	return ""
}
