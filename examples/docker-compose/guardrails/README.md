# Guardrails Example (Docker Compose)

Runs the Inference Gateway with [OPA/Rego](https://www.openpolicyagent.org/) guardrails
enabled. Policies are mounted read-only from `./policies` and compiled at startup.

## How it works

- `GUARDRAILS_ENABLED=true` turns the guardrails middleware on.
- `GUARDRAILS_POLICY_DIR` points at the mounted policy directory. Every `.rego` file in
  it is loaded; each must be in `package guardrails` and expose a `main` rule returning
  `{"action": "allow"}` or `{"action": "block", "message": "..."}`.
- `GUARDRAILS_FAIL_MODE` (`closed` by default) decides what happens if evaluation errors:
  `closed` blocks the request, `open` lets it through.

See [`Configurations.md`](../../../Configurations.md) for all `GUARDRAILS_*` settings.

## Policies

- `policies/allow_all.rego` - default allow.
- `policies/block_pii.rego` - blocks bodies containing a sample credit-card prefix.

## Quick Start

```bash
cp .env.example .env
# set at least one provider API key in .env, e.g. OPENAI_API_KEY=...
docker compose up
```

Blocked request (contains the sample `4111` prefix):

```bash
curl -X POST http://localhost:8080/v1/chat/completions \
  -H 'Content-Type: application/json' \
  -d '{"model":"openai/gpt-4o","messages":[{"role":"user","content":"card 4111 1111 1111 1111"}]}'
```

Edit or add `.rego` files under `policies/` and restart to change enforcement.
