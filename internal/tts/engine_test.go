package tts

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	require "github.com/stretchr/testify/require"

	logger "github.com/inference-gateway/inference-gateway/internal/platform/logger"
)

func testEngine(t *testing.T, cfg Config) *Engine {
	t.Helper()
	log, err := logger.NewLogger("test")
	require.NoError(t, err)
	return NewEngine(log, cfg)
}

// seedGGUFs drops placeholder weight files into the shared cache so readiness
// passes without network.
func seedGGUFs(t *testing.T, home string) {
	t.Helper()
	dir := filepath.Join(home, CacheModelsDir)
	require.NoError(t, os.MkdirAll(dir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, BackboneGGUF), []byte("backbone"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, MmprojGGUF), []byte("mmproj"), 0o644))
}

// installFakeBinary puts a sh script named llama-tts first on PATH
// (resolveBinary prefers PATH over the cache); body decides what it "synthesizes".
func installFakeBinary(t *testing.T, body string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("the fake llama-tts binary is a sh script")
	}
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, BinaryName), []byte("#!/bin/sh\n"+body), 0o755))
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func TestSynthesize(t *testing.T) {
	t.Run("passes input through the CLI args and returns the wav", func(t *testing.T) {
		home := t.TempDir()
		seedGGUFs(t, home)
		installFakeBinary(t, `out=""; prev=""
for a in "$@"; do
  [ "$prev" = "--output" ] && out="$a"
  prev="$a"
done
printf '%s' "$*" > "$out"
`)
		e := testEngine(t, Config{AutoDownload: false, Home: home})

		wav, err := e.Synthesize(context.Background(), Request{Input: "Hello world"})
		require.NoError(t, err)
		wavStr := string(wav)
		require.Contains(t, wavStr, "-p Hello world", "input must reach -p")
		require.Contains(t, wavStr, "--tts-lang en")
		require.NotContains(t, wavStr, "--tts-speaker-file", "no reference audio, no speaker flag")

		wav, err = e.Synthesize(context.Background(), Request{Input: "Hallo Welt", Language: "de"})
		require.NoError(t, err)
		require.Contains(t, string(wav), "--tts-lang de", "the requested language must override the default")
	})

	t.Run("hands decoded reference audio to --tts-speaker-file", func(t *testing.T) {
		home := t.TempDir()
		seedGGUFs(t, home)
		installFakeBinary(t, `out=""; ref=""; prev=""
for a in "$@"; do
  [ "$prev" = "--output" ] && out="$a"
  [ "$prev" = "--tts-speaker-file" ] && ref="$a"
  prev="$a"
done
if [ -n "$ref" ]; then cp "$ref" "$out"; fi
`)
		e := testEngine(t, Config{AutoDownload: false, Home: home})

		ref := []byte("RIFF-fake-cloned-voice")
		wav, err := e.Synthesize(context.Background(), Request{Input: "Hello", ReferenceAudio: ref})
		require.NoError(t, err)
		require.Equal(t, ref, wav, "the speaker file must carry the decoded reference bytes")
	})

	t.Run("missing assets fail fast with NotReadyError", func(t *testing.T) {
		e := testEngine(t, Config{AutoDownload: false, Home: t.TempDir()})

		_, err := e.Synthesize(context.Background(), Request{Input: "Hello"})
		var notReady *NotReadyError
		require.ErrorAs(t, err, &notReady)
		require.Contains(t, err.Error(), "AUDIO_LOCAL_AUTO_DOWNLOAD", "error must be actionable")
	})
}

func TestSynthesizeQueuesBeyondConcurrencyLimit(t *testing.T) {
	installFakeBinary(t, `out=""; prev=""
for a in "$@"; do
  [ "$prev" = "--output" ] && out="$a"
  prev="$a"
done
sleep 0.3
printf 'done' > "$out"
`)
	e := testEngine(t, Config{AutoDownload: false, MaxConcurrency: 1, Home: t.TempDir()})
	seedGGUFs(t, e.cfg.Home)

	var wg sync.WaitGroup
	errs := make([]error, 3)
	for i := range errs {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, errs[i] = e.Synthesize(context.Background(), Request{Input: "Hello"})
		}(i)
	}
	wg.Wait()

	for i, err := range errs {
		require.NoError(t, err, "request %d: requests beyond the limit must queue, not fail", i)
	}
}

