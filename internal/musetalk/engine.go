// Package musetalk lip-syncs a portrait to an audio clip on this machine by
// running a MuseTalk engine as a one-shot process per render. The engine is an
// executable with a fixed command line, so the runtime behind it can change
// without the gateway noticing.
package musetalk

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	logger "github.com/inference-gateway/inference-gateway/internal/platform/logger"
	types "github.com/inference-gateway/inference-gateway/providers/types"
)

const (
	// ReservedModelID is the gateway model id routed to the built-in local
	// engine instead of a provider.
	ReservedModelID = "local/musetalk"

	// ModelPrefix reserves the local/ namespace for built-in engines.
	ModelPrefix = "local/"

	// BinaryName is the engine executable. It is run as
	// `musetalk --model-dir <dir> --image <portrait> --audio <clip> --out <mp4>`.
	BinaryName = "musetalk"

	// ModelRepo is the Hugging Face repository holding the weights.
	ModelRepo = "mlx-community/MuseTalk-1.5-fp16"

	// Shared cache layout, fixed like the speech engine's. The weights are
	// where `infer models download` puts a Hugging Face repository.
	CacheToolsDir = ".infer/bin/tools"
	CacheModelDir = ".infer/models/huggingface/" + ModelRepo

	// RetryAfterSeconds is advised on the 503 returned while the engine or
	// its weights are missing.
	RetryAfterSeconds = 30

	defaultTimeout  = 15 * time.Minute
	maxKeptRenders  = 16
	jobIDBytes      = 12
	stderrTailBytes = 500
	workDirPattern  = "musetalk-*"
	clipName        = "clip.mp4"
	defaultImageExt = ".png"
	defaultAudioExt = ".wav"
	weightsFile     = "unet.safetensors"
	renderFailed    = "the local render failed"
)

// NotReadyError means the engine or its weights are not installed, with what
// to do about it. The videos endpoint maps it to 503 + Retry-After.
type NotReadyError struct{ Detail string }

func (e *NotReadyError) Error() string { return e.Detail }

// Upload is a file a render starts from: its bytes and the name the client
// gave it, which decides the extension the engine reads it under.
type Upload struct {
	Name string
	Data io.Reader
}

// Engine renders talking clips one at a time and remembers the jobs it was
// given, since nothing else holds a local job between a client's polls. The
// zero value is not usable, construct with NewEngine.
type Engine struct {
	logger  logger.Logger
	timeout time.Duration
	slot    chan struct{}

	mu      sync.Mutex
	renders map[string]*render
	order   []string
}

type render struct {
	job  types.VideoJob
	dir  string
	clip string
}

// NewEngine builds the local lip-sync engine.
func NewEngine(logger logger.Logger) *Engine {
	return &Engine{
		logger:  logger,
		timeout: defaultTimeout,
		slot:    make(chan struct{}, 1),
		renders: map[string]*render{},
	}
}

// Submit keeps the portrait and the clip, queues the render and returns its
// job. The render carries on after the request that started it is answered.
func (e *Engine) Submit(portrait, audio Upload) (types.VideoJob, error) {
	binary, modelDir, err := installed()
	if err != nil {
		return types.VideoJob{}, err
	}

	dir, err := os.MkdirTemp("", workDirPattern)
	if err != nil {
		return types.VideoJob{}, fmt.Errorf("creating the render folder: %w", err)
	}
	job, err := e.start(dir, binary, modelDir, portrait, audio)
	if err != nil {
		_ = os.RemoveAll(dir)
	}
	return job, err
}

// start keeps the uploads in dir, names the job and sets the render going.
func (e *Engine) start(dir, binary, modelDir string, portrait, audio Upload) (types.VideoJob, error) {
	image, err := keep(dir, "portrait", defaultImageExt, portrait)
	if err != nil {
		return types.VideoJob{}, err
	}
	clip, err := keep(dir, "audio", defaultAudioExt, audio)
	if err != nil {
		return types.VideoJob{}, err
	}
	id, err := newJobID()
	if err != nil {
		return types.VideoJob{}, err
	}
	job := e.remember(id, dir)
	go e.run(id, binary, modelDir, image, clip)
	return job, nil
}

// Job returns the job with id and, once it completed, the path of its video.
func (e *Engine) Job(id string) (types.VideoJob, string, bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	r, known := e.renders[id]
	if !known {
		return types.VideoJob{}, "", false
	}
	return r.job, r.clip, true
}

