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
			got, err := outputFormat(tt.responseFormat)
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), supportedFormats)
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
		assert.Equal(t, defaultDurationSeconds*millisPerSecond, got.DurationMs)
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
		require.ErrorContains(t, err, supportedFormats)
	})
}

func TestPlan(t *testing.T) {
	assert.Equal(t, Polling{Query: "wait=25", Requests: 6}, Plan(30*time.Second, 0))
	assert.Equal(t, Polling{Query: "wait=25", Requests: 6}, Plan(5*time.Minute, time.Minute))
	assert.Equal(t, Polling{Query: "wait=5", Requests: 30}, Plan(30*time.Second, 10*time.Second), "the transport's header timeout caps the wait, the budget stays")
	assert.Equal(t, Polling{Query: "wait=1", Requests: 150}, Plan(2*time.Second, 0))
}

func TestJobPath(t *testing.T) {
	assert.Equal(t, "/text-to-sfx/generations/gen%2Fabc", JobPath("/text-to-sfx/generations", "gen/abc"))
}

func TestJob(t *testing.T) {
	t.Run("a succeeded job yields its audio file with the Content-Type of its format", func(t *testing.T) {
		tests := []struct {
			format          string
			wantContentType string
		}{
			{"mp3_128", "audio/mpeg"},
			{"wav", "audio/wav"},
			{"flac", "audio/flac"},
			{"ogg", ""},
		}
		for _, tt := range tests {
			t.Run(tt.format, func(t *testing.T) {
				job, err := ParseJob([]byte(`{"id":"` + testJobID + `","status":"succeeded","errors":[],"result":{"output":{"variants":[
					{"index":0,"status":"failed","files":null},
					{"index":1,"status":"succeeded","files":{"audio":{"url":"` + testAudioURL + `","format":"` + tt.format + `"}}}
				]}}}`))
				require.NoError(t, err)
				assert.False(t, job.Pending())

				audioURL, contentType, err := job.Audio("")
				require.NoError(t, err)
				assert.Equal(t, testAudioURL, audioURL)
				assert.Equal(t, tt.wantContentType, contentType, "an unknown format leaves the Content-Type to the download response")
			})
		}
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

			_, _, err = job.Audio("")
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

		audioURL, contentType, err := job.Audio("")
		require.NoError(t, err)
		assert.Equal(t, testAudioURL, audioURL)
		assert.Equal(t, "audio/mpeg", contentType)
	})

	t.Run("a succeeded job without a file is an error", func(t *testing.T) {
		job, err := ParseJob([]byte(`{"id":"` + testJobID + `","status":"succeeded","result":{"output":{"variants":[]}}}`))
		require.NoError(t, err)
		_, _, err = job.Audio("")
		require.Error(t, err)
	})

	t.Run("a payload without an id is rejected", func(t *testing.T) {
		_, err := ParseJob([]byte(`{"status":"queued"}`))
		require.Error(t, err)
		_, err = ParseJob([]byte(`not json`))
		require.Error(t, err)
	})
}

const (
	testAssetID  = "ast_1"
	testVideoURL = "https://cdn.example.com/scene.mov"
)

func videoJobBody(t *testing.T, req types.CreateSFXRequest, video MediaRef) map[string]any {
	t.Helper()
	score, err := FromVideo(testModel, req)
	require.NoError(t, err)
	var body map[string]any
	require.NoError(t, json.Unmarshal(score.Body(video), &body))
	return body
}