func sha256Hex(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

// overrideModelSums points the pinned GGUF checksums at test payloads.
func overrideModelSums(t *testing.T, sums map[string]string) {
	t.Helper()
	old := modelSHA256
	modelSHA256 = sums
	t.Cleanup(func() { modelSHA256 = old })
}

func TestWarmupTamperedModelDownloadFails(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("tampered payload"))
	}))
	defer server.Close()

	oldModel := modelRepoBase
	modelRepoBase = server.URL
	t.Cleanup(func() { modelRepoBase = oldModel })
	overrideModelSums(t, map[string]string{
		BackboneGGUF: sha256Hex("the real backbone"),
		MmprojGGUF:   sha256Hex("the real mmproj"),
	})
	installFakeBinary(t, "exit 0\n") // binary present so only models download

	e := testEngine(t, Config{AutoDownload: true, MaxConcurrency: 1, Timeout: 30 * time.Second, Home: t.TempDir()})
	e.Warmup(context.Background())

	ok, detail := e.readiness()
	require.False(t, ok, "tampered download must not make the engine ready")
	require.Contains(t, detail, "sha256 mismatch")
	entries, err := filepath.Glob(filepath.Join(e.modelDir(), "*"))
	require.NoError(t, err)
	require.Empty(t, entries, "no gguf or temp file may survive a failed verification")
}

func TestWarmupDownloadsAssetsOnce(t *testing.T) {
	content := "#!/bin/sh\necho hi\n"
	sum := sha256.Sum256([]byte(content))

	var mu sync.Mutex
	hits := map[string]int{}
	asset := func(name, body string) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			mu.Lock()
			hits[name]++
			mu.Unlock()
			_, _ = w.Write([]byte(body))
		}
	}
	mux := http.NewServeMux()
	mux.Handle("/"+checksumsName, asset(checksumsName, hex.EncodeToString(sum[:])+"  "+binaryAsset()+"\n"))
	mux.Handle("/"+binaryAsset(), asset(binaryFileName(), content))
	mux.Handle("/"+BackboneGGUF, asset(BackboneGGUF, "backbone"))
	mux.Handle("/"+MmprojGGUF, asset(MmprojGGUF, "mmproj"))
	server := httptest.NewServer(mux)
	defer server.Close()

	oldBinary, oldModel := binaryRepoBase, modelRepoBase
	binaryRepoBase, modelRepoBase = server.URL, server.URL
	t.Cleanup(func() { binaryRepoBase, modelRepoBase = oldBinary, oldModel })
	overrideModelSums(t, map[string]string{
		BackboneGGUF: sha256Hex("backbone"),
		MmprojGGUF:   sha256Hex("mmproj"),
	})
	t.Setenv("PATH", t.TempDir())

	home := t.TempDir()
	e := testEngine(t, Config{AutoDownload: true, MaxConcurrency: 1, Timeout: 30 * time.Second, Home: home})
	e.Warmup(context.Background())

	bin, err := os.ReadFile(e.binPath())
	require.NoError(t, err)
	require.Equal(t, content, string(bin), "binary must be sha256-verified and renamed into place")
	info, err := os.Stat(e.binPath())
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0o755), info.Mode().Perm())
	for _, gguf := range []string{BackboneGGUF, MmprojGGUF} {
		body, err := os.ReadFile(filepath.Join(e.modelDir(), gguf))
		require.NoError(t, err)
		require.NotEmpty(t, body)
	}
	for _, dir := range []string{filepath.Dir(e.binPath()), e.modelDir()} {
		leftovers, err := filepath.Glob(filepath.Join(dir, ".*"))
		require.NoError(t, err)
		require.Empty(t, leftovers, "no .part temp files may survive the atomic rename")
	}

	e.Warmup(context.Background())
	mu.Lock()
	defer mu.Unlock()
	for name, n := range hits {
		want := 1
		if name == checksumsName {
			want = 2 // re-read every warmup to spot a newer release
		}
		require.Equal(t, want, n, "asset %s downloaded %d times; cache was not reused", name, n)
	}
	require.Len(t, hits, 4, fmt.Sprintf("unexpected fetches: %v", hits))
}