// installed returns the engine executable and its weights folder, or what is
// missing and how to get it.
func installed() (binary, modelDir string, err error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", "", fmt.Errorf("resolving the home directory: %w", err)
	}
	binary = filepath.Join(home, CacheToolsDir, BinaryName)
	if info, statErr := os.Stat(binary); statErr != nil || info.IsDir() {
		return "", "", &NotReadyError{Detail: fmt.Sprintf("The local lip-sync engine is not installed: %s is missing.", binary)}
	}
	modelDir = filepath.Join(home, filepath.FromSlash(CacheModelDir))
	if _, statErr := os.Stat(filepath.Join(modelDir, weightsFile)); statErr != nil {
		return "", "", &NotReadyError{Detail: fmt.Sprintf("The local lip-sync weights are missing: download %s to %s (infer models download %s).", ModelRepo, modelDir, ModelRepo)}
	}
	return binary, modelDir, nil
}

// keep writes an upload into dir under base, with the extension of the name
// the client gave it.
func keep(dir, base, defaultExt string, upload Upload) (string, error) {
	ext := strings.ToLower(filepath.Ext(upload.Name))
	if ext == "" {
		ext = defaultExt
	}
	path := filepath.Join(dir, base+ext)
	file, err := os.Create(path)
	if err != nil {
		return "", fmt.Errorf("keeping the %s: %w", base, err)
	}
	defer file.Close()
	if _, err := io.Copy(file, upload.Data); err != nil {
		return "", fmt.Errorf("keeping the %s: %w", base, err)
	}
	return path, nil
}

func newJobID() (string, error) {
	raw := make([]byte, jobIDBytes)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("naming the render: %w", err)
	}
	return hex.EncodeToString(raw), nil
}

// remember stores a queued job and forgets the oldest finished ones beyond
// what the engine keeps, removing their files.
func (e *Engine) remember(id, dir string) types.VideoJob {
	job := types.VideoJob{
		ID:        id,
		Object:    types.VideoJobObjectVideo,
		Model:     ReservedModelID,
		Status:    types.VideoJobStatusQueued,
		CreatedAt: int(time.Now().Unix()),
	}

	e.mu.Lock()
	defer e.mu.Unlock()
	e.renders[id] = &render{job: job, dir: dir}
	e.order = append(e.order, id)
	for len(e.order) > maxKeptRenders {
		oldest := e.renders[e.order[0]]
		if !finished(oldest.job.Status) {
			break
		}
		_ = os.RemoveAll(oldest.dir)
		delete(e.renders, e.order[0])
		e.order = e.order[1:]
	}
	return job
}

func finished(status types.VideoJobStatus) bool {
	return status == types.VideoJobStatusCompleted || status == types.VideoJobStatusFailed
}

// run waits for the engine's turn, renders the clip and records how it ended.
func (e *Engine) run(id, binary, modelDir, image, audio string) {
	e.slot <- struct{}{}
	defer func() { <-e.slot }()
	e.update(id, func(r *render) { r.job.Status = types.VideoJobStatusInProgress })

	ctx, cancel := context.WithTimeout(context.Background(), e.timeout)
	defer cancel()

	out := filepath.Join(filepath.Dir(image), clipName)
	cmd := exec.CommandContext(ctx, binary, "--model-dir", modelDir, "--image", image, "--audio", audio, "--out", out)
	output, err := cmd.CombinedOutput()
	if err == nil {
		if _, statErr := os.Stat(out); statErr != nil {
			err = fmt.Errorf("the engine wrote no video")
		}
	}

	now := int(time.Now().Unix())
	if err != nil {
		e.logger.Error("local lip-sync render failed", err, "id", id, "output", stderrTail(output))
		message := failure(ctx, output)
		e.update(id, func(r *render) {
			r.job.Status, r.job.CompletedAt = types.VideoJobStatusFailed, &now
			r.job.Error = &struct {
				Code    *string `json:"code,omitempty"`
				Message *string `json:"message,omitempty"`
			}{Message: &message}
		})
		return
	}
	e.update(id, func(r *render) {
		r.job.Status, r.job.CompletedAt, r.clip = types.VideoJobStatusCompleted, &now, out
	})
}

// failure words why a render ended without a video: it ran out of time, or
// whatever the engine last said.
func failure(ctx context.Context, output []byte) string {
	if ctx.Err() == context.DeadlineExceeded {
		return "the local render timed out"
	}
	if tail := stderrTail(output); tail != "" {
		return tail
	}
	return renderFailed
}

func (e *Engine) update(id string, change func(*render)) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if r, known := e.renders[id]; known {
		change(r)
	}
}

// stderrTail returns the last stderrTailBytes of process output.
func stderrTail(b []byte) string {
	if len(b) > stderrTailBytes {
		b = b[len(b)-stderrTailBytes:]
	}
	return strings.TrimSpace(string(b))
}
