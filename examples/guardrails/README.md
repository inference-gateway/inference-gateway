# Guardrails

OPA/Rego policies enforced on requests before they reach a provider: pre-call
blocking, secret and PII redaction on requests and responses, and authorization
keyed to the authenticated request. Policies can also delegate to an external
guardrail service on the request and response phases. The
[auth example](../auth/README.md) provides the bearer token the policies read.

## Flavours

- [docker-compose](docker-compose/README.md) - the gateway with the example
  policies from `policies/` (allow-all, block-PII, client-authorization and
  secret-redaction), plus a mock external guardrail service behind
  `GUARDRAILS_EXTERNAL_URL`
- [kubernetes](kubernetes/README.md) - the same policies through the
  [Inference Gateway Operator](https://github.com/inference-gateway/operator)'s
  `spec.guardrails`, which maps onto the `GUARDRAILS_*` settings