// releaseServer serves a checksums.txt plus the llama-tts asset for body, and
// counts how often the asset itself was fetched.
func releaseServer(t *testing.T, body string) *atomic.Int64 {
	t.Helper()
	var assetHits atomic.Int64
	mux := http.NewServeMux()
	mux.HandleFunc("/"+checksumsName, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(sha256Hex(body) + "  " + binaryAsset() + "\n"))
	})
	mux.HandleFunc("/"+binaryAsset(), func(w http.ResponseWriter, r *http.Request) {
		assetHits.Add(1)
		_, _ = w.Write([]byte(body))
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	old := binaryRepoBase
	binaryRepoBase = server.URL
	t.Cleanup(func() { binaryRepoBase = old })
	return &assetHits
}

func TestEnsureBinaryUpgradesStaleBinary(t *testing.T) {
	const latest = "#!/bin/sh\necho new\n"

	t.Run("replaces a stale binary in the tools dir", func(t *testing.T) {
		releaseServer(t, latest)
		t.Setenv("PATH", t.TempDir())
		e := testEngine(t, Config{AutoDownload: true, Home: t.TempDir()})
		writeBinary(t, e.binPath(), "#!/bin/sh\necho old\n")

		require.NoError(t, e.ensureBinary(context.Background()))
		require.Equal(t, latest, readFile(t, e.binPath()), "a sha256 mismatch must be replaced")
	})

	t.Run("replaces a stale binary in the legacy bin dir in place", func(t *testing.T) {
		releaseServer(t, latest)
		t.Setenv("PATH", t.TempDir())
		e := testEngine(t, Config{AutoDownload: true, Home: t.TempDir()})
		writeBinary(t, e.legacyBinPath(), "#!/bin/sh\necho old\n")

		require.NoError(t, e.ensureBinary(context.Background()))
		require.Equal(t, latest, readFile(t, e.legacyBinPath()))
		require.NoFileExists(t, e.binPath(), "the legacy copy is upgraded where it lives")
	})

	t.Run("keeps a matching binary", func(t *testing.T) {
		assetHits := releaseServer(t, latest)
		t.Setenv("PATH", t.TempDir())
		e := testEngine(t, Config{AutoDownload: true, Home: t.TempDir()})
		writeBinary(t, e.binPath(), latest)

		require.NoError(t, e.ensureBinary(context.Background()))
		require.Zero(t, assetHits.Load(), "a matching binary must not be re-downloaded")
	})

	t.Run("keeps the cached binary when checksums.txt is unreachable", func(t *testing.T) {
		old := binaryRepoBase
		binaryRepoBase = "http://127.0.0.1:1"
		t.Cleanup(func() { binaryRepoBase = old })
		t.Setenv("PATH", t.TempDir())
		e := testEngine(t, Config{AutoDownload: true, Home: t.TempDir()})
		writeBinary(t, e.binPath(), "#!/bin/sh\necho offline\n")

		require.NoError(t, e.ensureBinary(context.Background()), "offline must not fail warmup")
		require.Equal(t, "#!/bin/sh\necho offline\n", readFile(t, e.binPath()))
	})

	t.Run("leaves a PATH binary alone", func(t *testing.T) {
		installFakeBinary(t, "exit 0\n")
		old := binaryRepoBase
		binaryRepoBase = "http://127.0.0.1:1"
		t.Cleanup(func() { binaryRepoBase = old })
		e := testEngine(t, Config{AutoDownload: true, Home: t.TempDir()})

		require.NoError(t, e.ensureBinary(context.Background()))
		require.NoFileExists(t, e.binPath())
	})
}

func writeBinary(t *testing.T, path, body string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(body), 0o755))
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	body, err := os.ReadFile(path)
	require.NoError(t, err)
	return string(body)
}
