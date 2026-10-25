package communitygen

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	modelsdev "github.com/inference-gateway/inference-gateway/providers/modelsdev"
	types "github.com/inference-gateway/inference-gateway/providers/types"
)

func TestPerMTokToPerToken(t *testing.T) {
	tests := []struct {
		name    string
		perMTok float64
		want    string
	}{
		{"whole dollars", 3, "0.000003"},
		{"sub-dollar", 0.59, "0.00000059"},
		{"cents precision", 15.075, "0.000015075"},
		{"fraction of a cent", 0.0028, "0.0000000028"},
		{"large rate keeps integer part", 2500000, "2.5"},
		{"exactly one dollar per token", 1000000, "1"},
		{"zero is unpublished", 0, ""},
		{"negative is unpublished", -1, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := perMTokToPerToken(tt.perMTok)
			if tt.want == "" {
				if got != nil {
					t.Fatalf("perMTokToPerToken(%v) = %q, want nil", tt.perMTok, *got)
				}
				return
			}
			if got == nil || *got != tt.want {
				t.Fatalf("perMTokToPerToken(%v) = %v, want %q", tt.perMTok, got, tt.want)
			}
		})
	}
}

func TestTableKey(t *testing.T) {
	tests := []struct {
		name string
		path string
		want string
	}{
		{"mapped provider", "sst-models.dev-abc/providers/moonshotai/models/kimi-k2.toml", "moonshot/kimi-k2"},
		{"nested model path", "sst-models.dev-abc/providers/cloudflare-workers-ai/models/@cf/meta/llama-3.1-8b.toml", "cloudflare/@cf/meta/llama-3.1-8b"},
		{"unsupported provider", "sst-models.dev-abc/providers/openrouter/models/auto.toml", ""},
		{"provider metadata file", "sst-models.dev-abc/providers/openai/provider.toml", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := tableKey(tt.path)
			if ok != (tt.want != "") || got != tt.want {
				t.Fatalf("tableKey(%q) = %q, %v, want %q", tt.path, got, ok, tt.want)
			}
		})
	}
}

// writeTarball builds a gzip'd tar of name→body entries mimicking a
// models.dev repository tarball and returns its path.
func writeTarball(t *testing.T, files map[string]string) string {
	t.Helper()

	tarball := filepath.Join(t.TempDir(), "models.dev.tar.gz")
	f, err := os.Create(tarball)
	if err != nil {
		t.Fatal(err)
	}
	gz := gzip.NewWriter(f)
	tw := tar.NewWriter(gz)
	for name, body := range files {
		if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0o644, Size: int64(len(body)), Typeflag: tar.TypeReg}); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write([]byte(body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	return tarball
}

// syncModels runs GenerateModels over a tarball of the given files into output
// and returns the decoded table.
func syncModels(t *testing.T, output string, files map[string]string) map[string]modelsdev.Entry {
	t.Helper()

	if err := GenerateModels(output, writeTarball(t, files)); err != nil {
		t.Fatalf("GenerateModels() = %v", err)
	}
	var table map[string]modelsdev.Entry
	readJSON(t, output, &table)
	return table
}

// readJSON decodes a generated table file into v.
func readJSON(t *testing.T, path string, v any) {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, v); err != nil {
		t.Fatal(err)
	}
}

