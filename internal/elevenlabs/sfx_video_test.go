package elevenlabs

import (
	"archive/zip"
	"bytes"
	"io"
	"testing"

	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

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
