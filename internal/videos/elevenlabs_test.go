package videos

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/textproto"
	"testing"

	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"

	types "github.com/inference-gateway/inference-gateway/providers/types"
)

// Fixtures shared by the translation tests.
const (
	testModel        = "eleven_multilingual_v2"
	testJobID        = "gen-abc123"
	testMediaURL     = "https://cdn.elevenlabs.io/renders/gen-abc123.mp4"
	testFallbackTime = 1700000000

	testSeedanceModel = "bytedance-seedance-v2"
	testVeoModel      = "veo-3.1-generate-001"
	testAvatarModel   = "creatify-aurora"
	testPrompt        = "the same woman walking through a market"
	testImageBytes    = "FAKE-IMAGE-BYTES"
	testFormMaxMemory = 1 << 20
)

// newForm parses a real multipart body, since FileHeader.Open only works on a
// header that came out of ReadForm. Each value in files becomes one file part.
func newForm(t *testing.T, fields map[string]string, files map[string]int) *multipart.Form {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	for name, value := range fields {
		require.NoError(t, mw.WriteField(name, value))
	}
	for name, n := range files {
		for range n {
			fw, err := mw.CreateFormFile(name, name+".png")
			require.NoError(t, err)
			_, err = fw.Write([]byte(testImageBytes))
			require.NoError(t, err)
		}
	}
	require.NoError(t, mw.Close())
	form, err := multipart.NewReader(&buf, mw.Boundary()).ReadForm(testFormMaxMemory)
	require.NoError(t, err)
	t.Cleanup(func() { _ = form.RemoveAll() })
	return form
}

func TestJob(t *testing.T) {
	t.Run("maps a queued job and reports no download url", func(t *testing.T) {
		job, downloadURL, err := Job([]byte(`{"generation_id":"`+testJobID+`","status":"pending","progress":0}`), testModel, testFallbackTime)
		require.NoError(t, err)

		assert.Equal(t, testJobID, job.ID)
		assert.Equal(t, types.VideoJobObjectVideo, job.Object)
		assert.Equal(t, types.VideoJobStatusQueued, job.Status)
		assert.Equal(t, testModel, job.Model, "an unechoed model falls back to the requested one")
		assert.Equal(t, testFallbackTime, job.CreatedAt, "a payload without a timestamp falls back to the caller's")
		assert.Nil(t, job.CompletedAt)
		assert.Nil(t, job.Error)
		assert.Empty(t, downloadURL)
	})

	t.Run("maps a completed job and returns the download url", func(t *testing.T) {
		job, downloadURL, err := Job([]byte(`{"id":"`+testJobID+`","state":"succeeded","model_id":"creatify-aurora","created_at_unix":1699999999,"completed_at_unix":1700000001,"media_url":"`+testMediaURL+`"}`), testModel, testFallbackTime)
		require.NoError(t, err)

		assert.Equal(t, types.VideoJobStatusCompleted, job.Status)
		assert.Equal(t, "creatify-aurora", job.Model, "an echoed model wins over the fallback")
		assert.Equal(t, 1699999999, job.CreatedAt)
		require.NotNil(t, job.CompletedAt)
		assert.Equal(t, 1700000001, *job.CompletedAt)
		assert.Equal(t, testMediaURL, downloadURL)
	})

	t.Run("maps a failed job onto the error envelope", func(t *testing.T) {
		job, downloadURL, err := Job([]byte(`{"id":"`+testJobID+`","status":"error","error_message":"the reference image is unusable"}`), testModel, testFallbackTime)
		require.NoError(t, err)

		assert.Equal(t, types.VideoJobStatusFailed, job.Status)
		require.NotNil(t, job.Error)
		require.NotNil(t, job.Error.Message)
		assert.Equal(t, "the reference image is unusable", *job.Error.Message)
		assert.Empty(t, downloadURL, "a failed job has no bytes to download")
	})

	t.Run("an unrecognized state keeps the client polling", func(t *testing.T) {
		job, _, err := Job([]byte(`{"id":"`+testJobID+`","status":"rendering_audio"}`), testModel, testFallbackTime)
		require.NoError(t, err)
		assert.Equal(t, types.VideoJobStatusInProgress, job.Status)
	})

	t.Run("rejects a payload with no id", func(t *testing.T) {
		_, _, err := Job([]byte(`{"status":"queued"}`), testModel, testFallbackTime)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "generation id")
	})

	t.Run("rejects an undecodable payload", func(t *testing.T) {
		_, _, err := Job([]byte(`not json`), testModel, testFallbackTime)
		require.Error(t, err)
	})
}

