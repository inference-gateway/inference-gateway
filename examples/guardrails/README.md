# Guardrails

OPA/Rego policies enforced on requests before they reach a provider: pre-call
blocking, PII and secret detection, and authorization keyed to the authenticated
request. The [auth example](../auth/README.md) provides the bearer token the
policies read.

## Flavours

- [docker-compose](docker-compose/README.md) - the gateway in front of an OPA
  service with the example policies from `policies/` (allow-all, block-PII and
  a client-authorization policy)
- [kubernetes](kubernetes/README.md) - the same policies through the
  [Inference Gateway Operator](https://github.com/inference-gateway/operator)'s
  `spec.guardrails`, which maps onto the `GUARDRAILS_*` settings
