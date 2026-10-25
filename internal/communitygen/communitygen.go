// Package communitygen syncs the models.dev metadata table from the models.dev
// dataset (https://github.com/sst/models.dev). It reads a GitHub tarball of
// that repository, filters it to the gateway's supported cloud providers, and
// emits one JSON table embedded by providers/modelsdev (`task modelsdev:sync`):
// per model, its pricing (USD per-million-token rates converted to the
// gateway's per-token decimal-string format), context window, input and output
// modalities, lifecycle status, and release date.
//
// The table has a hand-maintained companion, "models.overrides.json", for
// models models.dev does not carry (or carries wrong). Its entries are merged
// last, field by field, so they win over the synced values and survive every
// re-sync.
package communitygen

import (
	"archive/tar"
	"cmp"
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"strconv"
	"strings"
	"time"

	toml "github.com/pelletier/go-toml/v2"

	modelsdev "github.com/inference-gateway/inference-gateway/providers/modelsdev"
	types "github.com/inference-gateway/inference-gateway/providers/types"
)

// overridesSuffix names the hand-maintained companion file of the table:
// "models.json" -> "models.overrides.json".
const overridesSuffix = ".overrides.json"

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

// subscriptionProviders are the gateway providers whose models are gated
// behind a paid subscription (e.g. Ollama Cloud Pro). models.dev carries no
// subscription marker, so every "<provider>/<model>" key from these providers
// emits subscription=true: with the published per-token rates when models.dev
// has a cost table (pay-as-you-go is also offered), zero-rate otherwise.
var subscriptionProviders = map[string]bool{
	"ollama_cloud": true,
}

// modelTOML is the subset of a models.dev model file the sync needs. Cost
// rates are USD per million tokens. The cost table is a pointer so an absent
// section ("no per-token price published", e.g. subscription-gated Ollama
// Cloud models) stays distinguishable from an explicit zero rate, which
// models.dev uses for free tiers (e.g. nvidia's free NIM endpoints).
type modelTOML struct {
	Cost        *costTOML `toml:"cost"`
	Status      string    `toml:"status"`
	ReleaseDate string    `toml:"release_date"`
	Limit       struct {
		Context int64 `toml:"context"`
		Output  int64 `toml:"output"`
	} `toml:"limit"`
	Modalities struct {
		Input  []string `toml:"input"`
		Output []string `toml:"output"`
	} `toml:"modalities"`
	// BaseModel references a canonical "models/<lab>/<model>.toml" definition
	// ("<lab>/<model>") whose sections fill in anything this provider file
	// omits (models.dev's base_model inheritance).
	BaseModel string `toml:"base_model"`
}

// costTOML holds a models.dev cost table in either spelling: schema 1 flat
// per-MTok rates, or schema 2 nested under [cost.text.input]/[cost.text.output]
// with cache rates under the input table. The schema 2 cache-write rate is
// either one flat number or a map keyed by cache TTL ("5m", "1h").
type costTOML struct {
	Input      float64 `toml:"input"`
	Output     float64 `toml:"output"`
	CacheRead  float64 `toml:"cache_read"`
	CacheWrite float64 `toml:"cache_write"`
	Text       *struct {
		Input struct {
			Price float64 `toml:"price"`
			Cache struct {
				Read  float64 `toml:"read"`
				Write any     `toml:"write"`
			} `toml:"cache"`
		} `toml:"input"`
		Output struct {
			Price float64 `toml:"price"`
		} `toml:"output"`
	} `toml:"text"`
}

// defaultCacheWriteTTL is the schema 2 cache-write tier equivalent to the
// single schema 1 cache_write rate.
const defaultCacheWriteTTL = "5m"

// rates returns input, output, cache read and cache write as USD per million
// tokens regardless of schema version.
func (c *costTOML) rates() (input, output, cacheRead, cacheWrite float64) {
	if c.Text == nil {
		return c.Input, c.Output, c.CacheRead, c.CacheWrite
	}
	return c.Text.Input.Price, c.Text.Output.Price, c.Text.Input.Cache.Read, cacheWriteRate(c.Text.Input.Cache.Write)
}

// cacheWriteRate resolves a schema 2 cache-write value to the default TTL
// rate: a bare number applies to every TTL, a map is read at "5m".
func cacheWriteRate(value any) float64 {
	if rates, ok := value.(map[string]any); ok {
		value = rates[defaultCacheWriteTTL]
	}
	switch rate := value.(type) {
	case float64:
		return rate
	case int64:
		return float64(rate)
	}
	return 0
}

