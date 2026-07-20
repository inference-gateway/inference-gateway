package pricinggen

import "testing"

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
