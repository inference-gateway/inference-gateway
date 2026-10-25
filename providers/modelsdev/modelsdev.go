package modelsdev

import (
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

// Metadata is the models.dev view of one gateway model, rendered as API values.
// Pointer fields are fresh copies, so callers may keep or mutate them freely.
type Metadata struct {
	Created       int64
	ContextWindow *types.ContextWindow
	Pricing       *types.Pricing
	Modalities    *types.ModelModalities
	Deprecated    bool
}

// table lazily parses the embedded table once. The file is generated and
// committed, so a parse failure is a build defect that degrades to an empty
// table rather than a runtime error.
var table = sync.OnceValue(func() map[string]Entry {
	entries := make(map[string]Entry)
	_ = json.Unmarshal(modelsJSON, &entries)
	return entries
})

// Lookup returns the models.dev metadata of a gateway model ID. Each attribute
// comes from the most specific ID variant that carries it, so a date-pinned
// entry without a status still inherits its base model's status. Unknown
// models yield the zero Metadata.
func Lookup(id string) Metadata {
	return lookup(table(), id)
}

func lookup(entries map[string]Entry, id string) Metadata {
	var merged Entry
	for _, key := range lookupKeys(id) {
		entry, ok := entries[key]
		if !ok {
			continue
		}
		if merged.Pricing == nil {
			merged.Pricing = entry.Pricing
		}
		if merged.ContextWindow == nil && usableContextWindow(entry.ContextWindow) {
			merged.ContextWindow = entry.ContextWindow
		}
		if merged.Modalities == nil {
			merged.Modalities = entry.Modalities
		}
		if merged.Status == "" {
			merged.Status = entry.Status
		}
		if merged.ReleaseDate == "" {
			merged.ReleaseDate = entry.ReleaseDate
		}
	}
	return merged.metadata()
}

// AcceptsImages reports whether the model takes image input per the models.dev
// table. Unknown models return true: stripping images from a request we can't
// classify silently corrupts it, while a text-only provider rejects the image itself.
func AcceptsImages(provider types.Provider, model string) bool {
	modalities := Lookup(string(provider) + "/" + model).Modalities
	return modalities == nil || slices.Contains(modalities.Input, types.ModalityImage)
}

// metadata copies the entry into API values, so nothing handed out aliases the
// shared embedded table.
func (e Entry) metadata() Metadata {
	meta := Metadata{Deprecated: e.Status == deprecatedStatus}
	if released, err := time.Parse(time.DateOnly, e.ReleaseDate); err == nil {
		meta.Created = released.Unix()
	}
	if e.ContextWindow != nil {
		meta.ContextWindow = &types.ContextWindow{
			Tokens: int(e.ContextWindow.Context),
			Source: types.ContextWindowSourceCommunity,
		}
	}
	if e.Pricing != nil {
		pricing := *e.Pricing
		meta.Pricing = &pricing
	}
	if e.Modalities != nil {
		meta.Modalities = &types.ModelModalities{
			Input:  slices.Clone(e.Modalities.Input),
			Output: slices.Clone(e.Modalities.Output),
		}
	}
	return meta
}

func usableContextWindow(window *ContextWindow) bool {
	return window != nil && window.Context > 0 && window.Context <= math.MaxInt
}

// lookupKeys returns candidate table keys for a gateway model ID in preference
// order: exact, without Ollama's ":tag" suffix, without Google's "models/"
// prefix, without a "-latest" alias suffix, and without a trailing "-YYYYMMDD"
// or "-YYYY-MM-DD" date pin. Dotted candidates also get an underscored variant,
// because models.dev file names replace dots in model ids.
func lookupKeys(id string) []string {
	keys := []string{id}
	provider, model, ok := strings.Cut(id, "/")
	if !ok {
		return keys
	}
	if rest, _, found := strings.Cut(model, ":"); found {
		model = rest
		keys = append(keys, provider+"/"+model)
	}
	if rest, found := strings.CutPrefix(model, "models/"); found {
		model = rest
		keys = append(keys, provider+"/"+model)
	}
	if rest, found := strings.CutSuffix(model, "-latest"); found {
		keys = append(keys, provider+"/"+rest)
	}
	if len(model) > 9 && model[len(model)-9] == '-' && isDigits(model[len(model)-8:]) {
		keys = append(keys, provider+"/"+model[:len(model)-9])
	}
	if len(model) > 11 && isDashedDate(model[len(model)-11:]) {
		keys = append(keys, provider+"/"+model[:len(model)-11])
	}
	for _, key := range keys {
		if strings.Contains(key, ".") {
			keys = append(keys, strings.ReplaceAll(key, ".", "_"))
		}
	}
	return keys
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
