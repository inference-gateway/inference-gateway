package modelsdev

import (
	"cmp"
	_ "embed"
	"encoding/json"
	"math"
	"slices"
	"strings"
	"sync"
	"time"

	types "github.com/inference-gateway/inference-gateway/providers/types"
)

// deprecatedStatus is the models.dev lifecycle status of a model the vendor
// has retired from its catalog but may still serve.
const deprecatedStatus = "deprecated"

// maxVariants is the most name variants modelVariants derives from one model.
const maxVariants = 6

// maxStackName is the longest model name underscored on the stack. Longer
// names still resolve, at the cost of one allocation.
const maxStackName = 128

// modelsJSON is the per-model metadata table synced from the community-maintained
// models.dev dataset via `task modelsdev:sync` (internal/communitygen), keyed by
// "<provider>/<model>".
//
//go:embed models.json
var modelsJSON []byte

// ContextWindow is a model's context window in tokens and, when published, its
// maximum output tokens. Only the context window is surfaced through the API.
type ContextWindow struct {
	Context int64 `json:"context"`
	Output  int64 `json:"output,omitempty"`
}

// Entry is one row of the models.dev table. Every attribute is optional, so a
// row carries whatever models.dev (or the overrides file) publishes for it.
type Entry struct {
	Pricing       *types.Pricing         `json:"pricing,omitempty"`
	ContextWindow *ContextWindow         `json:"context_window,omitempty"`
	Modalities    *types.ModelModalities `json:"modalities,omitempty"`
	Status        string                 `json:"status,omitempty"`
	ReleaseDate   string                 `json:"release_date,omitempty"`
}

// Metadata is the models.dev view of one model, rendered as API values once
// when the table loads. Pointer fields are shared by every lookup and every
// listing: read them, never mutate them.
type Metadata struct {
	Created       int64
	ContextWindow *types.ContextWindow
	Pricing       *types.Pricing
	Modalities    *types.ModelModalities
	status        string
}

// Deprecated reports whether models.dev marks the model as deprecated.
func (m Metadata) Deprecated() bool {
	return m.status == deprecatedStatus
}

// Catalog is one provider's share of the table. rows holds the table entries by
// upstream model name. resolved holds every row name, and its dotted spelling,
// already merged across its variants, so a listed name costs one map lookup.
type Catalog struct {
	rows     map[string]Metadata
	resolved map[string]Metadata
}

// catalogs lazily parses the embedded table once into per-provider catalogs.
// The file is generated and committed, so a parse failure is a build defect
// that degrades to an empty table rather than a runtime error.
var catalogs = sync.OnceValue(func() map[types.Provider]Catalog {
	var entries map[string]Entry
	_ = json.Unmarshal(modelsJSON, &entries)
	return group(entries)
})

// group splits "<provider>/<model>" rows into per-provider catalogs of
// pre-rendered, pre-resolved metadata.
func group(entries map[string]Entry) map[types.Provider]Catalog {
	rowsByProvider := make(map[types.Provider]map[string]Metadata)
	for key, entry := range entries {
		provider, model, ok := strings.Cut(key, "/")
		if !ok {
			continue
		}
		rows := rowsByProvider[types.Provider(provider)]
		if rows == nil {
			rows = make(map[string]Metadata)
			rowsByProvider[types.Provider(provider)] = rows
		}
		rows[model] = entry.metadata()
	}
	catalogs := make(map[types.Provider]Catalog, len(rowsByProvider))
	for provider, rows := range rowsByProvider {
		resolved := make(map[string]Metadata, len(rows))
		for name := range rows {
			resolved[name] = resolve(rows, modelVariants(name, nil))
			if dotted := strings.ReplaceAll(name, "_", "."); dotted != name {
				resolved[dotted] = resolve(rows, modelVariants(dotted, nil))
			}
		}
		catalogs[provider] = Catalog{rows: rows, resolved: resolved}
	}
	return catalogs
}

// ForProvider returns the provider's catalog, empty for providers models.dev
// doesn't cover.
func ForProvider(provider types.Provider) Catalog {
	return catalogs()[provider]
}

// Lookup returns the models.dev metadata of a gateway model ID
// ("<provider>/<model>").
func Lookup(id string) Metadata {
	provider, model, ok := strings.Cut(id, "/")
	if !ok {
		return Metadata{}
	}
	return ForProvider(types.Provider(provider)).Lookup(model)
}

