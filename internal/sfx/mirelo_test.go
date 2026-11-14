package sfx

import (
	"encoding/json"
	"testing"
	"time"

	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"

	types "github.com/inference-gateway/inference-gateway/providers/types"
)

const (
	testModel    = "sfx-2.0"
	testPrompt   = "distant thunder rolling over a valley"
	testJobID    = "gen_abc123"
	testAudioURL = "https://cdn.mirelo.ai/out/gen_abc123.mp3"
)

func ptr[T any](v T) *T { return &v }

func TestOutputFormat(t *testing.T) {
	tests := []struct {
		name           string
		responseFormat string
		want           string
		wantErr        bool
	}{
		{name: "empty defaults to mp3", responseFormat: "", want: "mp3_128"},
		{name: "mp3", responseFormat: "mp3", want: "mp3_128"},
		{name: "flac", responseFormat: "flac", want: "flac"},
		{name: "pcm is rejected", responseFormat: "pcm", wantErr: true},
		{name: "opus is rejected", responseFormat: "opus", wantErr: true},
		{name: "aac is rejected", responseFormat: "aac", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := OutputFormat(tt.responseFormat)
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), SupportedFormats)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestSFX(t *testing.T) {
	t.Run("rewrites the request into the text-to-sfx shape", func(t *testing.T) {
		body, err := FromText(testModel, types.CreateSFXRequest{
			Prompt:          testPrompt,
			DurationSeconds: ptr(float32(2.5)),
			Loop:            ptr(true),
			ResponseFormat:  ptr(types.CreateSFXRequestResponseFormatFlac),
		})
		require.NoError(t, err)
		assert.JSONEq(t, `{
			"model": "sfx-2.0",
			"duration_ms": 2500,
			"input": {"prompt": "distant thunder rolling over a valley"},
			"controls": {"loop": true},
			"output": {"format": "flac"}
		}`, string(body))
	})

	t.Run("defaults the duration Mirelo requires", func(t *testing.T) {
		body, err := FromText(testModel, types.CreateSFXRequest{Prompt: testPrompt})
		require.NoError(t, err)

		var got sfxBody
		require.NoError(t, json.Unmarshal(body, &got))
		assert.Equal(t, DefaultDurationSeconds*millisPerSecond, got.DurationMs)
		assert.Nil(t, got.Controls)
	})

	t.Run("rejects an empty prompt", func(t *testing.T) {
		_, err := FromText(testModel, types.CreateSFXRequest{Prompt: " "})
		require.Error(t, err)
	})

	t.Run("rejects prompt_influence", func(t *testing.T) {
		_, err := FromText(testModel, types.CreateSFXRequest{Prompt: testPrompt, PromptInfluence: ptr(float32(0.5))})
		require.ErrorContains(t, err, "prompt_influence")
	})

	t.Run("rejects an unsupported format", func(t *testing.T) {
		_, err := FromText(testModel, types.CreateSFXRequest{Prompt: testPrompt, ResponseFormat: ptr(types.CreateSFXRequestResponseFormatPcm)})
		require.ErrorContains(t, err, SupportedFormats)
	})
}

func TestWaitQuery(t *testing.T) {
	assert.Equal(t, "wait=25", WaitQuery(30*time.Second))
	assert.Equal(t, "wait=25", WaitQuery(5*time.Minute))
	assert.Equal(t, "wait=5", WaitQuery(10*time.Second))
	assert.Equal(t, "wait=1", WaitQuery(2*time.Second))
}

func TestJobPath(t *testing.T) {
	assert.Equal(t, "/text-to-sfx/generations/gen%2Fabc", JobPath("/text-to-sfx/generations", "gen/abc"))
}

func TestJob(t *testing.T) {
	t.Run("a succeeded job yields its audio file", func(t *testing.T) {
		job, err := ParseJob([]byte(`{"id":"` + testJobID + `","status":"succeeded","errors":[],"result":{"output":{"variants":[
			{"index":0,"status":"failed","files":null},
			{"index":1,"status":"succeeded","files":{"audio":{"url":"` + testAudioURL + `","format":"mp3_128"}}}
		]}}}`))
		require.NoError(t, err)
		assert.False(t, job.Pending())

		audioURL, contentType, err := job.Audio()
		require.NoError(t, err)
		assert.Equal(t, testAudioURL, audioURL)
		assert.Equal(t, "audio/mpeg", contentType)
	})

	t.Run("queued, running and unknown statuses are pending", func(t *testing.T) {
		for _, status := range []string{"queued", "running", "post_processing"} {
			job, err := ParseJob([]byte(`{"id":"` + testJobID + `","status":"` + status + `","result":null}`))
			require.NoError(t, err)
			assert.True(t, job.Pending(), status)
		}
	})

	t.Run("a failed job surfaces Mirelo's message", func(t *testing.T) {
		for _, status := range []string{"failed", "expired", "partially_succeeded", "canceled"} {
			job, err := ParseJob([]byte(`{"id":"` + testJobID + `","status":"` + status + `","errors":[{"code":"moderation_blocked","message":"the prompt was blocked"}],"result":null}`))
			require.NoError(t, err)
			assert.False(t, job.Pending(), status)

			_, _, err = job.Audio()
			require.ErrorContains(t, err, status)
			require.ErrorContains(t, err, "the prompt was blocked")
		}
	})

	t.Run("a partially succeeded job relays the variant it rendered", func(t *testing.T) {
		job, err := ParseJob([]byte(`{"id":"` + testJobID + `","status":"partially_succeeded","errors":[{"code":"render_failed","message":"variant 0 failed"}],"result":{"output":{"variants":[
			{"index":0,"status":"failed","files":null},
			{"index":1,"status":"succeeded","files":{"audio":{"url":"` + testAudioURL + `","format":"mp3_128"}}}
		]}}}`))
		require.NoError(t, err)

		audioURL, contentType, err := job.Audio()
		require.NoError(t, err)
		assert.Equal(t, testAudioURL, audioURL)
		assert.Equal(t, "audio/mpeg", contentType)
	})

	t.Run("a succeeded job without a file is an error", func(t *testing.T) {
		job, err := ParseJob([]byte(`{"id":"` + testJobID + `","status":"succeeded","result":{"output":{"variants":[]}}}`))
		require.NoError(t, err)
		_, _, err = job.Audio()
		require.Error(t, err)
	})

	t.Run("a payload without an id is rejected", func(t *testing.T) {
		_, err := ParseJob([]byte(`{"status":"queued"}`))
		require.Error(t, err)
		_, err = ParseJob([]byte(`not json`))
		require.Error(t, err)
	})
}
