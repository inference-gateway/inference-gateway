// Package llamacpptest stands in for llama-server in tests. The test binary
// re-executes itself as the fake when EnvRun is set, so the runtime under
// test spawns, health-polls, proxies to and stops a real child process. A
// package's TestMain calls MaybeRun first; tests call Install.
package llamacpptest

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	require "github.com/stretchr/testify/require"

	llamacpp "github.com/inference-gateway/inference-gateway/internal/llamacpp"
)

const (
	// EnvRun turns the test binary into the fake server.
	EnvRun = "INFERENCE_GATEWAY_FAKE_LLAMA_SERVER"
	// EnvLoadDelay delays the first healthy /health answer by a duration.
	EnvLoadDelay = EnvRun + "_LOAD_DELAY"
	// EnvExitCode makes the fake exit at once with that code, like a server
	// that cannot load its model.
	EnvExitCode = EnvRun + "_EXIT"

	// ModelFile is the placeholder GGUF Seed writes.
	ModelFile = "model.gguf"
	// ArgsPrefix starts every completion the fake returns, followed by the
	// arguments it was launched with, so tests can assert load options.
	ArgsPrefix = "args: "

	portFlag       = "--port"
	loopback       = "127.0.0.1"
	windowsOS      = "windows"
	cacheDirMode   = 0o755
	cacheFileMode  = 0o644
	completionID   = "chatcmpl-fake"
	sseContentType = "text/event-stream"
)

// MaybeRun runs the fake server and exits when the process was launched as
// one; otherwise it returns so the tests run.
func MaybeRun() {
	if os.Getenv(EnvRun) == "" {
		return
	}
	os.Exit(run(os.Args[1:]))
}

// Install puts the test binary first on PATH under name, so the runtime
// resolves and spawns it as llama-server.
func Install(t *testing.T, name string) {
	t.Helper()
	if runtime.GOOS == windowsOS {
		t.Skip("the fake llama-server re-executes the test binary through a symlink")
	}
	exe, err := os.Executable()
	require.NoError(t, err)
	dir := t.TempDir()
	require.NoError(t, os.Symlink(exe, filepath.Join(dir, name)))
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv(EnvRun, "1")
}

// Seed plants a complete cache entry for ref (manifest plus placeholder
// GGUF) under home so the runtime finds it without network.
func Seed(t *testing.T, home string, ref llamacpp.Reference) {
	t.Helper()
	dir := filepath.Join(home, llamacpp.CacheModelsDir, ref.Owner, ref.Name)
	require.NoError(t, os.MkdirAll(dir, cacheDirMode))
	manifest, err := json.Marshal(llamacpp.Manifest{Model: ModelFile, Files: []llamacpp.File{{Name: ModelFile}}})
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dir, ref.Quant+llamacpp.ManifestSuffix), manifest, cacheFileMode))
	require.NoError(t, os.WriteFile(filepath.Join(dir, ModelFile), []byte("gguf"), cacheFileMode))
}

func run(args []string) int {
	if code := os.Getenv(EnvExitCode); code != "" {
		fmt.Fprintln(os.Stderr, "fake llama-server: failing on purpose")
		n, _ := strconv.Atoi(code)
		return n
	}
	delay, _ := time.ParseDuration(os.Getenv(EnvLoadDelay))
	started := time.Now()
	content := ArgsPrefix + strings.Join(args, " ")

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) {
		if time.Since(started) < delay {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})
	mux.HandleFunc("POST /v1/chat/completions", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Model  string `json:"model"`
			Stream bool   `json:"stream"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		if req.Stream {
			w.Header().Set("Content-Type", sseContentType)
			chunk, _ := json.Marshal(map[string]any{
				"id": completionID, "object": "chat.completion.chunk", "model": req.Model,
				"choices": []map[string]any{{"index": 0, "delta": map[string]string{"role": "assistant", "content": content}}},
			})
			fmt.Fprintf(w, "data: %s\n\ndata: [DONE]\n\n", chunk)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id": completionID, "object": "chat.completion", "model": req.Model,
			"choices": []map[string]any{{"index": 0, "finish_reason": "stop", "message": map[string]string{"role": "assistant", "content": content}}},
		})
	})

	addr := net.JoinHostPort(loopback, flagValue(args, portFlag))
	if err := http.ListenAndServe(addr, mux); err != nil {
		fmt.Fprintln(os.Stderr, "fake llama-server:", err)
		return 1
	}
	return 0
}

func flagValue(args []string, flag string) string {
	for i, a := range args {
		if a == flag && i+1 < len(args) {
			return args[i+1]
		}
	}
	return ""
}