// AcceptsImages reports whether the model takes image input per the models.dev
// table. Unknown models return true: stripping images from a request we can't
// classify silently corrupts it, while a text-only provider rejects the image itself.
func AcceptsImages(provider types.Provider, model string) bool {
	modalities := ForProvider(provider).Lookup(model).Modalities
	return modalities == nil || slices.Contains(modalities.Input, types.ModalityImage)
}

// Lookup returns the metadata of an upstream model name. Each attribute comes
// from the most specific name variant carrying it, so a date-pinned entry
// without a status still inherits its base model's status. Dotted names also
// try their underscored file name. Unknown models yield the zero Metadata.
func (c Catalog) Lookup(model string) Metadata {
	if len(c.rows) == 0 {
		return Metadata{}
	}
	if known, ok := c.resolved[model]; ok {
		return known
	}
	var stack [maxVariants]string
	variants := modelVariants(model, stack[:0])
	if len(variants) == 1 && !strings.Contains(model, "_") {
		return Metadata{}
	}
	return resolve(c.rows, variants)
}

// resolve merges the rows of every variant in preference order, then the rows
// of their underscored spellings.
func resolve(rows map[string]Metadata, variants []string) Metadata {
	var merged Metadata
	for _, variant := range variants {
		merged.fill(rows[variant])
	}
	for _, variant := range variants {
		if strings.Contains(variant, ".") {
			var name [maxStackName]byte
			merged.fill(rows[string(underscored(name[:0], variant))])
		}
	}
	return merged
}

// fill sets each attribute m still lacks from other.
func (m *Metadata) fill(other Metadata) {
	m.Created = cmp.Or(m.Created, other.Created)
	m.ContextWindow = cmp.Or(m.ContextWindow, other.ContextWindow)
	m.Pricing = cmp.Or(m.Pricing, other.Pricing)
	m.Modalities = cmp.Or(m.Modalities, other.Modalities)
	m.status = cmp.Or(m.status, other.status)
}

// metadata renders the row as API values. Unusable context windows and
// release dates are dropped so lookups fall through to the next variant.
func (e Entry) metadata() Metadata {
	meta := Metadata{Pricing: e.Pricing, Modalities: e.Modalities, status: e.Status}
	if released, err := time.Parse(time.DateOnly, e.ReleaseDate); err == nil {
		meta.Created = released.Unix()
	}
	if e.ContextWindow != nil && e.ContextWindow.Context > 0 && e.ContextWindow.Context <= math.MaxInt {
		meta.ContextWindow = &types.ContextWindow{
			Tokens: int(e.ContextWindow.Context),
			Source: types.ContextWindowSourceCommunity,
		}
	}
	return meta
}

// modelVariants appends the candidate names of an upstream model to variants in
// preference order: exact, without Ollama's ":tag" suffix, without Google's
// "models/" prefix, without a "-latest" alias suffix, and without a trailing
// "-YYYYMMDD" or "-YYYY-MM-DD" date pin. Every variant is a substring of model.
func modelVariants(model string, variants []string) []string {
	variants = append(variants, model)
	if rest, _, found := strings.Cut(model, ":"); found {
		model = rest
		variants = append(variants, model)
	}
	if rest, found := strings.CutPrefix(model, "models/"); found {
		model = rest
		variants = append(variants, model)
	}
	if rest, found := strings.CutSuffix(model, "-latest"); found {
		variants = append(variants, rest)
	}
	if len(model) > 9 && model[len(model)-9] == '-' && isDigits(model[len(model)-8:]) {
		variants = append(variants, model[:len(model)-9])
	}
	if len(model) > 11 && isDashedDate(model[len(model)-11:]) {
		variants = append(variants, model[:len(model)-11])
	}
	return variants
}

// underscored appends name to dst with dots replaced by underscores, the way
// models.dev file names spell dotted model ids.
func underscored(dst []byte, name string) []byte {
	dst = append(dst, name...)
	for i, b := range dst {
		if b == '.' {
			dst[i] = '_'
		}
	}
	return dst
}

// isDashedDate reports whether s is a "-YYYY-MM-DD" date-pin suffix.
func isDashedDate(s string) bool {
	return len(s) == 11 && s[0] == '-' && s[5] == '-' && s[8] == '-' &&
		isDigits(s[1:5]) && isDigits(s[6:8]) && isDigits(s[9:11])
}

func isDigits(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
