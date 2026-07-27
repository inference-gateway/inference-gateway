// Package guardrails provides OPA/Rego policy evaluation, secret/PII detection,
// and an external HTTP guardrail client for the inference gateway middleware.
package guardrails

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/open-policy-agent/opa/v1/rego"
)

// ---------------------------------------------------------------------------
// Action constants - what a policy decision can instruct the gateway to do.
// ---------------------------------------------------------------------------

const (
	ActionAllow  = "allow"
	ActionBlock  = "block"
	ActionRedact = "redact"
	ActionWarn   = "warn"
)

// ---------------------------------------------------------------------------
// Versioned input / decision documents
// ---------------------------------------------------------------------------

// Input is the document passed to every Rego policy query.
type Input struct {
	Method  string `json:"method"`
	Path    string `json:"path"`
	Phase   string `json:"phase"` // "pre_call", "post_call", "tool_call"
	Request *Req   `json:"request,omitempty"`
}

// Req is the request portion of the guardrail input.
type Req struct {
	Body    string            `json:"body,omitempty"`
	Model   string            `json:"model,omitempty"`
	Headers map[string]string `json:"headers,omitempty"`
}

// Decision is the structured result returned by a policy evaluation.
type Decision struct {
	Action      string   `json:"action"`
	Message     string   `json:"message,omitempty"`
	RedactPaths []string `json:"redact_paths,omitempty"`
}

// ---------------------------------------------------------------------------
// Evaluator - compiles and evaluates Rego policies.
// ---------------------------------------------------------------------------

// Evaluator compiles .rego files at startup and evaluates them concurrently.
type Evaluator struct {
	mu       sync.RWMutex
	query    rego.PreparedEvalQuery
	decision *Decision // cached default decision (e.g. allow when no policies)
}

// NewEvaluator loads and compiles all .rego files from dir, then prepares
// a single query for "data.guardrails.main". Returns a no-op evaluator that
// always allows when dir is empty or missing.
func NewEvaluator(ctx context.Context, dir string) (*Evaluator, error) {
	e := &Evaluator{}

	if dir == "" {
		e.decision = &Decision{Action: ActionAllow}
		return e, nil
	}

	modules := make(map[string]string)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			e.decision = &Decision{Action: ActionAllow}
			return e, nil
		}
		return nil, fmt.Errorf("guardrails: reading policy dir %s: %w", dir, err)
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".rego") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			return nil, fmt.Errorf("guardrails: reading %s: %w", entry.Name(), err)
		}
		modules[entry.Name()] = string(data)
	}

	if len(modules) == 0 {
		e.decision = &Decision{Action: ActionAllow}
		return e, nil
	}

	// Build a prepared query from all modules.
	// The policy package is expected to be "guardrails" with a rule "main".
	opts := []func(*rego.Rego){rego.Query("data.guardrails.main")}
	for name, source := range modules {
		opts = append(opts, rego.Module(name, source))
	}
	query, err := rego.New(opts...).PrepareForEval(ctx)
	if err != nil {
		return nil, fmt.Errorf("guardrails: compiling policies: %w", err)
	}

	e.query = query
	return e, nil
}

// Eval evaluates the policy input and returns a decision.
// It is safe for concurrent use.
func (e *Evaluator) Eval(ctx context.Context, input *Input) (*Decision, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	// No-op evaluator: always allow.
	if e.decision != nil {
		return e.decision, nil
	}

	inputBytes, err := json.Marshal(input)
	if err != nil {
		return nil, fmt.Errorf("guardrails: marshal input: %w", err)
	}

	results, err := e.query.Eval(ctx, rego.EvalInput(inputBytes))
	if err != nil {
		return nil, fmt.Errorf("guardrails: eval: %w", err)
	}

	if len(results) == 0 || len(results[0].Expressions) == 0 {
		return &Decision{Action: ActionAllow}, nil
	}

	dec, ok := results[0].Expressions[0].Value.(map[string]any)
	if !ok {
		return &Decision{Action: ActionAllow}, nil
	}

	action, _ := dec["action"].(string)
	if action == "" {
		action = ActionAllow
	}

	message, _ := dec["message"].(string)

	var redactPaths []string
	if rp, ok := dec["redact_paths"].([]any); ok {
		for _, p := range rp {
			if s, ok := p.(string); ok {
				redactPaths = append(redactPaths, s)
			}
		}
	}

	return &Decision{
		Action:      action,
		Message:     message,
		RedactPaths: redactPaths,
	}, nil
}

