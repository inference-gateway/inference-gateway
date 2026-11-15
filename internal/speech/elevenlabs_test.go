package speech

import (
	"encoding/json"
	"testing"

	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"

	types "github.com/inference-gateway/inference-gateway/providers/types"
)

// Fixtures shared by the translation tests.
const (
	testVoice = "21m00Tcm4TlvDq8ikWAM"
	testModel = "eleven_multilingual_v2"
)

func ptr[T any](v T) *T { return &v }

func TestOutputFormat(t *testing.T) {
	tests := []struct {
		name           string
		responseFormat string
		want           string
		wantErr        bool
	}{
		{name: "empty defaults to mp3", responseFormat: "", want: "mp3_44100_128"},
		{name: "mp3", responseFormat: "mp3", want: "mp3_44100_128"},
		{name: "opus", responseFormat: "opus", want: "opus_48000_128"},
		{name: "pcm", responseFormat: "pcm", want: "pcm_24000"},
		{name: "wav is rejected rather than downgraded", responseFormat: "wav", wantErr: true},
		{name: "aac is rejected rather than downgraded", responseFormat: "aac", wantErr: true},
		{name: "flac is rejected rather than downgraded", responseFormat: "flac", wantErr: true},
		{name: "unknown format", responseFormat: "ogg", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := OutputFormat(tt.responseFormat)
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), SupportedFormats, "the error must list what ElevenLabs can serve")
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestSpeech(t *testing.T) {
	t.Run("rewrites the request into the text-to-speech shape", func(t *testing.T) {
		path, query, body, err := Speech(VoicePathPlaceholder, testModel, types.CreateSpeechRequest{
			Input:          "Hello world",
			Voice:          testVoice,
			Model:          "elevenlabs/" + testModel,
			Speed:          ptr(float32(1.25)),
			Language:       ptr("de"),
			ResponseFormat: ptr(types.CreateSpeechRequestResponseFormatOpus),
		})
		require.NoError(t, err)

		assert.Equal(t, testVoice, path, "the voice belongs in the URL path, not the body")
		assert.Equal(t, "output_format=opus_48000_128", query)

		var got map[string]any
		require.NoError(t, json.Unmarshal(body, &got))
		assert.Equal(t, "Hello world", got["text"], "OpenAI input maps onto ElevenLabs text")
		assert.Equal(t, testModel, got["model_id"], "the provider prefix must already be stripped")
		assert.Equal(t, "de", got["language_code"])
		assert.Equal(t, map[string]any{"speed": 1.25}, got["voice_settings"])
	})

	t.Run("omits voice_settings when no speed is requested", func(t *testing.T) {
		_, _, body, err := Speech(VoicePathPlaceholder, testModel, types.CreateSpeechRequest{
			Input: "Hello",
			Voice: testVoice,
		})
		require.NoError(t, err)

		var got map[string]any
		require.NoError(t, json.Unmarshal(body, &got))
		assert.NotContains(t, got, "voice_settings")
		assert.NotContains(t, got, "language_code")
	})

	t.Run("requires a voice", func(t *testing.T) {
		_, _, _, err := Speech(VoicePathPlaceholder, testModel, types.CreateSpeechRequest{Input: "Hello", Voice: "  "})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "voice")
	})

	t.Run("rejects an unsupported response_format", func(t *testing.T) {
		_, _, _, err := Speech(VoicePathPlaceholder, testModel, types.CreateSpeechRequest{
			Input:          "Hello",
			Voice:          testVoice,
			ResponseFormat: ptr(types.CreateSpeechRequestResponseFormatFlac),
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "wav", "the error must list wav, which speech can serve")
	})

	t.Run("asks for a wav container when the caller wants wav", func(t *testing.T) {
		_, query, _, err := Speech(VoicePathPlaceholder, testModel, types.CreateSpeechRequest{
			Input:          "Hello",
			Voice:          testVoice,
			ResponseFormat: ptr(types.CreateSpeechRequestResponseFormatWav),
		})
		require.NoError(t, err)
		assert.Equal(t, "output_format="+SpeechWavFormat, query)
	})

	t.Run("a crafted voice cannot rewrite the upstream path", func(t *testing.T) {
		path, _, _, err := Speech("/text-to-speech/"+VoicePathPlaceholder, testModel, types.CreateSpeechRequest{
			Input: "Hello",
			Voice: "../../v1/user?x=1#frag",
		})
		require.NoError(t, err)
		assert.Equal(t, "/text-to-speech/..%2F..%2Fv1%2Fuser%3Fx=1%23frag", path,
			"the voice stays inside its own path segment")
	})
}
