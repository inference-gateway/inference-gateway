package sfx

import (
	"encoding/base64"
	"testing"

	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

const (
	testClip          = "FAKE-VIDEO-BYTES"
	testClipType      = "video/mp4"
	testQuickTimeType = "video/quicktime"
)

func TestParseVideo(t *testing.T) {
	encoded := base64.StdEncoding.EncodeToString([]byte(testClip))

	t.Run("an https URL is kept for the provider to fetch", func(t *testing.T) {
		video, err := ParseVideo(testVideoURL)
		require.NoError(t, err)
		assert.Equal(t, Video{URL: testVideoURL}, video)
	})

	t.Run("a data URI is decoded with its media type", func(t *testing.T) {
		video, err := ParseVideo("data:" + testClipType + ";base64," + encoded)
		require.NoError(t, err)
		assert.Equal(t, Video{ContentType: testClipType, Content: []byte(testClip)}, video)
	})

	t.Run("media type parameters are left out of the content type", func(t *testing.T) {
		video, err := ParseVideo("data:" + testClipType + ";codecs=avc1;base64," + encoded)
		require.NoError(t, err)
		assert.Equal(t, testClipType, video.ContentType)
	})

	for name, raw := range map[string]string{
		"a plain http URL":          "http://cdn.example.com/scene.mov",
		"a URL without a host":      "https://",
		"something that is no URL":  "scene.mov",
		"an image data URI":         "data:image/png;base64," + encoded,
		"a data URI without base64": "data:" + testClipType + "," + testClip,
		"invalid base64":            "data:" + testClipType + ";base64,%%%",
		"an empty data URI":         "data:" + testClipType + ";base64,",
	} {
		t.Run("rejects "+name, func(t *testing.T) {
			_, err := ParseVideo(raw)
			require.ErrorIs(t, err, errInvalidVideo)
		})
	}
}

func TestVideoFilenameFollowsTheMediaType(t *testing.T) {
	assert.Equal(t, inlineVideoName+".mov", Video{ContentType: testQuickTimeType}.Filename())
	assert.Equal(t, inlineVideoName, Video{ContentType: "video/x-unknown"}.Filename(), "an unknown type gets no extension")
}
