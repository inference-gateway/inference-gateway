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
	"time"

	"github.com/open-policy-agent/opa/v1/rego"

	logger "github.com/inference-gateway/inference-gateway/logger"
	otel "github.com/inference-gateway/inference-gateway/otel"
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

// Phase is the point in the request lifecycle a policy runs.
type Phase string

const (
	PhasePreCall    Phase = "pre_call"
	PhasePostCall   Phase = "post_call"
	PhaseToolArgs   Phase = "tool_args"
	PhaseToolOutput Phase = "tool_output"
)

// ---------------------------------------------------------------------------
// FailMode constants - behavior when a guardrail evaluation errors.
// ---------------------------------------------------------------------------

const (
	FailModeOpen   = "open"
	FailModeClosed = "closed"
)

// ---------------------------------------------------------------------------
// Versioned input / decision documents
// ---------------------------------------------------------------------------

// Input is the document passed to every Rego policy query.
type Input struct {
	Method  string `json:"method"`
	Path    string `json:"path"`
	Phase   Phase  `json:"phase"`
	Request *Req   `json:"request,omitempty"`
}

// Req is the request portion of the guardrail input.
type Req struct {
	Body  string `json:"body,omitempty"`
	Model string `json:"model,omitempty"`
}

// Decision is the structured result returned by a policy evaluation.
type Decision struct {
	Action  string `json:"action"`
	Message string `json:"message,omitempty"`
}

// ---------------------------------------------------------------------------
// Evaluator - compiles and evaluates Rego policies.
// ---------------------------------------------------------------------------

// Evaluator compiles .rego files at startup and evaluates them concurrently.
type Evaluator struct {
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
func (e *Evaluator) Eval(ctx context.Context, input *Input) (Decision, error) {
	if e.decision != nil {
		return *e.decision, nil
	}

	inputBytes, err := json.Marshal(input)
	if err != nil {
		return Decision{}, fmt.Errorf("guardrails: marshal input: %w", err)
	}

	results, err := e.query.Eval(ctx, rego.EvalInput(inputBytes))
	if err != nil {
		return Decision{}, fmt.Errorf("guardrails: eval: %w", err)
	}

	if len(results) == 0 || len(results[0].Expressions) == 0 {
		return Decision{Action: ActionAllow}, nil
	}

	dec, ok := results[0].Expressions[0].Value.(map[string]any)
	if !ok {
		return Decision{Action: ActionAllow}, nil
	}

	action, _ := dec["action"].(string)
	if action == "" {
		action = ActionAllow
	}

	message, _ := dec["message"].(string)

	return Decision{
		Action:  action,
		Message: message,
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
	url    string
	client *http.Client
}

// NewExternalClient creates a new external guardrail client.
func NewExternalClient(url string, timeout time.Duration) *ExternalClient {
	return &ExternalClient{
		url: url,
		client: &http.Client{
			Timeout: timeout,
		},
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

// ---------------------------------------------------------------------------
// Tool call evaluation
// ---------------------------------------------------------------------------

// EvaluateToolCall evaluates a tool call against guardrails policies.
// This is called from the MCP agent's ExecuteTools method.
func EvaluateToolCall(
	ctx context.Context,
	evaluator *Evaluator,
	telemetry otel.OpenTelemetry,
	log logger.Logger,
	failMode string,
	toolName string,
	toolArgs string,
	toolOutput string,
	phase Phase,
) error {
	if evaluator == nil {
		return nil
	}

	input := &Input{
		Method: "TOOL_CALL",
		Path:   toolName,
		Phase:  phase,
		Request: &Req{
			Body:  toolArgs,
			Model: "",
		},
	}

	dec, err := evaluator.Eval(ctx, input)
	if err != nil {
		if failMode == FailModeClosed {
			return fmt.Errorf("guardrails: tool call blocked: %w", err)
		}
		log.Warn("guardrails: tool call evaluation error, allowing in open mode", "error", err.Error())
		return nil
	}

	if dec.Action == ActionBlock {
		if telemetry != nil {
			telemetry.RecordGuardrail(ctx, otel.SourceGateway, string(phase), ActionBlock, toolName, "")
		}
		return fmt.Errorf("guardrails: tool call blocked: %s", dec.Message)
	}

	if telemetry != nil {
		telemetry.RecordGuardrail(ctx, otel.SourceGateway, string(phase), dec.Action, toolName, "")
	}

	return nil
}
