//go:build e2e

package tests

import (
	"bytes"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	gateway "github.com/inference-gateway/tokenless/gateway"
)

const (
	e2eListedModels    = 100
	e2eStartupTimeout  = 30 * time.Second
	e2eHealthPoll      = 50 * time.Millisecond
	e2eHealthTimeout   = 2 * time.Second
	e2eAPIKey          = "e2e"
	e2eGatewaySource   = "../cmd/gateway"
	e2eAssistantReply  = "Hello from the e2e upstream."
	e2eMetadataInclude = "context_window,pricing,modalities"
)

// e2eKnownModels are OpenAI model names models.dev carries, so the scripted
// listing exercises both the catalog hit and miss paths.
var e2eKnownModels = []string{
	"chat-latest", "gpt-3.5-turbo", "gpt-4", "gpt-4-turbo", "gpt-4.1", "gpt-4.1-mini", "gpt-4.1-nano",
	"gpt-4o", "gpt-4o-2024-05-13", "gpt-4o-2024-08-06", "gpt-4o-2024-11-20", "gpt-4o-mini",
	"gpt-4o-mini-transcribe", "gpt-4o-mini-tts", "gpt-4o-transcribe", "gpt-4o-transcribe-diarize",
	"gpt-5", "gpt-5-mini", "gpt-5-nano", "gpt-5-pro",
}

// BenchmarkE2E measures the gateway's own per-request cost: the built binary
// serves real HTTP requests while an in-process tokenless mock answers every
// upstream call instantly, so the timings cover middleware, routing, the
// /proxy hop, transforms and JSON, but no provider backend. B/op and
// allocs/op count this process (client and mock), not the gateway's.
func BenchmarkE2E(b *testing.B) {
	upstream := httptest.NewServer(gateway.New(e2eScenarios()))
	b.Cleanup(upstream.Close)
	base := startE2EGateway(b, upstream.URL)
	client := &http.Client{}

	b.Run("ListModels", func(b *testing.B) {
		benchmarkE2ERequest(b, client, http.MethodGet, base+"/v1/models?provider=openai", nil)
	})
	b.Run("ListModelsWithMetadata", func(b *testing.B) {
		benchmarkE2ERequest(b, client, http.MethodGet, base+"/v1/models?provider=openai&include="+e2eMetadataInclude, nil)
	})
	b.Run("ChatCompletions", func(b *testing.B) {
		benchmarkE2ERequest(b, client, http.MethodPost, base+"/v1/chat/completions",
			[]byte(`{"model":"openai/gpt-4o","messages":[{"role":"user","content":"Say hello"}]}`))
	})
	b.Run("ChatCompletionsStream", func(b *testing.B) {
		benchmarkE2ERequest(b, client, http.MethodPost, base+"/v1/chat/completions",
			[]byte(`{"model":"openai/gpt-4o","stream":true,"messages":[{"role":"user","content":"Say hello"}]}`))
	})
	b.Run("MessagesStream", func(b *testing.B) {
		benchmarkE2ERequest(b, client, http.MethodPost, base+"/v1/messages",
			[]byte(`{"model":"anthropic/claude-sonnet-4-5","max_tokens":64,"stream":true,"messages":[{"role":"user","content":"Say hello"}]}`))
	})
}

// e2eScenarios scripts the mock upstream: a listing of known and unknown
// models, and one fixed assistant reply for every chat or messages call.
func e2eScenarios() *gateway.ScenarioFile {
	models := make([]gateway.Model, 0, e2eListedModels)
	for _, name := range e2eKnownModels {
		models = append(models, gateway.Model{ID: name, Object: "model", OwnedBy: "openai"})
	}
	for i := len(models); i < e2eListedModels; i++ {
		models = append(models, gateway.Model{ID: fmt.Sprintf("ft:gpt-4o:e2e-%03d", i), Object: "model", OwnedBy: "openai"})
	}
	return &gateway.ScenarioFile{Fallback: gateway.Turn{Content: e2eAssistantReply}, Models: models}
}

// benchmarkE2ERequest sends one request per iteration over a keep-alive
// connection and drains the full response, streaming ones included.
func benchmarkE2ERequest(b *testing.B, client *http.Client, method, url string, body []byte) {
	b.Helper()

	for b.Loop() {
		req, err := http.NewRequest(method, url, bytes.NewReader(body))
		if err != nil {
			b.Fatal(err)
		}
		req.Header.Set("Content-Type", "application/json")
		resp, err := client.Do(req)
		if err != nil {
			b.Fatal(err)
		}
		_, err = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
		if err != nil {
			b.Fatal(err)
		}
		if resp.StatusCode != http.StatusOK {
			b.Fatalf("%s %s = %d", method, url, resp.StatusCode)
		}
	}
}

// startE2EGateway builds cmd/gateway and runs it with only the environment
// the benchmark sets, pointing OpenAI and Anthropic at upstream. It returns
// the gateway's base URL once /health answers.
func startE2EGateway(b *testing.B, upstream string) string {
	b.Helper()

	bin := filepath.Join(b.TempDir(), "inference-gateway")
	if out, err := exec.Command("go", "build", "-o", bin, e2eGatewaySource).CombinedOutput(); err != nil {
		b.Fatalf("building gateway: %v\n%s", err, out)
	}

	port := e2eFreePort(b)
	cmd := exec.Command(bin)
	cmd.Env = []string{
		"PATH=" + os.Getenv("PATH"),
		"HOME=" + b.TempDir(),
		"SERVER_HOST=127.0.0.1",
		"SERVER_PORT=" + port,
		"OPENAI_API_URL=" + upstream + "/v1",
		"OPENAI_API_KEY=" + e2eAPIKey,
		"ANTHROPIC_API_URL=" + upstream + "/v1",
		"ANTHROPIC_API_KEY=" + e2eAPIKey,
	}
	if err := cmd.Start(); err != nil {
		b.Fatal(err)
	}
	b.Cleanup(func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	})

	base := "http://127.0.0.1:" + port
	e2eWaitHealthy(b, base+"/health")
	return base
}

func e2eFreePort(b *testing.B) string {
	b.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		b.Fatal(err)
	}
	defer listener.Close()
	return strconv.Itoa(listener.Addr().(*net.TCPAddr).Port)
}

func e2eWaitHealthy(b *testing.B, url string) {
	b.Helper()

	client := &http.Client{Timeout: e2eHealthTimeout}
	deadline := time.Now().Add(e2eStartupTimeout)
	for time.Now().Before(deadline) {
		if resp, err := client.Get(url); err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return
			}
		}
		time.Sleep(e2eHealthPoll)
	}
	b.Fatalf("gateway not healthy at %s after %s", url, e2eStartupTimeout)
}
