package tests

import (
	"archive/zip"
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"

	elevenlabs "github.com/inference-gateway/inference-gateway/internal/elevenlabs"
	sfx "github.com/inference-gateway/inference-gateway/internal/sfx"
	constants "github.com/inference-gateway/inference-gateway/providers/constants"
)

// Fixtures for the ElevenLabs video-to-music tests.
const (
	elevenlabsVideoMusicModel = "music_v2_5"
	elevenlabsVideoName       = "clip.mov"
	elevenlabsVideoType       = "video/quicktime"
	elevenlabsVideoBytes      = "FAKE-VIDEO-BYTES"
	elevenlabsMixBytes        = "FAKE-SCORED-MIX"
	elevenlabsEffectsBytes    = "FAKE-SFX-TRACK"
	elevenlabsMusicTrackBytes = "FAKE-MUSIC-TRACK"
	elevenlabsMixFormat       = "output_format=mp3_44100_128"
	elevenlabsZIPType         = "application/zip"
	elevenlabsEffectsEntry    = "sfx.mp3"
	elevenlabsMusicEntry      = "music.mp3"
	elevenlabsSeparateTracks  = "separate_tracks"
)

// elevenlabsVideoUpstream stands in for ElevenLabs' video-to-music endpoint
// and records what the gateway sent it.
type elevenlabsVideoUpstream struct {
	server    *httptest.Server
	query     string
	fields    map[string]string
	clipName  string
	clipType  string
	clipBytes string
	clips     int
}

func newElevenlabsVideoUpstream(t *testing.T) *elevenlabsVideoUpstream {
	t.Helper()
	u := &elevenlabsVideoUpstream{fields: map[string]string{}}
	u.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPost, r.Method)
		require.Equal(t, constants.ElevenlabsSFXVideoEndpoint, r.URL.Path)
		require.Equal(t, elevenlabsTestKey, r.Header.Get(elevenlabsAPIKey), "the call carries the provider key")
		u.query = r.URL.RawQuery
		require.NoError(t, r.ParseMultipartForm(1<<20))
		for field, values := range r.MultipartForm.Value {
			u.fields[field] = values[0]
		}
		clips := r.MultipartForm.File[elevenlabs.VideoClipPart]
		u.clips = len(clips)
		require.NotEmpty(t, clips, "the clip must reach the provider")
		u.clipName = clips[0].Filename
		u.clipType = clips[0].Header.Get("Content-Type")
		file, err := clips[0].Open()
		require.NoError(t, err)
		defer file.Close()
		content, err := io.ReadAll(file)
		require.NoError(t, err)
		u.clipBytes = string(content)

		if u.fields[elevenlabsSeparateTracks] == "true" {
			w.Header().Set("Content-Type", elevenlabsZIPType)
			_, _ = w.Write(elevenlabsTrackArchive(t))
			return
		}
		w.Header().Set("Content-Type", "audio/mpeg")
		_, _ = w.Write([]byte(elevenlabsMixBytes))
	}))
	t.Cleanup(u.server.Close)
	return u
}

// elevenlabsTrackArchive is the ZIP of separated tracks a separate_tracks
// request answers with.
func elevenlabsTrackArchive(t *testing.T) []byte {
	t.Helper()
	archive := &bytes.Buffer{}
	zw := zip.NewWriter(archive)
	for name, content := range map[string]string{
		elevenlabsMusicEntry:   elevenlabsMusicTrackBytes,
		elevenlabsEffectsEntry: elevenlabsEffectsBytes,
	} {
		entry, err := zw.Create(name)
		require.NoError(t, err)
		_, err = io.WriteString(entry, content)
		require.NoError(t, err)
	}
	require.NoError(t, zw.Close())
	return archive.Bytes()
}

func elevenlabsVideoFields(extra ...imagesMultipartField) []imagesMultipartField {
	return append([]imagesMultipartField{
		{name: "model", value: "elevenlabs/" + elevenlabsVideoMusicModel},
		{name: "video", filename: elevenlabsVideoName, value: elevenlabsVideoBytes},
	}, extra...)
}

// TestSFXHandler_ElevenlabsScoresAVideo proves the clip reaches ElevenLabs'
// video-to-music endpoint with the sfx cues enabled and the mixed audio comes
// back as raw bytes.
func TestSFXHandler_ElevenlabsScoresAVideo(t *testing.T) {
	u := newElevenlabsVideoUpstream(t)

	w := postSFXVideo(t, u.server.URL, elevenlabsVideoFields(
		imagesMultipartField{name: "prompt", value: "soft keyboard taps"},
	))

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Equal(t, elevenlabsMixBytes, w.Body.String())
	assert.Equal(t, "audio/mpeg", w.Header().Get("Content-Type"))

	assert.Equal(t, elevenlabsMixFormat, u.query)
	assert.Equal(t, elevenlabsVideoMusicModel, u.fields["model_id"], "the provider prefix must be stripped")
	assert.Equal(t, "true", u.fields["add_sfx"], "the sfx endpoint always asks for the sound cues")
	assert.Equal(t, "soft keyboard taps", u.fields["description"], "the prompt guides the score")
	assert.NotContains(t, u.fields, elevenlabsSeparateTracks, "the mix is one file")
	assert.Equal(t, 1, u.clips)
	assert.Equal(t, elevenlabsVideoName, u.clipName)
	assert.Equal(t, elevenlabsVideoType, u.clipType, "a .mov is announced as QuickTime even when sniffing cannot tell")
	assert.Equal(t, elevenlabsVideoBytes, u.clipBytes)
}

// TestSFXHandler_ElevenlabsAnswersTheEffectsTrack proves asking for the sound
// effects alone asks ElevenLabs for separate tracks and answers with the
// effects file out of the archive, leaving the music behind.
func TestSFXHandler_ElevenlabsAnswersTheEffectsTrack(t *testing.T) {
	u := newElevenlabsVideoUpstream(t)

	w := postSFXVideo(t, u.server.URL, elevenlabsVideoFields(
		imagesMultipartField{name: sfx.FieldOutput, value: sfx.OutputSFX},
	))

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Equal(t, "true", u.fields[elevenlabsSeparateTracks])
	assert.Equal(t, elevenlabsEffectsBytes, w.Body.String())
	assert.Equal(t, "audio/mpeg", w.Header().Get("Content-Type"), "the unpacked track is announced as audio, not a ZIP")
}

// TestSFXHandler_ElevenlabsRefusesUnservableFields proves a field ElevenLabs
// has no counterpart for is refused by name instead of being dropped.
func TestSFXHandler_ElevenlabsRefusesUnservableFields(t *testing.T) {
	for _, field := range []imagesMultipartField{
		{name: sfx.FieldPreserveSpeech, value: "true"},
		{name: sfx.FieldStartOffsetSeconds, value: "3"},
		{name: sfx.FieldDurationSeconds, value: "5"},
		{name: sfx.FieldOutput, value: sfx.OutputSpeech},
		{name: sfx.FieldResponseFormat, value: "flac"},
	} {
		t.Run(field.name+"="+field.value, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				t.Error("the request must be rejected before it reaches the provider")
			}))
			defer server.Close()

			w := postSFXVideo(t, server.URL, elevenlabsVideoFields(field))

			require.Equal(t, http.StatusBadRequest, w.Code)
			assert.Contains(t, w.Body.String(), field.name, "the caller must learn which field was refused")
		})
	}
}
