package musetalk

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"

	logger "github.com/inference-gateway/inference-gateway/internal/platform/logger"
	types "github.com/inference-gateway/inference-gateway/providers/types"
)

const (
	testPortrait = "PNG-PORTRAIT"
	testClip     = "RIFF-CLIP"
	testWait     = 10 * time.Second
	testTick     = 20 * time.Millisecond

	// A stand-in engine that writes the portrait's extension, the clip and
	// the weights folder it was handed into the video it was asked for.
	workingEngine = `#!/bin/sh
while [ $# -gt 0 ]; do
  case "$1" in
    --model-dir) model="$2"; shift ;;
    --image) image="$2"; shift ;;
    --audio) audio="$2"; shift ;;
    --out) out="$2"; shift ;;
  esac
  shift
done
printf 'MP4 %s %s %s' "${image##*.}" "$(cat "$audio")" "$(basename "$model")" > "$out"
`
	brokenEngine = `#!/bin/sh
echo "no face found in the portrait" >&2
exit 3
`
	silentEngine = `#!/bin/sh
exit 0
`
)

// home points the engine at a fresh home folder, with the engine executable
// and the weights installed as asked.
func home(t *testing.T, engine string, weights bool) {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	if engine != "" {
		tools := filepath.Join(dir, CacheToolsDir)
		require.NoError(t, os.MkdirAll(tools, 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(tools, BinaryName), []byte(engine), 0o755))
	}
	if weights {
		model := filepath.Join(dir, filepath.FromSlash(CacheModelDir))
		require.NoError(t, os.MkdirAll(model, 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(model, weightsFile), []byte("weights"), 0o644))
	}
}

func submit(t *testing.T, e *Engine) types.VideoJob {
	t.Helper()
	job, err := e.Submit(
		Upload{Name: "front.JPEG", Data: strings.NewReader(testPortrait)},
		Upload{Name: "line.wav", Data: strings.NewReader(testClip)},
	)
	require.NoError(t, err)
	return job
}

func settled(t *testing.T, e *Engine, id string) (types.VideoJob, string) {
	t.Helper()
	var job types.VideoJob
	var clip string
	require.Eventually(t, func() bool {
		var known bool
		job, clip, known = e.Job(id)
		return known && finished(job.Status)
	}, testWait, testTick)
	return job, clip
}

func TestSubmitRendersTheClipWithTheInstalledEngine(t *testing.T) {
	home(t, workingEngine, true)
	e := NewEngine(logger.NewNoopLogger())

	queued := submit(t, e)
	assert.Equal(t, types.VideoJobStatusQueued, queued.Status)
	assert.Equal(t, ReservedModelID, queued.Model)
	assert.Equal(t, types.VideoJobObjectVideo, queued.Object)
	assert.NotZero(t, queued.CreatedAt)

	job, clip := settled(t, e, queued.ID)
	require.Equal(t, types.VideoJobStatusCompleted, job.Status)
	assert.NotNil(t, job.CompletedAt)
	rendered, err := os.ReadFile(clip)
	require.NoError(t, err)
	assert.Equal(t, "MP4 jpeg "+testClip+" MuseTalk-1.5-fp16", string(rendered),
		"the engine gets the portrait under its own extension, the clip and the weights folder")
}

func TestAFailedRenderCarriesWhatTheEngineSaid(t *testing.T) {
	tests := []struct {
		name    string
		engine  string
		wantMsg string
	}{
		{"an engine that exits with an error", brokenEngine, "no face found in the portrait"},
		{"an engine that writes no video", silentEngine, renderFailed},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			home(t, tt.engine, true)
			e := NewEngine(logger.NewNoopLogger())

			job, clip := settled(t, e, submit(t, e).ID)

			require.Equal(t, types.VideoJobStatusFailed, job.Status)
			require.NotNil(t, job.Error)
			assert.Equal(t, tt.wantMsg, *job.Error.Message)
			assert.Empty(t, clip)
		})
	}
}

func TestSubmitSaysWhatIsMissing(t *testing.T) {
	tests := []struct {
		name    string
		engine  string
		weights bool
		want    string
	}{
		{"no engine", "", true, "engine is not installed"},
		{"no weights", workingEngine, false, "infer models download " + ModelRepo},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			home(t, tt.engine, tt.weights)
			e := NewEngine(logger.NewNoopLogger())

			_, err := e.Submit(Upload{Name: "a.png", Data: strings.NewReader("p")}, Upload{Name: "a.wav", Data: strings.NewReader("a")})

			var notReady *NotReadyError
			require.ErrorAs(t, err, &notReady)
			assert.Contains(t, notReady.Detail, tt.want)
		})
	}
}

func TestJobKnowsOnlyWhatWasSubmitted(t *testing.T) {
	home(t, workingEngine, true)
	e := NewEngine(logger.NewNoopLogger())

	_, _, known := e.Job("never-submitted")

	assert.False(t, known)
}

func TestTheOldestFinishedRendersAreForgotten(t *testing.T) {
	home(t, workingEngine, true)
	e := NewEngine(logger.NewNoopLogger())

	first := submit(t, e)
	_, firstClip := settled(t, e, first.ID)
	var last types.VideoJob
	for range maxKeptRenders {
		last = submit(t, e)
		settled(t, e, last.ID)
	}

	_, _, known := e.Job(first.ID)
	assert.False(t, known, "the oldest render is dropped once more are kept than the engine holds")
	assert.NoFileExists(t, firstClip)
	_, _, known = e.Job(last.ID)
	assert.True(t, known)
}
