package guardrails

# Redact instead of refuse: returning {"action": "redact"} makes the middleware
# run its built-in detectors over the payload. API keys, bearer tokens and
# credit cards are replaced with [REDACTED]; emails are detected but not
# redacted. The call continues either way - the provider receives a redacted
# request body on pre_call, the client a redacted response body on post_call.
#
# A conditional `main` body must stay mutually exclusive with the other
# policies' bodies (see authz.rego): a body that matched every request would
# override the allow_all default and collide with the others, an OPA conflict
# that blocks under GUARDRAILS_FAIL_MODE=closed. So it only fires on bodies
# holding a secret-shaped value, copying the redactable patterns of
# DefaultDetectors in internal/guardrails/guardrails.go - the middleware owns
# the real detection.
main := {"action": "redact"} if {
    input.phase in {"pre_call", "post_call"}
    secrets_in_body
}

# api_key / token / secret assignments, e.g. "api_key: sk-1234567890abcdef"
secrets_in_body if {
    regex.match(`(?i)(?:api[_-]?key|apikey|token|secret)\s*[:=]\s*['"]?\S{8,}`, input.request.body)
}

# bearer tokens, e.g. "Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9"
secrets_in_body if {
    regex.match(`(?i)bearer\s+[A-Za-z0-9\-._~+/]{20,}`, input.request.body)
}
