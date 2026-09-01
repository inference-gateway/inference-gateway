// Package tts synthesizes speech locally by shelling out to llama.cpp's
// llama-tts binary as a one-shot process per request. It is an explicit
// stopgap: the moment llama.cpp ships a server-side speech endpoint
// (ggml-org/llama.cpp#21956, mtmd_gen work) this package can be removed in
// favor of proxying to llama-server.
//
// The asset cache is shared with the Inference Gateway CLI in fixed
// well-known locations (never configurable): binaries in ~/.infer/bin and
// GGUF weights in ~/.infer/models/tts. Downloads write to a temp file and
// rename atomically, so a concurrently running CLI and gateway never corrupt
// the cache; an already-present file is treated as done and never
// re-downloaded.
package tts

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	l "github.com/inference-gateway/inference-gateway/logger"
)

const (
	// ReservedModelID is the gateway model id routed to the built-in local
	// engine instead of a provider.
	ReservedModelID = "local/qwen3-tts"

	// ModelPrefix reserves the local/ namespace for built-in engines.
	ModelPrefix = "local/"

	// BinaryName is llama.cpp's one-shot text-to-speech tool.
	BinaryName = "llama-tts"

	// Shared cache layout, adopted from the CLI (fixed, not configurable).
	// Exported because the 503 guidance and manual pre-download docs name them.
	CacheBinDir    = ".infer/bin"
	CacheModelsDir = ".infer/models/tts"
	checksumsName  = "checksums.txt"
	referenceName  = "reference.wav"
	outputName     = "speech.wav"

	// ttsLanguage is the utterance language hint passed to llama-tts.
	ttsLanguage = "en"

	// RetryAfterSeconds is advised on the 503 returned while speech assets
	// are not ready yet.
	RetryAfterSeconds = 10

	// defaultTimeout is the belt-and-braces fallback for Timeout.
	defaultTimeout = 300 * time.Second

	downloadBufSize = 256 << 10
	stderrTailBytes = 500
)

var (
	// Package vars (not consts) so tests can aim them at an httptest server
	// instead of the real release / model hosts.
	binaryRepoBase = "https://github.com/inference-gateway/binaries/releases/latest/download"
	modelRepoBase  = "https://huggingface.co/ggml-org/Qwen3-TTS-12Hz-1.7B-Base-GGUF/resolve/main"
)

// BackboneGGUF / MmprojGGUF are the Qwen3-TTS weights (backbone + vision
// Tower) that llama-tts runs via -m / -mm.
//
// ponytail: exact filenames follow llama.cpp repo convention; the CLI's
// internal/audio/ttsmodel.go selects the same files so the shared cache
// lines up - keep the two in lockstep.
const (
	BackboneGGUF = "Qwen3-TTS-12Hz-1.7B-Base-Q4_K_M.gguf"
	MmprojGGUF   = "mmproj-Qwen3-TTS-12Hz-1.7B-Base-Q8_0.gguf"
)

// modelSHA256 pins each GGUF's sha256 (the HF LFS oid), so model downloads
// are verified and tamper-evident without a runtime HF API call. A package
// var (not const) only so tests can aim it at dummy payloads.
var modelSHA256 = map[string]string{
	BackboneGGUF: "8d18c94acb2addd042f97da63c98be144eafa76d0d9495177eab65130cf85129",
	MmprojGGUF:   "6fd65188839bcd6ecc91b277ad471e22a0edfada4699a0fe82f1165c18cfcce2",
}

// NotReadyError means the local speech assets are not usable yet (still
// downloading, the last download failed, or auto-download is disabled and
// something is missing). The speech endpoint maps it to 503 + Retry-After.
type NotReadyError struct{ Detail string }

func (e *NotReadyError) Error() string { return e.Detail }

// Request is one local speech synthesis job.
type Request struct {
	// Input is the text to speak.
	Input string
	// ReferenceAudio is the base64-decoded wav/mp3 voice sample for
	// zero-shot cloning; empty uses the model's stock voice.
	ReferenceAudio []byte
}