// TestGenerateModels_FreeVsUnpublished distinguishes the cost states in
// models.dev files: an explicit zero cost is a free tier and must emit "0"
// rates, an absent cost section means no per-token price unless the model
// belongs to a subscription-gated provider (then it gets a zero-rate pricing
// with subscription=true), and a priced model converts as usual.
func TestGenerateModels_FreeVsUnpublished(t *testing.T) {
	table := syncModels(t, filepath.Join(t.TempDir(), "models.json"), map[string]string{
		"sst-models.dev-abc/providers/nvidia/models/meta/llama-free.toml":       "[cost]\ninput = 0.0\noutput = 0.0\n",
		"sst-models.dev-abc/providers/ollama-cloud/models/kimi-sub.toml":        "name = \"kimi-sub\"\n",
		"sst-models.dev-abc/providers/ollama-cloud/models/deepseek-v4-pro.toml": "name = \"deepseek-v4-pro\"\n",
		"sst-models.dev-abc/providers/ollama-cloud/models/glm-priced.toml":      "[cost]\ninput = 0.15\noutput = 0.5\n",
		"sst-models.dev-abc/providers/openai/models/gpt-paid.toml":              "[cost]\ninput = 3.0\noutput = 15.0\n",
		"sst-models.dev-abc/providers/openai/models/gpt-unpriced.toml":          "[limit]\ncontext = 8192\n",
		"sst-models.dev-abc/providers/anthropic/models/claude-v2.toml":          "schema = 2\n[cost.text.input]\nprice = 4\ncache = { read = 0.2, write = { \"5m\" = 5, \"1h\" = 8 } }\n[cost.text.output]\nprice = 20\n",
		"sst-models.dev-abc/providers/minimax/models/flat-write.toml":           "schema = 2\n[cost.text.input]\nprice = 0.3\ncache = { read = 0.03, write = 0.375 }\n[cost.text.output]\nprice = 1.2\n",
	})

	schema2 := table["anthropic/claude-v2"].Pricing
	if schema2 == nil {
		t.Fatal("schema 2 priced model missing pricing")
	}
	if schema2.InputPerToken != "0.000004" || schema2.OutputPerToken != "0.00002" {
		t.Errorf("schema 2 rates = %v/%v, want 0.000004/0.00002", schema2.InputPerToken, schema2.OutputPerToken)
	}
	if schema2.CacheReadPerToken == nil || *schema2.CacheReadPerToken != "0.0000002" {
		t.Errorf("schema 2 cache read = %v, want 0.0000002", schema2.CacheReadPerToken)
	}
	if schema2.CacheWritePerToken == nil || *schema2.CacheWritePerToken != "0.000005" {
		t.Errorf("schema 2 cache write = %v, want 0.000005", schema2.CacheWritePerToken)
	}

	flatWrite := table["minimax/flat-write"].Pricing
	if flatWrite == nil || flatWrite.CacheWritePerToken == nil || *flatWrite.CacheWritePerToken != "0.000000375" {
		t.Errorf("flat cache write pricing = %+v, want cache write 0.000000375", flatWrite)
	}

	free := table["nvidia/meta/llama-free"].Pricing
	if free == nil || free.InputPerToken != "0" || free.OutputPerToken != "0" {
		t.Errorf("free-tier pricing = %+v, want \"0\"/\"0\"", free)
	}
	for _, key := range []string{"ollama_cloud/kimi-sub", "ollama_cloud/deepseek-v4-pro"} {
		sub := table[key].Pricing
		if sub == nil {
			t.Fatalf("subscription-gated provider model %q missing pricing", key)
		}
		if sub.InputPerToken != "0" || sub.OutputPerToken != "0" {
			t.Errorf("%s subscription rates = %v/%v, want \"0\"/\"0\"", key, sub.InputPerToken, sub.OutputPerToken)
		}
		if sub.Subscription == nil || !*sub.Subscription {
			t.Errorf("%s must have subscription=true", key)
		}
	}
	pricedSub := table["ollama_cloud/glm-priced"].Pricing
	if pricedSub == nil || pricedSub.InputPerToken != "0.00000015" {
		t.Errorf("priced subscription model pricing = %+v, want input_per_token 0.00000015", pricedSub)
	}
	if pricedSub == nil || pricedSub.Subscription == nil || !*pricedSub.Subscription {
		t.Error("priced subscription-provider model must keep subscription=true")
	}
	if paid := table["openai/gpt-paid"].Pricing; paid == nil || paid.InputPerToken != "0.000003" {
		t.Errorf("paid model pricing = %+v, want input_per_token 0.000003", paid)
	}
	if unpriced := table["openai/gpt-unpriced"]; unpriced.Pricing != nil || unpriced.ContextWindow == nil {
		t.Errorf("model without a cost section = %+v, want a context window and no pricing", unpriced)
	}
}

