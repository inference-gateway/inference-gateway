package elevenlabs

import (
	"encoding/json"
	"testing"

	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"

	types "github.com/inference-gateway/inference-gateway/providers/types"
)

// Fixtures shared by the translation tests.
const (
	testVoice        = "21m00Tcm4TlvDq8ikWAM"
	testModel        = "eleven_multilingual_v2"
	testSFXModel     = "eleven_text_to_sound_v2"
	testJobID        = "gen-abc123"
	testMediaURL     = "https://cdn.elevenlabs.io/renders/gen-abc123.mp4"
	testFallbackTime = 1700000000
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
			ResponseFormat: ptr(types.CreateSpeechRequestResponseFormatWav),
		})
		require.Error(t, err)
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

func TestSFX(t *testing.T) {
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