// Config wires the AUDIO_LOCAL_* settings plus the cache root.
type Config struct {
	// AutoDownload allows fetching the llama-tts binary and GGUF weights.
	// Anything already present is never re-downloaded. When false the engine
	// is purely a consumer of the existing cache or PATH.
	AutoDownload bool
	// MaxConcurrency bounds concurrent syntheses; requests beyond it queue.
	MaxConcurrency int
	// Timeout bounds a single synthesis (queue wait and exec).
	Timeout time.Duration
	// Home is the root of the shared ~/.infer cache.
	Home string
}

// Engine runs one-shot llama-tts processes. The zero value is not usable;
// construct with NewEngine.
type Engine struct {
	cfg    Config
	logger l.Logger
	http   *http.Client
	sem    chan struct{}

	mu          sync.Mutex
	downloading bool
	progress    string // human-readable status of the running download
	lastFailure string // why the last ensure failed, "" if none
}

// NewEngine builds the local speech engine.
func NewEngine(logger l.Logger, cfg Config) *Engine {
	cfg.MaxConcurrency = max(cfg.MaxConcurrency, 1)
	if cfg.Timeout <= 0 {
		cfg.Timeout = defaultTimeout
	}
	return &Engine{
		cfg:    cfg,
		logger: logger,
		http:   &http.Client{},
		sem:    make(chan struct{}, cfg.MaxConcurrency),
	}
}

// Warmup ensures the binary and GGUF weights exist, downloading the missing
// ones when AutoDownload allows it. The gateway invokes it in a background
// goroutine at startup (never blocking boot or chat routes) and requests may
// re-trigger it to self-heal a failed attempt; concurrent calls are
// deduplicated.
func (e *Engine) Warmup(ctx context.Context) {
	if !e.cfg.AutoDownload {
		e.logger.Debug("audio: local speech auto-download disabled; serving only from the cache or PATH")
		return
	}

	e.mu.Lock()
	if e.downloading {
		e.mu.Unlock()
		return // an attempt is already running (startup or a request retry)
	}
	e.downloading = true
	e.progress = "checking speech assets"
	e.mu.Unlock()

	err := e.ensure(ctx)

	e.mu.Lock()
	e.downloading = false
	if err != nil {
		e.lastFailure = err.Error()
	} else {
		e.lastFailure = ""
		e.progress = ""
	}
	e.mu.Unlock()

	if err != nil {
		e.logger.Error("audio: speech assets are not ready; /v1/audio/speech will return 503 with details", err)
		return
	}
	e.logger.Info("audio: local speech engine ready",
		"binary", filepath.Base(e.binPath()),
		"backbone", BackboneGGUF,
		"mmproj", MmprojGGUF)
}

// Synthesize runs a one-shot llama-tts process and returns the WAV bytes.
// It returns *NotReadyError while assets are not usable, never blocks on a
// download, and honors the configured Timeout for queue wait and exec.
func (e *Engine) Synthesize(ctx context.Context, req Request) ([]byte, error) {
	if ok, detail := e.readiness(); !ok {
		if e.cfg.AutoDownload {
			// Warmup may have failed or not run; retry in the background
			// without ever holding this request open for a download.
			go e.Warmup(context.WithoutCancel(ctx))
		}
		return nil, &NotReadyError{Detail: detail}
	}

	ctx, cancel := context.WithTimeout(ctx, e.cfg.Timeout)
	defer cancel()

	select { // bounded concurrency: queue beyond the limit, never fail
	case e.sem <- struct{}{}:
		defer func() { <-e.sem }()
	case <-ctx.Done():
		return nil, fmt.Errorf("timed out waiting for a free synthesis slot: %w", ctx.Err())
	}

	bin, ok := e.resolveBinary()
	if !ok {
		return nil, &NotReadyError{Detail: BinaryName + " binary is unavailable"}
	}

	dir, err := os.MkdirTemp("", "inference-gateway-tts-")
	if err != nil {
		return nil, fmt.Errorf("creating synthesis temp dir: %w", err)
	}
	defer os.RemoveAll(dir)

	out := filepath.Join(dir, outputName)
	args := []string{
		"-m", e.backbonePath(),
		"-mm", e.mmprojPath(),
		"-p", req.Input,
		"--tts-lang", ttsLanguage,
	}
	if len(req.ReferenceAudio) > 0 {
		ref := filepath.Join(dir, referenceName)
		if err := os.WriteFile(ref, req.ReferenceAudio, 0o600); err != nil {
			return nil, fmt.Errorf("storing reference audio: %w", err)
		}
		args = append(args, "--tts-speaker-file", ref)
	}
	args = append(args, "--output", out)

	output, err := exec.CommandContext(ctx, bin, args...).CombinedOutput()
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil { // a ctx kill surfaces as ExitError, not ctx.Err()
			return nil, fmt.Errorf("llama-tts timed out: %w", ctxErr)
		}
		return nil, fmt.Errorf("llama-tts failed: %w: %s", err, stderrTail(output))
	}
	wav, err := os.ReadFile(out)
	if err != nil {
		return nil, fmt.Errorf("llama-tts produced no wav: %w", err)
	}
	return wav, nil
}

