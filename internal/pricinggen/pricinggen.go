// Package pricinggen syncs the community fallback tables from the models.dev
// dataset (https://github.com/sst/models.dev). It reads a GitHub tarball of
// that repository, filters it to the gateway's supported cloud providers, and
// emits the JSON tables embedded by providers/core: model pricing (USD
// per-million-token rates converted to the gateway's per-token decimal-string
// format, `task pricing:sync`) and context windows (`task contextwindow:sync`).
package pricinggen

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	toml "github.com/pelletier/go-toml/v2"

	types "github.com/inference-gateway/inference-gateway/providers/types"
)

// providerDirs maps a models.dev provider directory to the gateway provider
// ID. Local providers (ollama, llamacpp) are intentionally absent: their
// pricing stays null by design.
var providerDirs = map[string]string{
	"anthropic":             "anthropic",
	"cloudflare-workers-ai": "cloudflare",
	"cohere":                "cohere",
	"deepseek":              "deepseek",
	"google":                "google",
	"groq":                  "groq",
	"minimax":               "minimax",
	"mistral":               "mistral",
	"moonshotai":            "moonshot",
	"nvidia":                "nvidia",
	"ollama-cloud":          "ollama_cloud",
	"openai":                "openai",
	"zai":                   "zai",
}

// modelTOML is the subset of a models.dev model file the sync needs. Cost
// rates are USD per million tokens. The cost table is a pointer so an absent
// section ("no per-token price published", e.g. subscription-gated Ollama
// Cloud models) stays distinguishable from an explicit zero rate, which
// models.dev uses for free tiers (e.g. nvidia's free NIM endpoints).
type modelTOML struct {
	Cost *struct {
		Input      float64 `toml:"input"`
		Output     float64 `toml:"output"`
		CacheRead  float64 `toml:"cache_read"`
		CacheWrite float64 `toml:"cache_write"`
	} `toml:"cost"`
	Limit struct {
		Context int64 `toml:"context"`
		Output  int64 `toml:"output"`
	} `toml:"limit"`
}

// contextWindowEntry is one row of the community context-window table: the
// model's context window in tokens and, when published, its maximum output
// tokens.
type contextWindowEntry struct {
	Context int64 `json:"context"`
	Output  int64 `json:"output,omitempty"`
}

// Generate reads a models.dev repository tarball (as served by
// `gh api repos/sst/models.dev/tarball`) and writes the community pricing
// table keyed by "<provider>/<model>" to output.
func Generate(output, tarballPath string) error {
	table := make(map[string]types.ModelPricing)
	err := forEachModel(tarballPath, func(key string, model modelTOML) {
		if model.Cost == nil {
			return
		}
		input := freeOrRate(model.Cost.Input)
		outputRate := freeOrRate(model.Cost.Output)
		if input == nil && outputRate == nil {
			return
		}
		table[key] = types.ModelPricing{
			Currency:           "USD",
			InputPerToken:      input,
			OutputPerToken:     outputRate,
			CacheReadPerToken:  perMTokToPerToken(model.Cost.CacheRead),
			CacheWritePerToken: perMTokToPerToken(model.Cost.CacheWrite),
			Source:             types.PricingSourceCommunity,
		}
	})
	if err != nil {
		return err
	}
	return writeTable(output, tarballPath, table)
}

// GenerateContextWindows reads a models.dev repository tarball and writes the
// community context-window table keyed by "<provider>/<model>" to output.
// Models without a published context limit get no entry and keep rendering as
// explicit nulls.
func GenerateContextWindows(output, tarballPath string) error {
	table := make(map[string]contextWindowEntry)
	err := forEachModel(tarballPath, func(key string, model modelTOML) {
		if model.Limit.Context <= 0 {
			return
		}
		table[key] = contextWindowEntry{
			Context: model.Limit.Context,
			Output:  max(model.Limit.Output, 0),
		}
	})
	if err != nil {
		return err
	}
	return writeTable(output, tarballPath, table)
}

// forEachModel walks a models.dev repository tarball and calls visit for every
// model file that maps to a supported gateway provider.
func forEachModel(tarballPath string, visit func(key string, model modelTOML)) error {
	f, err := os.Open(tarballPath)
	if err != nil {
		return fmt.Errorf("opening models.dev tarball: %w", err)
	}
	defer f.Close()

	gz, err := gzip.NewReader(f)
	if err != nil {
		return fmt.Errorf("reading models.dev tarball: %w", err)
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("reading models.dev tarball: %w", err)
		}
		if hdr.Typeflag != tar.TypeReg {
			continue
		}
		key, ok := tableKey(hdr.Name)
		if !ok {
			continue
		}
		data, err := io.ReadAll(tr)
		if err != nil {
			return fmt.Errorf("reading %s: %w", hdr.Name, err)
		}
		var model modelTOML
		if err := toml.Unmarshal(data, &model); err != nil {
			return fmt.Errorf("parsing %s: %w", hdr.Name, err)
		}
		visit(key, model)
	}
	return nil
}

// writeTable writes a community table as indented JSON, refusing to emit an
// empty table (that means the tarball was not a models.dev checkout).
func writeTable[V any](output, tarballPath string, table map[string]V) error {
	if len(table) == 0 {
		return fmt.Errorf("no supported provider models found in %s", tarballPath)
	}
	data, err := json.MarshalIndent(table, "", "  ")
	if err != nil {
		return fmt.Errorf("encoding community table: %w", err)
	}
	return os.WriteFile(output, append(data, '\n'), 0o644)
}

// tableKey maps a tarball entry like
// "sst-models.dev-abc123/providers/moonshotai/models/kimi-k2.toml" to a
// gateway pricing key like "moonshot/kimi-k2". Nested model paths (e.g.
// cloudflare's "@cf/meta/...") keep their slashes as part of the model ID.
func tableKey(name string) (string, bool) {
	_, rest, ok := strings.Cut(name, "providers/")
	if !ok {
		return "", false
	}
	dir, modelPath, ok := strings.Cut(rest, "/models/")
	if !ok {
		return "", false
	}
	model, ok := strings.CutSuffix(modelPath, ".toml")
	if !ok || model == "" {
		return "", false
	}
	provider, ok := providerDirs[dir]
	if !ok {
		return "", false
	}
	return provider + "/" + model, true
}

// freeOrRate maps an input/output rate from a present cost section: an
// explicit zero is a published free-tier rate and becomes "0", anything else
// converts as usual. Cache rates keep the plain conversion — a zero cache
// rate means "not applicable", matching the gateway's omit-zero convention.
func freeOrRate(perMTok float64) *string {
	if perMTok == 0 {
		zero := "0"
		return &zero
	}
	return perMTokToPerToken(perMTok)
}

// perMTokToPerToken converts a USD-per-million-token rate to a per-token
// decimal string by shifting the decimal point six places, so the committed
// rates never go through float division. Zero and negative rates mean "not
// published" and yield nil.
func perMTokToPerToken(perMTok float64) *string {
	if perMTok <= 0 {
		return nil
	}
	intPart, fracPart, _ := strings.Cut(strconv.FormatFloat(perMTok, 'f', -1, 64), ".")
	digits := intPart + fracPart
	point := len(intPart) - 6
	if point < 0 {
		digits = strings.Repeat("0", -point) + digits
		point = 0
	}
	whole, frac := digits[:point], strings.TrimRight(digits[point:], "0")
	if whole == "" {
		whole = "0"
	}
	if frac == "" {
		return &whole
	}
	rate := whole + "." + frac
	return &rate
}
