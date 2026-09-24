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
- `policies/authz.rego` - authorization by caller identity (restricts a model to a group).

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
