package guardrails

# Redact instead of refuse: returning {"action": "redact"} makes the middleware
# run its built-in detectors over the payload. API keys, bearer tokens and
# Luhn-validated credit cards are replaced with [REDACTED]; emails are detected
# but not redacted. The call continues either way - the provider receives a
# redacted request body on pre_call, the client a redacted response body on
# post_call.
#
# Like every conditional `main` body, keep it mutually exclusive with the other
# policies (see authz.rego): a payload matching two mains with different values
# is an OPA conflict, which blocks under GUARDRAILS_FAIL_MODE=closed.
main := {"action": "redact"} if {
    input.phase in {"pre_call", "post_call"}
}
