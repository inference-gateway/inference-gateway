// Package llamacpp runs open-weights GGUF chat models inside the gateway: a
// reserved local/<owner>/<repo>[:<quant>] model names a Hugging Face GGUF the
// way llama.cpp's -hf flag does, the gateway downloads it into the shared
// ~/.infer cache and serves it through a managed llama-server child process.
// The package knows nothing about HTTP handlers or providers: the API layer
// turns a request into a Reference and proxies to the URL Acquire returns.
package llamacpp

import (
	"cmp"
	"errors"
	"fmt"
	"regexp"
	"strings"
)

const (
	// ModelPrefix reserves the local/ namespace for built-in engines.
	ModelPrefix = "local/"
	// DefaultQuant is the quantization used when the reference names none,
	// matching llama.cpp's -hf default.
	DefaultQuant = "Q4_K_M"

	quantSeparator = ":"
	repoSeparator  = "/"
)

// ErrInvalidReference marks a model id that is not a usable local/ reference.
var ErrInvalidReference = errors.New("invalid local model reference")

// hubSegment matches Hugging Face namespace and repo names as well as the
// quant tag: one path segment, no traversal, so a reference maps to a cache
// directory without escaping it.
var hubSegment = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)

// Reference names a GGUF on Hugging Face: <owner>/<name>[:<quant>] behind the
// local/ prefix. It is the key for the cache and for running servers.
type Reference struct {
	Owner string
	Name  string
	Quant string
}

// ParseReference parses "local/<owner>/<repo>[:<quant>]", defaulting the
// quant to DefaultQuant and upper-casing it (the Hub resolves tags
// case-insensitively, so one canonical form keeps the cache deduplicated).
func ParseReference(model string) (Reference, error) {
	rest, ok := strings.CutPrefix(model, ModelPrefix)
	if !ok {
		return Reference{}, fmt.Errorf("%w: %q lacks the %s prefix", ErrInvalidReference, model, ModelPrefix)
	}
	repo, quant, _ := strings.Cut(rest, quantSeparator)
	owner, name, ok := strings.Cut(repo, repoSeparator)
	quant = strings.ToUpper(cmp.Or(quant, DefaultQuant))
	if !ok || !hubSegment.MatchString(owner) || !hubSegment.MatchString(name) || !hubSegment.MatchString(quant) {
		return Reference{}, fmt.Errorf("%w: %q is not local/<owner>/<repo>[:<quant>]", ErrInvalidReference, model)
	}
	return Reference{Owner: owner, Name: name, Quant: quant}, nil
}

// ParseModels parses the comma-separated LLAMACPP_LOCAL_MODELS list; entries
// may carry or omit the local/ prefix.
func ParseModels(list string) ([]Reference, error) {
	var refs []Reference
	for entry := range strings.SplitSeq(list, ",") {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		if !strings.HasPrefix(entry, ModelPrefix) {
			entry = ModelPrefix + entry
		}
		ref, err := ParseReference(entry)
		if err != nil {
			return nil, err
		}
		refs = append(refs, ref)
	}
	return refs, nil
}

// String is the canonical gateway model id, quant included.
func (r Reference) String() string {
	return ModelPrefix + r.Repo() + quantSeparator + r.Quant
}

// Repo is the Hugging Face repo id, <owner>/<name>.
func (r Reference) Repo() string {
	return r.Owner + repoSeparator + r.Name
}
