package guardrails

# Default allow-all policy.
# Override with more specific rules in additional .rego files.
main := {"action": "allow"}
