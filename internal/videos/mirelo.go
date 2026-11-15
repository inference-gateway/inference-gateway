package videos

import (
	"cmp"
	"encoding/json"
	"mime/multipart"

	sfx "github.com/inference-gateway/inference-gateway/internal/sfx"
	types "github.com/inference-gateway/inference-gateway/providers/types"
)

const (
	// fieldSeconds is the Videos API duration field, the window to score.
	fieldSeconds = "seconds"
)

// videoJobStatuses maps a Mirelo job status onto the gateway's VideoJob status.
// Mirelo's status list is open, so an unknown status counts as still running.
var videoJobStatuses = map[string]types.VideoJobStatus{
	"queued":              types.VideoJobStatusInProgress,
	"running":             types.VideoJobStatusInProgress,
	sfx.StatusSucceeded:   types.VideoJobStatusCompleted,
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
	body := sfx.VideoBody{
		Model:  model,
		Input:  sfx.VideoInput{Video: sfx.AssetRef(assetID), Prompt: formValue(form, sfx.FieldPrompt)},
		Output: sfx.SFXOutput{Media: sfx.MediaVideo},
	}

	var err error
	if body.DurationMs, err = sfx.OptionalMillis(form, fieldSeconds, sfx.MinVideoSeconds); err != nil {
		return nil, err
	}
	if body.StartOffsetMs, err = sfx.OptionalMillis(form, sfx.FieldStartOffsetSeconds, sfx.MinOffsetSeconds); err != nil {
		return nil, err
	}
	if body.Controls, err = sfx.VideoControlsFrom(form, sfx.OutputMix); err != nil {
		return nil, err
	}
	return json.Marshal(body)
}

// VideoJob maps a Mirelo generation job onto the gateway's VideoJob and returns
// the URL the dubbed clip can be downloaded from, empty until it is rendered.
// fallbackModel is used when the job does not echo the model back.
func VideoJob(raw []byte, fallbackModel string, createdAt int) (types.VideoJob, string, error) {
	job, err := sfx.ParseJob(raw)
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
		message := job.ErrorMessage()
		mapped.Error = &struct {
			Code    *string `json:"code,omitempty"`
			Message *string `json:"message,omitempty"`
		}{Message: &message}
		return mapped, "", nil
	}
	if status != types.VideoJobStatusCompleted {
		return mapped, "", nil
	}
	return mapped, job.VideoURL(), nil
}
