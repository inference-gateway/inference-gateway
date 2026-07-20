package pricinggen

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

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

// TestGenerate_FreeVsUnpublished distinguishes the three cost states in
// models.dev files: an explicit zero cost is a free tier and must emit "0"
// rates, an absent cost section means no per-token price and must emit no
// entry, and a priced model converts as usual.
func TestGenerate_FreeVsUnpublished(t *testing.T) {
	files := map[string]string{
		"sst-models.dev-abc/providers/nvidia/models/meta/llama-free.toml": "[cost]\ninput = 0.0\noutput = 0.0\n",
		"sst-models.dev-abc/providers/ollama-cloud/models/kimi-sub.toml":  "name = \"kimi-sub\"\n",
		"sst-models.dev-abc/providers/openai/models/gpt-paid.toml":        "[cost]\ninput = 3.0\noutput = 15.0\n",
	}

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

	output := filepath.Join(t.TempDir(), "pricing.json")
	if err := Generate(output, tarball); err != nil {
		t.Fatalf("Generate() = %v", err)
	}

	data, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	var table map[string]types.ModelPricing
	if err := json.Unmarshal(data, &table); err != nil {
		t.Fatal(err)
	}

	free, ok := table["nvidia/meta/llama-free"]
	if !ok {
		t.Fatal("free-tier model with explicit zero cost missing from table")
	}
	if free.InputPerToken == nil || *free.InputPerToken != "0" || free.OutputPerToken == nil || *free.OutputPerToken != "0" {
		t.Errorf("free-tier rates = %v/%v, want \"0\"/\"0\"", free.InputPerToken, free.OutputPerToken)
	}
	if _, ok := table["ollama_cloud/kimi-sub"]; ok {
		t.Error("model without a cost section must not get an entry")
	}
	paid, ok := table["openai/gpt-paid"]
	if !ok || paid.InputPerToken == nil || *paid.InputPerToken != "0.000003" {
		t.Errorf("paid model entry = %+v, want input_per_token 0.000003", paid)
	}
}