// readiness reports whether every asset is usable; when not, detail carries
// download progress or an actionable message for the 503 body.
func (e *Engine) readiness() (ok bool, detail string) {
	var missing []string
	if _, ok := e.resolveBinary(); !ok {
		missing = append(missing, BinaryName+" binary")
	}
	for _, path := range []string{e.backbonePath(), e.mmprojPath()} {
		if _, err := os.Stat(path); err != nil {
			missing = append(missing, filepath.Base(path))
		}
	}
	if len(missing) == 0 {
		return true, ""
	}

	hint := fmt.Sprintf("; pre-download them into %s and %s, or set AUDIO_LOCAL_AUTO_DOWNLOAD=true",
		filepath.Join(e.cfg.Home, CacheBinDir), filepath.Join(e.cfg.Home, CacheModelsDir))

	e.mu.Lock()
	defer e.mu.Unlock()
	switch {
	case e.downloading:
		return false, fmt.Sprintf("speech assets are still downloading (%s); retry shortly", e.progress)
	case e.lastFailure != "":
		return false, fmt.Sprintf("speech assets unavailable: the last download failed (%s); calling again retriggers it in the background%s",
			e.lastFailure, hint)
	default:
		return false, "speech assets are not ready: " + strings.Join(missing, ", ") + "." + hint
	}
}

// ensure verifies every asset and downloads the ones that are missing.
// Callers must ensure only one runs at a time (Warmup dedups).
func (e *Engine) ensure(ctx context.Context) error {
	if _, ok := e.resolveBinary(); !ok {
		if err := e.downloadBinary(ctx); err != nil {
			return fmt.Errorf("obtaining llama-tts binary: %w", err)
		}
	}
	for _, gguf := range []string{BackboneGGUF, MmprojGGUF} {
		dest := filepath.Join(e.modelDir(), gguf)
		if _, err := os.Stat(dest); err == nil {
			continue // an existing file is treated as done, whoever wrote it (CLI or gateway)
		}
		if err := e.download(ctx, modelRepoBase+"/"+gguf, dest, modelSHA256[gguf], 0o644); err != nil {
			return fmt.Errorf("downloading %s: %w", gguf, err)
		}
	}
	return nil
}

// downloadBinary fetches the static binary from the binaries repo, sha256-verified
// against the release's checksums.txt.
func (e *Engine) downloadBinary(ctx context.Context) error {
	asset := binaryAsset()
	sums, err := e.fetchChecksums(ctx, strings.TrimSuffix(binaryRepoBase, "/")+"/"+checksumsName)
	if err != nil {
		return fmt.Errorf("fetching %s: %w", checksumsName, err)
	}
	want, ok := sums[asset]
	if !ok {
		return fmt.Errorf("%s has no entry for %s", checksumsName, asset)
	}
	return e.download(ctx, strings.TrimSuffix(binaryRepoBase, "/")+"/"+asset, e.binPath(), want, 0o755)
}