// GenerateModels reads a models.dev repository tarball (as served by
// `gh api repos/sst/models.dev/tarball`) and writes the per-model metadata
// table keyed by "<provider>/<model>" to output. Models without any usable
// attribute get no entry. Pricing whose rates did not change keeps the prior
// file's updated_at, so re-syncs don't rewrite the whole committed table.
func GenerateModels(output, tarballPath string) error {
	syncedAt := time.Now().UTC().Truncate(time.Second)
	prior := make(map[string]modelsdev.Entry)
	if data, err := os.ReadFile(output); err == nil {
		_ = json.Unmarshal(data, &prior)
	}
	table := make(map[string]modelsdev.Entry)
	err := forEachModel(tarballPath, func(key string, model modelTOML) {
		entry := modelEntry(key, model, syncedAt)
		if old := prior[key].Pricing; old != nil && entry.Pricing != nil && sameRates(*old, *entry.Pricing) {
			entry.Pricing.UpdatedAt = old.UpdatedAt
		}
		if entry != (modelsdev.Entry{}) {
			table[key] = entry
		}
	})
	if err != nil {
		return err
	}
	return writeTable(output, tarballPath, table)
}

// modelEntry maps one models.dev model file to its table row. Each attribute
// is set only when models.dev publishes a usable value for it.
func modelEntry(key string, model modelTOML, syncedAt time.Time) modelsdev.Entry {
	entry := modelsdev.Entry{Status: model.Status}
	if pricing, ok := pricingEntry(key, model, syncedAt); ok {
		entry.Pricing = &pricing
	}
	if model.Limit.Context > 0 {
		entry.ContextWindow = &modelsdev.ContextWindow{
			Context: model.Limit.Context,
			Output:  max(model.Limit.Output, 0),
		}
	}
	modalities := types.ModelModalities{
		Input:  enumModalities(model.Modalities.Input),
		Output: enumModalities(model.Modalities.Output),
	}
	if len(modalities.Input) > 0 && len(modalities.Output) > 0 {
		entry.Modalities = &modalities
	}
	if _, err := time.Parse(time.DateOnly, model.ReleaseDate); err == nil {
		entry.ReleaseDate = model.ReleaseDate
	}
	return entry
}

// enumModalities maps each value through the Modality enum and keeps the
// members in first-seen order without duplicates.
func enumModalities(values []string) []types.Modality {
	var out []types.Modality
	seen := make(map[types.Modality]bool)
	for _, v := range values {
		m := toEnumModality(v)
		if !m.Valid() || seen[m] {
			continue
		}
		seen[m] = true
		out = append(out, m)
	}
	return out
}

// toEnumModality maps a models.dev modality spelling onto the gateway's
// Modality enum: members pass through, MIME-like granular values
// ("image/jpeg") reduce to their base modality, and values with no enum
// base (e.g. "text" subtypes' "application/pdf") keep dropping.
func toEnumModality(value string) types.Modality {
	if m := types.Modality(value); m.Valid() {
		return m
	}
	base, _, _ := strings.Cut(value, "/")
	return types.Modality(base)
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

	type entry struct {
		key   string
		model modelTOML
	}
	bases := make(map[string]modelTOML)
	var entries []entry

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
		baseKey, isBase := canonicalModelKey(hdr.Name)
		if !ok && !isBase {
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
		if isBase {
			bases[baseKey] = model
			continue
		}
		entries = append(entries, entry{key, model})
	}

	for _, e := range entries {
		if base, ok := bases[e.model.BaseModel]; ok {
			e.model = mergeBase(e.model, base)
		}
		visit(e.key, e.model)
	}
	return nil
}

// canonicalModelKey maps a tarball entry like
// "sst-models.dev-abc123/models/openai/gpt-image-2.toml" to its base-model key
// "openai/gpt-image-2". These canonical files are the inheritance targets of
// provider files' base_model references, not table entries themselves.
func canonicalModelKey(name string) (string, bool) {
	if strings.Contains(name, "/providers/") {
		return "", false
	}
	_, rest, ok := strings.Cut(name, "/models/")
	if !ok {
		return "", false
	}
	key, ok := strings.CutSuffix(rest, ".toml")
	if !ok || key == "" {
		return "", false
	}
	return key, true
}