// ---------------------------------------------------------------------------
// Detectors - stdlib regexp-based secret and PII detection.
// ---------------------------------------------------------------------------

// DetectorResult describes a single match found by a detector.
type DetectorResult struct {
	Type   string `json:"type"`
	Value  string `json:"value,omitempty"`
	Redact bool   `json:"redact"`
}

// Detector is a compiled pattern that finds secrets or PII in text.
type Detector struct {
	Type    string
	Pattern *regexp.Regexp
	Redact  bool
}

// DefaultDetectors returns a set of built-in detectors.
func DefaultDetectors() []Detector {
	return []Detector{
		{
			Type:    "api_key",
			Pattern: regexp.MustCompile(`(?i)(?:api[_-]?key|apikey|token|secret)\s*[:=]\s*['"]?\S{8,}`),
			Redact:  true,
		},
		{
			Type:    "bearer_token",
			Pattern: regexp.MustCompile(`(?i)bearer\s+[A-Za-z0-9\-._~+/]{20,}`),
			Redact:  true,
		},
		{
			Type:    "credit_card",
			Pattern: regexp.MustCompile(`\b(?:\d[ -]*?){13,19}\b`),
			Redact:  true,
		},
		{
			Type:    "email",
			Pattern: regexp.MustCompile(`\b[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}\b`),
			Redact:  false,
		},
	}
}

// ScanDetectors runs all detectors against the given text and returns matches.
func ScanDetectors(text string, detectors []Detector) []DetectorResult {
	var results []DetectorResult
	for _, d := range detectors {
		matches := d.Pattern.FindAllString(text, -1)
		for _, m := range matches {
			// Luhn check for credit-card-like patterns.
			if d.Type == "credit_card" {
				cleaned := strings.Map(func(r rune) rune {
					if r >= '0' && r <= '9' {
						return r
					}
					return -1
				}, m)
				if len(cleaned) < 13 || len(cleaned) > 19 || !luhnCheck(cleaned) {
					continue
				}
			}
			results = append(results, DetectorResult{
				Type:   d.Type,
				Value:  m,
				Redact: d.Redact,
			})
		}
	}
	return results
}

// luhnCheck validates a digit string using the Luhn algorithm.
func luhnCheck(s string) bool {
	var sum int
	alt := false
	for i := len(s) - 1; i >= 0; i-- {
		d := int(s[i] - '0')
		if d < 0 || d > 9 {
			return false
		}
		if alt {
			d *= 2
			if d > 9 {
				d -= 9
			}
		}
		sum += d
		alt = !alt
	}
	return sum%10 == 0
}

// RedactSensitive replaces all redactable matches in text with "[REDACTED]".
func RedactSensitive(text string, detectors []Detector) string {
	for _, d := range detectors {
		if d.Redact {
			text = d.Pattern.ReplaceAllString(text, "[REDACTED]")
		}
	}
	return text
}

// ---------------------------------------------------------------------------
// External HTTP guardrail client
// ---------------------------------------------------------------------------

// ExternalClient sends requests to an external guardrail service.
type ExternalClient struct {
	url     string
	client  *http.Client
	timeout time.Duration
}

// NewExternalClient creates a new external guardrail client.
func NewExternalClient(url string, timeout time.Duration) *ExternalClient {
	return &ExternalClient{
		url: url,
		client: &http.Client{
			Timeout: timeout,
		},
		timeout: timeout,
	}
}

// Check sends an input to the external guardrail service and returns the decision.
func (e *ExternalClient) Check(ctx context.Context, input *Input) (*Decision, error) {
	if e.url == "" {
		return &Decision{Action: ActionAllow}, nil
	}

	body, err := json.Marshal(input)
	if err != nil {
		return nil, fmt.Errorf("guardrails: external marshal: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, e.url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("guardrails: external request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := e.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("guardrails: external call: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("guardrails: external read: %w", err)
	}

	var dec Decision
	if err := json.Unmarshal(respBody, &dec); err != nil {
		return nil, fmt.Errorf("guardrails: external decode: %w", err)
	}

	if dec.Action == "" {
		dec.Action = ActionAllow
	}

	return &dec, nil
}
