# Guardrails Example (Docker Compose)

Runs the Inference Gateway with [OPA/Rego](https://www.openpolicyagent.org/) guardrails
enabled. Policies are mounted read-only from `./policies` and compiled at startup, and a
mock external guardrail service is wired in behind `GUARDRAILS_EXTERNAL_URL`.

## How it works

- `GUARDRAILS_ENABLED=true` turns the guardrails middleware on.
- `GUARDRAILS_POLICY_DIR` points at the mounted policy directory. Every `.rego` file in
  it is loaded; each must be in `package guardrails` and expose a `main` rule returning
  one of the four actions below.
- `GUARDRAILS_FAIL_MODE` (`closed` by default) decides what happens if evaluation errors:
  `closed` blocks the request, `open` lets it through.
- `GUARDRAILS_EXTERNAL_URL` / `GUARDRAILS_EXTERNAL_TIMEOUT` point the gateway at an
  external guardrail service consulted alongside the Rego policies (see below).

See [`Configurations.md`](../../../Configurations.md) for all `GUARDRAILS_*` settings.

## Actions

A policy returns a decision like `{"action": "...", "message": "..."}`. Four actions
exist (`internal/guardrails/guardrails.go`):

| Action   | Effect                                                                                               |
| -------- | ---------------------------------------------------------------------------------------------------- |
| `allow`  | continues                                                                                            |
| `block`  | refuses with `403`; on `/mcp` and `/a2a` as a JSON-RPC error envelope with code `-32001`             |
| `redact` | replaces matches of the built-in detectors in the payload with `[REDACTED]`, then continues          |
| `warn`   | continues like `allow`; only visible as a counter in the `inference_gateway_guardrails_total` metric |

`redact` covers the request body on `pre_call` and the response body on `post_call`. The
detectors know API keys, bearer tokens and credit cards (redacted) as well as emails
(detected but not redacted).

## Policies

- `policies/allow_all.rego` - default allow.
- `policies/block_pii.rego` - blocks bodies containing a sample credit-card prefix.
- `policies/authz.rego` - authorization by caller identity (restricts a model to a group).
- `policies/redact_secrets.rego` - redacts secrets from requests and responses instead of
  blocking.

Conditional `main` bodies must stay mutually exclusive (at most one decision per
request): a payload matching two mains with different values is an OPA conflict, and
under `GUARDRAILS_FAIL_MODE=closed` every conflicting evaluation blocks.

### Redaction of secrets and PII

`policies/redact_secrets.rego` returns `{"action": "redact"}` for any payload whose body
holds a secret-shaped value, which makes the middleware run `guardrails.RedactSensitive`
with the built-in detectors instead of refusing:

```rego
main := {"action": "redact"} if {
    input.phase in {"pre_call", "post_call"}
    secrets_in_body
}
```

The trigger mirrors the api-key and bearer-token patterns the detectors replace, so bodies
without secrets keep the default `allow` and the rule stays mutually exclusive with the
other conditional `main` bodies.

A secret typed by the caller is redacted on `pre_call`, so the provider never sees it -
the model echoes back the placeholder:

```bash
curl -X POST http://localhost:8080/v1/chat/completions \
  -H 'Content-Type: application/json' \
  -d '{"model":"openai/gpt-4o","messages":[{"role":"user","content":"Repeat this verbatim: api_key: sk-1234567890abcdef and mail me at alice@example.com"}]}'
```

A secret the provider emits is redacted on `post_call` before the client sees it - note
the request must not use streaming, since `post_call` only covers non-streaming calls:

```bash
curl -X POST http://localhost:8080/v1/chat/completions \
  -H 'Content-Type: application/json' \
  -d '{"model":"openai/gpt-4o","messages":[{"role":"user","content":"Write an example HTTP Authorization header with a bearer token."}]}'
```

The email in the first request survives: the detectors flag it but only redact secrets
and payment cards.

### Authorization by identity

Policies can authorize on the caller's identity via `input.identity`, which holds the
verified OIDC claims (`sub`, `email`, `groups`, `roles`, ...). This requires
`AUTH_ENABLED=true`; with auth off, `input.identity` is undefined and identity-based
rules do not match (default allow).

```rego
main := {"action": "block", "message": "restricted to the ml-eng group"} if {
    input.request.model == "openai/gpt-4o"
    input.identity
    not group_allowed
}

group_allowed if {
    some g in input.identity.groups
    g == "ml-eng"
}
```

Identity is passed on every phase, so the same `input.identity` is available when
guarding tool arguments and outputs.

### Phases

Every policy sees `input.phase`:

| Phase         | When it runs                    | `input.method` | `input.path`         | `input.request.body` |
| ------------- | ------------------------------- | -------------- | -------------------- | -------------------- |
| `pre_call`    | before the request is forwarded | the HTTP verb  | the request path     | the request body     |
| `post_call`   | on the response body            | the HTTP verb  | the request path     | the response body    |
| `tool_args`   | before an MCP tool runs         | `TOOL_CALL`    | `mcp_<alias>_<tool>` | the tool arguments   |
| `tool_output` | after an MCP tool returns       | `TOOL_CALL`    | `mcp_<alias>_<tool>` | the tool output      |

The tool phases apply to both surfaces that run MCP tools: the agent loop behind
`/v1/chat/completions` and a `tools/call` on `POST /mcp`, so one policy covers both.
On `/mcp` a block comes back as a JSON-RPC error envelope with code `-32001`.

```rego
main := {"action": "block", "message": "that tool is off limits"} if {
    input.phase == "tool_args"
    input.path == "mcp_filesystem_write_file"
}
```

`post_call` only runs where the whole response is one body: non-streaming
`/v1/chat/completions` and non-streaming `/a2a` calls. Streaming responses are relayed
as they arrive and are never buffered for a post-call check.

## External guardrail service

`GUARDRAILS_EXTERNAL_URL` wires in an HTTP guardrail service that runs next to the Rego
policies. On every `pre_call` and `post_call` evaluation the gateway POSTs the same
structured input the policies see (`guardrails.Input`) and expects
`{"action": "...", "message": "..."}` back:

```json
{
  "method": "POST",
  "path": "/v1/chat/completions",
  "phase": "pre_call",
  "request": {
    "body": "{\"model\":\"openai/gpt-4o\",\"messages\":[...]}",
    "model": "openai/gpt-4o"
  },
  "identity": { "sub": "user-1", "groups": ["ml-eng"] }
}
```

The contract (`guardrails.ExternalClient.Check`):

- the service receives `method`, `path`, `phase`, `request.body`, `request.model` and
  `identity` (the verified OIDC claims when auth is on)
- only `block` overrides the Rego decision: the call (or response) is refused and the
  service's `message` reaches the client
- any other action, including `redact` and `warn`, is ignored - the Rego decision stands
- an error, a non-JSON response or a timeout (after `GUARDRAILS_EXTERNAL_TIMEOUT`, `5s`
  by default) is treated like any evaluation failure and follows `GUARDRAILS_FAIL_MODE`
- the service is consulted on `pre_call` and `post_call` only; the `tool_args` /
  `tool_output` phases never call it

### The mock service

`external-guardrail/` is a minimal mock built behind that contract. It logs every
decision input it receives - `docker compose logs external-guardrail` shows what the
gateway sends - and allows everything by default. Start it with `BLOCK=1` to watch the
gateway honour an external block, which no policy can override:

```bash
BLOCK=1 docker compose up
curl -X POST http://localhost:8080/v1/chat/completions \
  -H 'Content-Type: application/json' \
  -d '{"model":"openai/gpt-4o","messages":[{"role":"user","content":"hello"}]}'
# {"error":"request blocked by guardrails","message":"refused by the external guardrail service"}
```

Run the Rego policies only by removing the `GUARDRAILS_EXTERNAL_URL` and
`GUARDRAILS_EXTERNAL_TIMEOUT` lines from `docker-compose.yml`.

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