func TestVideoReferenceImages(t *testing.T) {
	prompt := map[string]string{videoFieldPrompt: testPrompt}
	inlined := map[string]any{
		"type":           inlineMediaType,
		"content_base64": base64.StdEncoding.EncodeToString([]byte(testImageBytes)),
		"mime_type":      "text/plain; charset=utf-8",
	}

	tests := []struct {
		name       string
		model      string
		fields     map[string]string
		files      map[string]int
		wantImages []any
		wantErr    string
	}{
		{
			name:       "seedance takes each image as-is",
			model:      testSeedanceModel,
			fields:     prompt,
			files:      map[string]int{videoFieldRefImages: 2},
			wantImages: []any{inlined, inlined},
		},
		{
			name:       "veo wraps each image as a subject",
			model:      testVeoModel,
			fields:     prompt,
			files:      map[string]int{videoFieldRefImages: 1},
			wantImages: []any{map[string]any{"image": inlined, "role": veoRoleSubject}},
		},
		{
			name:    "more images than the model accepts",
			model:   testVeoModel,
			fields:  prompt,
			files:   map[string]int{videoFieldRefImages: referenceImageLimits[testVeoModel] + 1},
			wantErr: "at most",
		},
		{
			name:    "a model without reference-image support",
			model:   testAvatarModel,
			fields:  prompt,
			files:   map[string]int{videoFieldRefImages: 1},
			wantErr: "does not accept",
		},
		{
			name:    "cannot be combined with a first frame",
			model:   testSeedanceModel,
			fields:  prompt,
			files:   map[string]int{videoFieldRefImages: 1, videoFieldReference: 1},
			wantErr: "cannot be combined",
		},
		{
			name:  "avatar renders ignore them",
			model: testAvatarModel,
			files: map[string]int{videoFieldRefImages: 1, videoFieldReference: 1, videoFieldAudio: 1},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			raw, err := Video(tt.model, newForm(t, tt.fields, tt.files))
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				return
			}
			require.NoError(t, err)
			var body map[string]any
			require.NoError(t, json.Unmarshal(raw, &body))
			if tt.wantImages == nil {
				assert.NotContains(t, body, "images")
				return
			}
			assert.Equal(t, tt.wantImages, body["images"])
			assert.NotContains(t, body, "start_frame")
		})
	}
}

// TestVideoLabelsAudioAsElevenlabsNamesIt proves a driving clip reaches
// ElevenLabs under a mime type it accepts: Go sniffs a WAV as audio/wave,
// which ElevenLabs refuses.
func TestVideoLabelsAudioAsElevenlabsNamesIt(t *testing.T) {
	const (
		wavBytes      = "RIFF\x24\x00\x00\x00WAVEfmt "
		mp3Bytes      = "ID3\x03\x00\x00\x00\x00\x00\x0a"
		octetStream   = "application/octet-stream"
		acceptedWav   = "audio/wav"
		acceptedMpeg  = "audio/mpeg"
		contentHeader = "Content-Type"
	)

	tests := []struct {
		name        string
		contentType string
		audio       string
		want        string
	}{
		{"an untyped wav is sniffed", octetStream, wavBytes, acceptedWav},
		{"an untyped mp3 is sniffed", octetStream, mp3Bytes, acceptedMpeg},
		{"a wav the client calls audio/wave", "audio/wave", wavBytes, acceptedWav},
		{"a wav the client calls audio/x-wav", "audio/x-wav", wavBytes, acceptedWav},
		{"an mp3 the client calls audio/mp3", "audio/mp3", mp3Bytes, acceptedMpeg},
		{"a type elevenlabs accepts is kept", acceptedWav, wavBytes, acceptedWav},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			mw := multipart.NewWriter(&buf)
			for _, part := range []struct{ field, file, contentType, content string }{
				{videoFieldReference, "portrait.png", "image/png", testImageBytes},
				{videoFieldAudio, "line.wav", tt.contentType, tt.audio},
			} {
				header := textproto.MIMEHeader{}
				header.Set("Content-Disposition", fmt.Sprintf(`form-data; name=%q; filename=%q`, part.field, part.file))
				header.Set(contentHeader, part.contentType)
				pw, err := mw.CreatePart(header)
				require.NoError(t, err)
				_, err = pw.Write([]byte(part.content))
				require.NoError(t, err)
			}
			require.NoError(t, mw.Close())
			form, err := multipart.NewReader(&buf, mw.Boundary()).ReadForm(testFormMaxMemory)
			require.NoError(t, err)
			t.Cleanup(func() { _ = form.RemoveAll() })

			raw, err := Video(testAvatarModel, form)

			require.NoError(t, err)
			var body struct {
				Audio struct {
					MimeType string `json:"mime_type"`
				} `json:"audio"`
			}
			require.NoError(t, json.Unmarshal(raw, &body))
			assert.Equal(t, tt.want, body.Audio.MimeType)
		})
	}
}
