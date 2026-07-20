package tests

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"runtime"
	"syscall"
	"testing"

	gin "github.com/gin-gonic/gin"

	constants "github.com/inference-gateway/inference-gateway/providers/constants"
)

// mockModelsPayload builds an OpenAI-compatible model listing with n models,
// each carrying the given extra fields (e.g. a provider-published window).
func mockModelsPayload(b *testing.B, n int, extra map[string]any) []byte {
	b.Helper()

	models := make([]map[string]any, n)
	for i := range n {
		model := map[string]any{
			"id":       fmt.Sprintf("model-%d", i),
			"object":   "model",
			"created":  1750000000,
			"owned_by": "benchmark",
		}
		for k, v := range extra {
			model[k] = v
		}
		models[i] = model
	}
	payload, err := json.Marshal(map[string]any{"object": "list", "data": models})
	if err != nil {
		b.Fatal(err)
	}
	return payload
}

// benchmarkModelsEndpoint drives GET /v1/models?include=context_window and
// reports, on top of the standard ns/op + allocs, the process CPU time per
// request (cpu_ms/op, user+sys via getrusage) and the peak Go heap observed
// across the run (peak_heap_MB), so memory/CPU spikes for large listings are
// visible in `task benchmark` output.
func benchmarkModelsEndpoint(b *testing.B, r *gin.Engine, wantModels int) {
	b.Helper()

	serve := func() *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		req, err := http.NewRequest("GET", "/v1/models?include=context_window", nil)
		if err != nil {
			b.Fatal(err)
		}
		r.ServeHTTP(w, req)
		return w
	}

	w := serve()
	if w.Code != http.StatusOK {
		b.Fatalf("expected 200, got %d", w.Code)
	}
	var response struct {
		Data []json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		b.Fatal(err)
	}
	if len(response.Data) != wantModels {
		b.Fatalf("expected %d models, got %d", wantModels, len(response.Data))
	}

	var memStats runtime.MemStats
	var peakHeap uint64
	var ruBefore, ruAfter syscall.Rusage
	runtime.GC()
	if err := syscall.Getrusage(syscall.RUSAGE_SELF, &ruBefore); err != nil {
		b.Fatal(err)
	}

	b.ReportAllocs()
	for b.Loop() {
		serve()
		runtime.ReadMemStats(&memStats)
		peakHeap = max(peakHeap, memStats.HeapAlloc)
	}

	if err := syscall.Getrusage(syscall.RUSAGE_SELF, &ruAfter); err != nil {
		b.Fatal(err)
	}
	cpuSeconds := float64(ruAfter.Utime.Sec-ruBefore.Utime.Sec) +
		float64(ruAfter.Stime.Sec-ruBefore.Stime.Sec) +
		float64(ruAfter.Utime.Usec-ruBefore.Utime.Usec)/1e6 +
		float64(ruAfter.Stime.Usec-ruBefore.Stime.Usec)/1e6
	b.ReportMetric(cpuSeconds*1e3/float64(b.N), "cpu_ms/op")
	b.ReportMetric(float64(peakHeap)/(1<<20), "peak_heap_MB")
}

// BenchmarkListModelsContextWindow1000ProviderModels measures the provider
// fallback path: 1000 models whose windows come from the listing payload
// itself (max_context_length), so no extra upstream calls are made.
func BenchmarkListModelsContextWindow1000ProviderModels(b *testing.B) {
	const numModels = 1000
	payload := mockModelsPayload(b, numModels, map[string]any{"max_context_length": 32768})

	mux := http.NewServeMux()
	mux.HandleFunc("/proxy/mistral/models", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(payload)
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	r := newContextWindowRouter(b, server, contextWindowProviderConfig(server.URL, constants.MistralID))
	benchmarkModelsEndpoint(b, r, numModels)
}

// BenchmarkListModelsContextWindow1000RuntimeModels measures the worst case:
// 1000 Ollama models, each requiring its own /api/show runtime lookup,
// bounded by the per-request lookup semaphore.
func BenchmarkListModelsContextWindow1000RuntimeModels(b *testing.B) {
	const numModels = 1000
	payload := mockModelsPayload(b, numModels, nil)

	mux := http.NewServeMux()
	mux.HandleFunc("/proxy/ollama/models", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(payload)
	})
	mux.HandleFunc("/api/show", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"parameters":"num_ctx 8192","model_info":{"llama.context_length":131072}}`))
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	r := newContextWindowRouter(b, server, contextWindowProviderConfig(server.URL, constants.OllamaID))
	benchmarkModelsEndpoint(b, r, numModels)
}