func TestFromVideo(t *testing.T) {
	t.Run("scores the uploaded asset for the rest of the clip", func(t *testing.T) {
		body := videoJobBody(t, types.CreateSFXRequest{Prompt: "office ambience"}, AssetRef(testAssetID))

		assert.Equal(t, testModel, body["model"])
		assert.Equal(t, map[string]any{"video": map[string]any{"type": "asset", "id": testAssetID}, "prompt": "office ambience"}, body["input"])
		assert.Equal(t, map[string]any{"format": "mp3_128"}, body["output"])
		assert.NotContains(t, body, "duration_ms", "an omitted duration scores the rest of the clip")
		assert.NotContains(t, body, "start_offset_ms")
		assert.NotContains(t, body, "controls", "the mix with the provider's speech default needs no controls")
	})

	t.Run("references a URL for Mirelo to fetch", func(t *testing.T) {
		body := videoJobBody(t, types.CreateSFXRequest{}, URLRef(testVideoURL))

		assert.Equal(t, map[string]any{"video": map[string]any{"type": "url", "url": testVideoURL}}, body["input"])
	})

	t.Run("passes the window and the speech choice through", func(t *testing.T) {
		body := videoJobBody(t, types.CreateSFXRequest{
			DurationSeconds:    ptr(float32(12.5)),
			StartOffsetSeconds: ptr(float32(3)),
			PreserveSpeech:     ptr(true),
			Output:             ptr(types.CreateSFXRequestOutputSpeech),
			ResponseFormat:     ptr(types.CreateSFXRequestResponseFormatFlac),
		}, AssetRef(testAssetID))

		assert.Equal(t, float64(12500), body["duration_ms"])
		assert.Equal(t, float64(3000), body["start_offset_ms"])
		assert.Equal(t, map[string]any{"preserve_speech": true, "separate_speech": true}, body["controls"])
		assert.Equal(t, map[string]any{"format": "flac"}, body["output"])
	})

	t.Run("the sfx track turns speech preservation off unless asked", func(t *testing.T) {
		body := videoJobBody(t, types.CreateSFXRequest{Output: ptr(types.CreateSFXRequestOutputSFX)}, AssetRef(testAssetID))

		assert.Equal(t, map[string]any{"preserve_speech": false, "separate_speech": true}, body["controls"])
	})

	tests := []struct {
		name    string
		req     types.CreateSFXRequest
		wantErr string
	}{
		{"an unknown track", types.CreateSFXRequest{Output: ptr(types.CreateSFXRequestOutput("music"))}, "'output' field must be one of"},
		{"a window shorter than a second", types.CreateSFXRequest{DurationSeconds: ptr(float32(0.5))}, "'duration_seconds' field must be a number of at least 1"},
		{"a negative offset", types.CreateSFXRequest{StartOffsetSeconds: ptr(float32(-1))}, "'start_offset_seconds'"},
		{"loop, a text prompt field", types.CreateSFXRequest{Loop: ptr(true)}, "'loop'"},
		{"prompt_influence, a text prompt field", types.CreateSFXRequest{PromptInfluence: ptr(float32(0.5))}, "'prompt_influence'"},
		{"an unsupported format", types.CreateSFXRequest{ResponseFormat: ptr(types.CreateSFXRequestResponseFormatPcm)}, supportedFormats},
	}
	for _, tt := range tests {
		t.Run("rejects "+tt.name, func(t *testing.T) {
			_, err := FromVideo(testModel, tt.req)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func TestAudioPicksTheWantedTrack(t *testing.T) {
	job, err := ParseJob([]byte(`{"id":"` + testJobID + `","status":"succeeded","errors":[],"result":{"outputs":[
		{"index":0,"type":"mix","variants":[{"index":0,"status":"succeeded","files":{"audio":{"url":"https://cdn/mix.mp3","format":"mp3_128"}}}]},
		{"index":1,"type":"speech","variants":[{"index":0,"status":"succeeded","files":{"audio":{"url":"https://cdn/speech.wav","format":"wav"}}}]}
	]}}`))
	require.NoError(t, err)

	mixURL, mixType, err := job.Audio(OutputMix)
	require.NoError(t, err)
	assert.Equal(t, "https://cdn/mix.mp3", mixURL)
	assert.Equal(t, "audio/mpeg", mixType)

	speechURL, speechType, err := job.Audio(OutputSpeech)
	require.NoError(t, err)
	assert.Equal(t, "https://cdn/speech.wav", speechURL)
	assert.Equal(t, "audio/wav", speechType)

	_, _, err = job.Audio(OutputSFX)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "without a sfx track")
}
