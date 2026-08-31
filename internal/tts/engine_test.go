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
	"testing"
	"time"

	require "github.com/stretchr/testify/require"

	l "github.com/inference-gateway/inference-gateway/logger"
)

func testEngine(t *testing.T, cfg Config) *Engine {
	t.Helper()
	log, err := l.NewLogger("test")
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
	for _, dir := range []string{filepath.Join(home, CacheBinDir), e.modelDir()} {
		leftovers, err := filepath.Glob(filepath.Join(dir, ".*"))
		require.NoError(t, err)
		require.Empty(t, leftovers, "no .part temp files may survive the atomic rename")
	}

	// A second warmup must treat existing files as done and fetch nothing.
	e.Warmup(context.Background())
	mu.Lock()
	defer mu.Unlock()
	for name, n := range hits {
		require.Equal(t, 1, n, "asset %s downloaded %d times; cache was not reused", name, n)
	}
	require.Len(t, hits, 4, fmt.Sprintf("unexpected fetches: %v", hits))
}