// mergeBase fills the sections a provider model file omits from its canonical
// base model: provider-published values always win, section by section.
func mergeBase(m, base modelTOML) modelTOML {
	if m.Cost == nil {
		m.Cost = base.Cost
	}
	if m.Limit.Context == 0 && m.Limit.Output == 0 {
		m.Limit = base.Limit
	}
	if len(m.Modalities.Input) == 0 && len(m.Modalities.Output) == 0 {
		m.Modalities = base.Modalities
	}
	m.Status = cmp.Or(m.Status, base.Status)
	m.ReleaseDate = cmp.Or(m.ReleaseDate, base.ReleaseDate)
	return m
}

// writeTable merges the hand-maintained overrides over the synced table and
// writes it as indented JSON, refusing to emit an empty table (that means the
// tarball was not a models.dev checkout).
func writeTable(output, tarballPath string, table map[string]modelsdev.Entry) error {
	if len(table) == 0 {
		return fmt.Errorf("no supported provider models found in %s", tarballPath)
	}
	if err := mergeOverrides(output, table); err != nil {
		return err
	}
	data, err := json.MarshalIndent(table, "", "  ")
	if err != nil {
		return fmt.Errorf("encoding models.dev table: %w", err)
	}
	return os.WriteFile(output, append(data, '\n'), 0o644)
}

// mergeOverrides applies the hand-maintained companion of the table
// ("<table>.overrides.json") field by field. Each override decodes onto the
// synced row, so it replaces only the attributes it names. An absent file
// means no overrides.
func mergeOverrides(output string, table map[string]modelsdev.Entry) error {
	path := strings.TrimSuffix(output, ".json") + overridesSuffix
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("reading %s: %w", path, err)
	}
	var overrides map[string]json.RawMessage
	if err := json.Unmarshal(data, &overrides); err != nil {
		return fmt.Errorf("parsing %s: %w", path, err)
	}
	for key, override := range overrides {
		entry := table[key]
		if err := json.Unmarshal(override, &entry); err != nil {
			return fmt.Errorf("parsing override %q in %s: %w", key, path, err)
		}
		table[key] = entry
	}
	return nil
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

// pricingEntry maps one models.dev model file to a community pricing entry.
// Models with a published cost section convert as usual; models of
// subscription-gated providers (no cost section) become zero-rate entries with
// subscription=true; everything else gets no entry.
func pricingEntry(key string, model modelTOML, syncedAt time.Time) (types.Pricing, bool) {
	provider, _, _ := strings.Cut(key, "/")
	var subscription *bool
	if subscriptionProviders[provider] {
		subscription = new(bool)
		*subscription = true
	}
	if model.Cost == nil {
		if subscription == nil {
			return types.Pricing{}, false
		}
		return types.Pricing{
			Currency:       "USD",
			InputPerToken:  "0",
			OutputPerToken: "0",
			Source:         types.PricingSourceCommunity,
			Subscription:   subscription,
			UpdatedAt:      syncedAt,
		}, true
	}
	inputRate, outputRate, cacheRead, cacheWrite := model.Cost.rates()
	input := freeOrRate(inputRate)
	output := freeOrRate(outputRate)
	if input == nil || output == nil {
		return types.Pricing{}, false
	}
	return types.Pricing{
		Currency:           "USD",
		InputPerToken:      *input,
		OutputPerToken:     *output,
		CacheReadPerToken:  perMTokToPerToken(cacheRead),
		CacheWritePerToken: perMTokToPerToken(cacheWrite),
		Source:             types.PricingSourceCommunity,
		Subscription:       subscription,
		UpdatedAt:          syncedAt,
	}, true
}

// sameRates reports whether two pricing entries carry identical rates,
// ignoring UpdatedAt, so re-syncs keep the prior timestamp for unchanged
// entries instead of rewriting the whole committed table.
func sameRates(a, b types.Pricing) bool {
	return a.Currency == b.Currency &&
		a.InputPerToken == b.InputPerToken &&
		a.OutputPerToken == b.OutputPerToken &&
		a.Source == b.Source &&
		eqRate(a.CacheReadPerToken, b.CacheReadPerToken) &&
		eqRate(a.CacheWritePerToken, b.CacheWritePerToken) &&
		subscribed(a) == subscribed(b)
}

func subscribed(p types.Pricing) bool {
	return p.Subscription != nil && *p.Subscription
}

func eqRate(a, b *string) bool {
	return a == b || (a != nil && b != nil && *a == *b)
}

// freeOrRate maps an input/output rate from a present cost section: an
// explicit zero is a published free-tier rate and becomes "0", anything else
// converts as usual. Cache rates keep the plain conversion - a zero cache
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
