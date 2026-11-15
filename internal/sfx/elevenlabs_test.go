package sfx

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"io"
	"testing"

	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"

	types "github.com/inference-gateway/inference-gateway/providers/types"
)

// Fixtures shared by the translation tests.
const testSFXModel = "eleven_text_to_sound_v2"

func TestElevenlabsSFX(t *testing.T) {
	t.Run("rewrites the request into the sound-generation shape", func(t *testing.T) {
		body, err := SFX(testSFXModel, types.CreateSFXRequest{
			Prompt:          "distant thunder rolling over a valley",
			Model:           "elevenlabs/" + testSFXModel,
			DurationSeconds: ptr(float32(6)),
			PromptInfluence: ptr(float32(0.4)),
			Loop:            ptr(true),
		})
		require.NoError(t, err)

		var got map[string]any
		require.NoError(t, json.Unmarshal(body, &got))
		assert.Equal(t, "distant thunder rolling over a valley", got["text"])
		assert.Equal(t, testSFXModel, got["model_id"])
		assert.Equal(t, "mp3_44100_128", got["output_format"], "an omitted response_format defaults to mp3")
		assert.Equal(t, float64(6), got["duration_seconds"])
		assert.Equal(t, 0.4, got["prompt_influence"])
		assert.Equal(t, true, got["loop"])
	})

	t.Run("omits the optional knobs when unset", func(t *testing.T) {
		body, err := SFX(testSFXModel, types.CreateSFXRequest{Prompt: "rain on a tin roof"})
		require.NoError(t, err)

		var got map[string]any
		require.NoError(t, json.Unmarshal(body, &got))
		assert.NotContains(t, got, "duration_seconds")
		assert.NotContains(t, got, "prompt_influence")
		assert.NotContains(t, got, "loop")
	})

	t.Run("requires a prompt", func(t *testing.T) {
		_, err := SFX(testSFXModel, types.CreateSFXRequest{Prompt: " "})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "prompt")
	})

	t.Run("rejects an unsupported response_format", func(t *testing.T) {
		_, err := SFX(testSFXModel, types.CreateSFXRequest{
			Prompt:         "thunder",
			ResponseFormat: ptr(types.CreateSFXRequestResponseFormatFlac),
		})
		require.Error(t, err)
	})
}

func trackArchive(t *testing.T, entries map[string]string) []byte {
	t.Helper()
	archive := &bytes.Buffer{}
	zw := zip.NewWriter(archive)
	for name, content := range entries {
		entry, err := zw.Create(name)
		require.NoError(t, err)
		_, err = io.WriteString(entry, content)
		require.NoError(t, err)
	}
	require.NoError(t, zw.Close())
	return archive.Bytes()
}

func TestEffectsTrack(t *testing.T) {
	const effects = "SFX-BYTES"

	track, err := EffectsTrack(trackArchive(t, map[string]string{"music.mp3": "MUSIC", "song_sfx.mp3": effects}))
	require.NoError(t, err)
	assert.Equal(t, effects, string(track), "the effects track is picked out of the archive")
}

func TestEffectsTrackRejectsAnArchiveWithoutEffects(t *testing.T) {
	_, err := EffectsTrack(trackArchive(t, map[string]string{"music.mp3": "MUSIC"}))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no sound-effects track")
}

func TestEffectsTrackRejectsAnUnreadableArchive(t *testing.T) {
	_, err := EffectsTrack([]byte("not a zip"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "archive")
}
