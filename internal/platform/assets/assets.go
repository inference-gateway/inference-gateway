// Package assets downloads and caches llama.cpp assets - release binaries and
// GGUF weights - in the fixed ~/.infer cache shared with the Inference Gateway
// CLI (never configurable): binaries in ~/.infer/bin/tools (falling back to
// the legacy ~/.infer/bin) and weights under ~/.infer/models. Downloads write
// to a temp file next to the destination and rename atomically, so a
// concurrently running CLI and gateway never corrupt the cache. A cached
// binary is replaced when its sha256 differs from the latest release's
// checksums.txt, mirroring the binaries repo's install.sh.
package assets

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

	logger "github.com/inference-gateway/inference-gateway/internal/platform/logger"
)

const (
	// ToolsDir is the CLI's tools install dir, where binary downloads land.
	ToolsDir = ".infer/bin/tools"
	// LegacyBinDir is the flat pre-tools cache location, still honored for reads.
	LegacyBinDir = ".infer/bin"
	// ModelsDir is the root of the shared GGUF cache.
	ModelsDir = ".infer/models"
	// ChecksumsName is the "sha256  filename" manifest shipped with every release.
	ChecksumsName = "checksums.txt"

	downloadBufSize   = 256 << 10
	checksumsMaxBytes = 1 << 20
	windowsOS         = "windows"
	exeSuffix         = ".exe"
)

// ReleaseBase is where the llama.cpp binaries are published. A var (not a
// const) so tests can aim it at an httptest server.
var ReleaseBase = "https://github.com/inference-gateway/binaries/releases/latest/download"

// Downloader fetches assets over HTTP. The zero value works without a logger
// or status sink.
type Downloader struct {
	HTTP   *http.Client
	Logger logger.Logger
	// Header is sent with every request, for example a Hugging Face bearer token.
	Header http.Header
	// Status receives human-readable progress while a download runs.
	Status func(string)
}

// Download fetches rawURL into dest atomically (temp file in the same
// directory, then rename), verifying the sha256 when wantSHA is non-empty.
func (d *Downloader) Download(ctx context.Context, rawURL, dest, wantSHA string, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	name := filepath.Base(dest)
	d.status("starting download of " + name)
	tmp, err := os.CreateTemp(filepath.Dir(dest), "."+name+".part")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name()) // no-op once renamed into place

	resp, err := d.Get(ctx, rawURL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

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
			d.status(Progress(name, got, resp.ContentLength))
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
	d.log().Info("downloaded asset", "file", dest, "bytes", got)
	return nil
}

// FetchChecksums parses a checksums.txt ("sha256  filename" per line).
func (d *Downloader) FetchChecksums(ctx context.Context, url string) (map[string]string, error) {
	resp, err := d.Get(ctx, url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, checksumsMaxBytes))
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

// EnsureBinary makes the named llama.cpp tool runnable: one on PATH is left
// alone, a cached one is replaced when its sha256 differs from the latest
// release, a missing one is downloaded. An unreachable checksums.txt keeps
// whatever is already cached.
func (d *Downloader) EnsureBinary(ctx context.Context, home, name string) error {
	if p, err := exec.LookPath(name); err == nil {
		d.log().Debug("using binary from PATH; upgrades are the operator's", "binary", p)
		return nil
	}

	dest, cached := CachedBinary(home, name)
	asset := BinaryAsset(name)
	base := strings.TrimSuffix(ReleaseBase, "/")
	sums, err := d.FetchChecksums(ctx, base+"/"+ChecksumsName)
	if err != nil {
		if cached {
			d.log().Warn("cannot check binary for a newer release; keeping the cached one",
				"binary", dest, "error", err.Error())
			return nil
		}
		return fmt.Errorf("fetching %s: %w", ChecksumsName, err)
	}
	want, ok := sums[asset]
	if !ok {
		return fmt.Errorf("%s has no entry for %s", ChecksumsName, asset)
	}

	if cached {
		got, err := FileSHA256(dest)
		if err == nil && got == want {
			return nil
		}
		d.log().Info("cached binary is stale; downloading the latest release", "binary", dest)
	}
	return d.Download(ctx, base+"/"+asset, dest, want, 0o755)
}

// Get issues a GET with the configured headers plus extra ones and fails on
// any non-200. The caller closes the body.
func (d *Downloader) Get(ctx context.Context, rawURL string, extra ...http.Header) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	d.Apply(req.Header, extra...)
	resp, err := d.Client().Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("%s responded %s", rawURL, resp.Status)
	}
	return resp, nil
}

// Apply copies the configured headers plus extra ones onto h.
func (d *Downloader) Apply(h http.Header, extra ...http.Header) {
	for _, src := range append([]http.Header{d.Header}, extra...) {
		for k, v := range src {
			h[k] = v
		}
	}
}

// Client is the HTTP client requests go through, http.DefaultClient when unset.
func (d *Downloader) Client() *http.Client {
	if d.HTTP != nil {
		return d.HTTP
	}
	return http.DefaultClient
}

func (d *Downloader) log() logger.Logger {
	if d.Logger != nil {
		return d.Logger
	}
	return logger.NewNoopLogger()
}

func (d *Downloader) status(s string) {
	if d.Status != nil {
		d.Status(s)
	}
}

// ResolveBinary returns the binary to exec: PATH wins, then the shared cache.
func ResolveBinary(home, name string) (string, bool) {
	if p, err := exec.LookPath(name); err == nil {
		return p, true
	}
	if p, exists := CachedBinary(home, name); exists {
		return p, true
	}
	return "", false
}

// CachedBinary returns the cached binary and whether it exists; when it does
// not, the path is where a download lands.
func CachedBinary(home, name string) (path string, exists bool) {
	for _, p := range []string{BinPath(home, name), LegacyBinPath(home, name)} {
		if info, err := os.Stat(p); err == nil && !info.IsDir() {
			return p, true
		}
	}
	return BinPath(home, name), false
}

// BinPath is where a downloaded binary lands.
func BinPath(home, name string) string {
	return filepath.Join(home, ToolsDir, BinaryFileName(name))
}

// LegacyBinPath is the flat pre-tools cache location.
func LegacyBinPath(home, name string) string {
	return filepath.Join(home, LegacyBinDir, BinaryFileName(name))
}

// FileSHA256 returns the hex sha256 of the file at path.
func FileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

// BinaryAsset is the per-platform asset name on binaries releases, following
// the whisper-cli and ffmpeg naming.
func BinaryAsset(name string) string {
	asset := fmt.Sprintf("%s-%s-%s", name, runtime.GOOS, runtime.GOARCH)
	if runtime.GOOS == windowsOS {
		asset += exeSuffix
	}
	return asset
}

// BinaryFileName is the cached binary's file name.
func BinaryFileName(name string) string {
	if runtime.GOOS == windowsOS {
		return name + exeSuffix
	}
	return name
}

// Progress renders a human-readable download status line.
func Progress(name string, got, total int64) string {
	if total <= 0 {
		return fmt.Sprintf("downloading %s (%s done)", name, humanBytes(got))
	}
	return fmt.Sprintf("downloading %s (%d%%)", name, got*100/total)
}

func humanBytes(n int64) string {
	return strconv.FormatFloat(float64(n)/(1<<20), 'f', 1, 64) + " MB"
}