// download fetches rawURL into dest atomically (temp file in the same
// directory, then rename), verifying the sha256 when wantSum is non-empty.
func (e *Engine) download(ctx context.Context, rawURL, dest, wantSHA string, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	name := filepath.Base(dest)
	e.setStatus("starting download of " + name)
	tmp, err := os.CreateTemp(filepath.Dir(dest), "."+name+".part")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name()) // no-op once renamed into place

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return err
	}
	resp, err := e.http.Do(httpReq)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%s responded %s", rawURL, resp.Status)
	}

	hash := sha256.New()
	var got int64
	buf := make([]byte, downloadBufSize)
	for {
		n, rerr := resp.Body.Read(buf)
		if n > 0 {
			if _, werr := tmp.Write(buf[:n]); werr != nil {
				return fmt.Errorf("writing %s: %w", tmp.Name(), werr)
			}
			_, _ = hash.Write(buf[:n])
			got += int64(n)
			e.setStatus(downloadProgress(name, got, resp.ContentLength))
		}
		if rerr != nil {
			if rerr == io.EOF {
				break
			}
			return rerr
		}
	}
	if err := tmp.Close(); err != nil {
		return err
	}

	if wantSHA != "" {
		if gotSum := hex.EncodeToString(hash.Sum(nil)); gotSum != wantSHA {
			return fmt.Errorf("sha256 mismatch for %s: want %s got %s", name, wantSHA, gotSum)
		}
	}
	if err := os.Chmod(tmp.Name(), mode); err != nil {
		return err
	}
	if err := os.Rename(tmp.Name(), dest); err != nil {
		return fmt.Errorf("moving %s into place: %w", name, err)
	}
	e.logger.Info("audio: downloaded speech asset", "file", dest, "bytes", got)
	return nil
}

// fetchChecksums parses a checksums.txt ("sha256  filename" per line).
func (e *Engine) fetchChecksums(ctx context.Context, url string) (map[string]string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := e.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s responded %s", url, resp.Status)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	sums := make(map[string]string)
	for line := range strings.SplitSeq(string(body), "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 2 {
			sums[fields[1]] = strings.ToLower(fields[0])
		}
	}
	return sums, nil
}

// resolveBinary returns the llama-tts binary to exec: PATH wins, then the
// shared cache.
func (e *Engine) resolveBinary() (string, bool) {
	if p, err := exec.LookPath(BinaryName); err == nil {
		return p, true
	}
	p := e.binPath()
	if info, err := os.Stat(p); err == nil && !info.IsDir() {
		return p, true
	}
	return "", false
}

func (e *Engine) binPath() string {
	return filepath.Join(e.cfg.Home, CacheBinDir, binaryFileName())
}

func (e *Engine) modelDir() string {
	return filepath.Join(e.cfg.Home, CacheModelsDir)
}

func (e *Engine) backbonePath() string {
	return filepath.Join(e.cfg.Home, CacheModelsDir, BackboneGGUF)
}

func (e *Engine) mmprojPath() string {
	return filepath.Join(e.cfg.Home, CacheModelsDir, MmprojGGUF)
}

// binaryAsset is the per-platform asset name on binaries releases,
// following the whisper-cli and ffmpeg naming.
func binaryAsset() string {
	name := fmt.Sprintf("%s-%s-%s", BinaryName, runtime.GOOS, runtime.GOARCH)
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	return name
}

// binaryFileName is the cached binary's file name.
func binaryFileName() string {
	if runtime.GOOS == "windows" {
		return BinaryName + ".exe"
	}
	return BinaryName
}

func (e *Engine) setStatus(s string) {
	e.mu.Lock()
	e.progress = s
	e.mu.Unlock()
}

func downloadProgress(name string, got, total int64) string {
	if total <= 0 {
		return fmt.Sprintf("downloading %s (%s done)", name, humanBytes(got))
	}
	return fmt.Sprintf("downloading %s (%d%%)", name, got*100/total)
}

func humanBytes(n int64) string {
	return strconv.FormatFloat(float64(n)/(1<<20), 'f', 1, 64) + " MB"
}

// stderrTail returns the last stderrTailBytes of process output.
func stderrTail(b []byte) string {
	if len(b) > stderrTailBytes {
		b = b[len(b)-stderrTailBytes:]
	}
	return strings.TrimSpace(string(b))
}