// TestGenerateModels_PreservesTimestampsForUnchangedRates re-syncs over an
// existing table: pricing whose rates did not change keeps its prior
// updated_at, while changed pricing gets a fresh stamp.
func TestGenerateModels_PreservesTimestampsForUnchangedRates(t *testing.T) {
	output := filepath.Join(t.TempDir(), "models.json")
	prior := `{
		"openai/gpt-same": {"pricing": {"currency":"USD","input_per_token":"0.000003","output_per_token":"0.000015","source":"community","updated_at":"2020-01-02T03:04:05Z"}},
		"openai/gpt-changed": {"pricing": {"currency":"USD","input_per_token":"0.000003","output_per_token":"0.000015","source":"community","updated_at":"2020-01-02T03:04:05Z"}}
	}`
	if err := os.WriteFile(output, []byte(prior), 0o644); err != nil {
		t.Fatal(err)
	}

	table := syncModels(t, output, map[string]string{
		"sst-models.dev-abc/providers/openai/models/gpt-same.toml":    "[cost]\ninput = 3.0\noutput = 15.0\n",
		"sst-models.dev-abc/providers/openai/models/gpt-changed.toml": "[cost]\ninput = 4.0\noutput = 15.0\n",
	})

	if got := table["openai/gpt-same"].Pricing.UpdatedAt.Format(time.RFC3339); got != "2020-01-02T03:04:05Z" {
		t.Errorf("unchanged pricing updated_at = %s, want prior timestamp preserved", got)
	}
	if got := table["openai/gpt-changed"].Pricing.UpdatedAt.Year(); got == 2020 {
		t.Error("changed pricing must get a fresh updated_at, kept the prior one")
	}
}

// TestGenerateModels_Overrides checks the hand-maintained companion file is
// merged field by field: models models.dev does not carry are added, an
// overridden attribute wins, and the row's other synced attributes survive.
func TestGenerateModels_Overrides(t *testing.T) {
	dir := t.TempDir()
	output := filepath.Join(dir, "models.json")
	overrides := `{
		"anthropic/claude-opus-5": {"context_window": {"context": 1000000, "output": 128000}},
		"openai/gpt-paid": {"pricing": {"currency":"USD","input_per_token":"0.000009","output_per_token":"0.000015","source":"community","updated_at":"2026-07-27T09:42:20Z"}},
		"openai/tts-1": {"modalities": {"input": ["text"], "output": ["audio"]}}
	}`
	if err := os.WriteFile(filepath.Join(dir, "models"+overridesSuffix), []byte(overrides), 0o644); err != nil {
		t.Fatal(err)
	}

	table := syncModels(t, output, map[string]string{
		"sst-models.dev-abc/providers/openai/models/gpt-paid.toml":      "[cost]\ninput = 3.0\noutput = 15.0\n[limit]\ncontext = 128000\n",
		"sst-models.dev-abc/providers/anthropic/models/claude-ctx.toml": "[cost]\ninput = 5.0\noutput = 25.0\n[limit]\ncontext = 200000\n",
	})

	if got := table["anthropic/claude-opus-5"].ContextWindow; got == nil || got.Context != 1000000 || got.Output != 128000 {
		t.Errorf("override-only context window = %+v, want context 1000000 output 128000", got)
	}
	paid := table["openai/gpt-paid"]
	if paid.Pricing == nil || paid.Pricing.InputPerToken != "0.000009" {
		t.Errorf("overridden pricing = %+v, want the override to win", paid.Pricing)
	}
	if paid.ContextWindow == nil || paid.ContextWindow.Context != 128000 {
		t.Errorf("synced context window of an overridden row = %+v, want it kept", paid.ContextWindow)
	}
	if got := table["openai/tts-1"].Modalities; got == nil || !slices.Equal(got.Output, []types.Modality{types.ModalityAudio}) {
		t.Errorf("override-only modalities = %+v, want audio output", got)
	}
	if table["anthropic/claude-ctx"].Pricing == nil {
		t.Error("synced model dropped by the override merge")
	}
}

