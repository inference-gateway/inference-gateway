package elevenlabs

import (
	"archive/zip"
	"bytes"
	"cmp"
	"fmt"
	"io"
	"mime/multipart"
	"strings"

	sfx "github.com/inference-gateway/inference-gateway/internal/sfx"
	types "github.com/inference-gateway/inference-gateway/providers/types"
)

const (
	// VideoClipPart names the multipart part ElevenLabs takes each clip of a
	// video-to-music request in.
	VideoClipPart = "videos"

	fieldModelID        = "model_id"
	fieldDescription    = "description"
	fieldAddSFX         = "add_sfx"
	fieldSeparateTracks = "separate_tracks"
	enabled             = "true"
	effectsTrackMarker  = "sfx"

	// maxTrackBytes caps how much of one unpacked track is read.
	maxTrackBytes = 64 << 20
)

// unscorableFields are the gateway video-to-sfx fields ElevenLabs'
// video-to-music request has no counterpart for. They are refused rather than
// dropped, so a caller never gets audio that ignores what it asked for.
var unscorableFields = []string{sfx.FieldPreserveSpeech, sfx.FieldStartOffsetSeconds, sfx.FieldDurationSeconds}

var trackContentTypes = map[string]string{
	string(types.CreateSFXRequestResponseFormatMp3):  "audio/mpeg",
	string(types.CreateSFXRequestResponseFormatOpus): "audio/ogg",
	string(types.CreateSFXRequestResponseFormatPcm):  "audio/L16",
}

// VideoMusic is the ElevenLabs video-to-music request a gateway video-to-sfx
// request becomes: the form fields that ride beside the clips, the query
// carrying the audio container, and, when only the sound effects were asked
// for, the Content-Type of the track unpacked from the answered archive.
type VideoMusic struct {
	Fields         map[string]string
	Query          string
	ContentType    string
	SeparateTracks bool
}

// SFXFromVideo rewrites a gateway video-to-sfx form into the ElevenLabs
// video-to-music request: the clip is scored with the sound effects it cues
// layered in, and the `sfx` track alone is asked for as a separate track.
// model is the request model with the provider prefix already stripped.
func SFXFromVideo(model string, form *multipart.Form) (VideoMusic, error) {
	for _, field := range unscorableFields {
		if formValue(form, field) != "" {
			return VideoMusic{}, fmt.Errorf("elevenlabs does not support '%s' when scoring a video, omit the field", field)
		}
	}

	output := cmp.Or(formValue(form, sfx.FieldOutput), sfx.OutputMix)
	if output != sfx.OutputMix && output != sfx.OutputSFX {
		return VideoMusic{}, fmt.Errorf("elevenlabs does not support '%s' %q when scoring a video, supported outputs: %s, %s", sfx.FieldOutput, output, sfx.OutputMix, sfx.OutputSFX)
	}

	responseFormat := cmp.Or(formValue(form, sfx.FieldResponseFormat), string(types.CreateSFXRequestResponseFormatMp3))
	format, err := OutputFormat(responseFormat)
	if err != nil {
		return VideoMusic{}, err
	}

	music := VideoMusic{
		Fields:         map[string]string{fieldModelID: model, fieldAddSFX: enabled},
		Query:          "output_format=" + format,
		ContentType:    trackContentTypes[responseFormat],
		SeparateTracks: output == sfx.OutputSFX,
	}
	if prompt := formValue(form, sfx.FieldPrompt); prompt != "" {
		music.Fields[fieldDescription] = prompt
	}
	if music.SeparateTracks {
		music.Fields[fieldSeparateTracks] = enabled
	}
	return music, nil
}

// EffectsTrack unpacks the sound-effects track from the archive ElevenLabs
// answers a separate-tracks request with, leaving the music track behind.
func EffectsTrack(archive []byte) ([]byte, error) {
	reader, err := zip.NewReader(bytes.NewReader(archive), int64(len(archive)))
	if err != nil {
		return nil, fmt.Errorf("failed to read the elevenlabs track archive: %w", err)
	}
	for _, file := range reader.File {
		if !strings.Contains(strings.ToLower(file.Name), effectsTrackMarker) {
			continue
		}
		track, err := file.Open()
		if err != nil {
			return nil, fmt.Errorf("failed to open the elevenlabs sound-effects track: %w", err)
		}
		defer track.Close()
		return io.ReadAll(io.LimitReader(track, maxTrackBytes))
	}
	return nil, fmt.Errorf("the elevenlabs track archive carries no sound-effects track")
}