// TestGenerateModels_ContextWindows covers the limit states in models.dev
// files: a published context limit emits a context window (with output tokens
// when present), and an absent limit section emits none.
func TestGenerateModels_ContextWindows(t *testing.T) {
	table := syncModels(t, filepath.Join(t.TempDir(), "models.json"), map[string]string{
		"sst-models.dev-abc/providers/openai/models/gpt-full.toml":       "[limit]\ncontext = 128000\noutput = 16384\n",
		"sst-models.dev-abc/providers/anthropic/models/claude-ctx.toml":  "[limit]\ncontext = 200000\n",
		"sst-models.dev-abc/providers/ollama-cloud/models/no-limit.toml": "name = \"no-limit\"\n",
	})

	if got := table["openai/gpt-full"].ContextWindow; got == nil || got.Context != 128000 || got.Output != 16384 {
		t.Errorf("openai/gpt-full context window = %+v, want context 128000 output 16384", got)
	}
	if got := table["anthropic/claude-ctx"].ContextWindow; got == nil || got.Context != 200000 || got.Output != 0 {
		t.Errorf("anthropic/claude-ctx context window = %+v, want context 200000 output 0", got)
	}
	if got := table["ollama_cloud/no-limit"].ContextWindow; got != nil {
		t.Errorf("model without a limit section must have no context window, got %+v", got)
	}
}

// TestGenerateModels_StatusesAndReleaseDates covers the lifecycle and
// release-date fields: values emit, base-model values are inherited, a
// malformed date is dropped, a model with no usable attribute gets no entry,
// and unsupported providers are skipped.
func TestGenerateModels_StatusesAndReleaseDates(t *testing.T) {
	table := syncModels(t, filepath.Join(t.TempDir(), "models.json"), map[string]string{
		"sst-models.dev-abc/providers/minimax/models/MiniMax-M2.toml":   "status = \"deprecated\"\n",
		"sst-models.dev-abc/providers/openai/models/gpt-beta.toml":      "status = \"beta\"\nrelease_date = \"2025-03\"\n",
		"sst-models.dev-abc/providers/openai/models/gpt-current.toml":   "name = \"gpt-current\"\n",
		"sst-models.dev-abc/providers/openai/models/gpt-inherited.toml": "base_model = \"openai/gpt-old\"\n",
		"sst-models.dev-abc/models/openai/gpt-old.toml":                 "status = \"deprecated\"\nrelease_date = \"2023-03-01\"\n",
		"sst-models.dev-abc/providers/cohere/models/command-a.toml":     "release_date = \"2025-03-13\"\n",
		"sst-models.dev-abc/providers/openrouter/models/skipped.toml":   "status = \"deprecated\"\nrelease_date = \"2025-03-13\"\n",
	})

	want := map[string]modelsdev.Entry{
		"minimax/MiniMax-M2":   {Status: "deprecated"},
		"openai/gpt-beta":      {Status: "beta"},
		"openai/gpt-inherited": {Status: "deprecated", ReleaseDate: "2023-03-01"},
		"cohere/command-a":     {ReleaseDate: "2025-03-13"},
	}
	if !maps.Equal(table, want) {
		t.Errorf("table = %+v, want %+v", table, want)
	}
}

// TestEnumModalities keeps only known Modality members, in first-seen
// order without duplicates: MIME-like granular values from models.dev's
// schema 2 files ("image/jpeg") reduce to their base ("image"), while
// values with no enum base (e.g. "pdf", "application/pdf") are dropped.
func TestEnumModalities(t *testing.T) {
	got := enumModalities([]string{
		"text", "pdf", "image", "text", "bogus", "audio",
		"image/jpeg", "image/png", "audio/mpeg", "video/mp4",
		"application/pdf", "image/webp",
	})
	want := []types.Modality{
		types.ModalityText,
		types.ModalityImage,
		types.ModalityAudio,
		types.ModalityVideo,
	}
	if !slices.Equal(got, want) {
		t.Errorf("enumModalities() = %v, want %v", got, want)
	}
	if got := enumModalities([]string{"pdf", "bogus"}); got != nil {
		t.Errorf("enumModalities(all-unknown) = %v, want nil", got)
	}
	if got := enumModalities([]string{"application/pdf"}); got != nil {
		t.Errorf("enumModalities(non-modality MIME) = %v, want nil", got)
	}
}
