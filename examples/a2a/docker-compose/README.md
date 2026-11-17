# Agent-to-Agent (A2A) Example

This example runs the Inference Gateway as an A2A server in front of two
instances of the [mock agent](https://github.com/inference-gateway/mock-agent),
registered as `mock` and `mock-b`. An A2A client talks to the gateway only; the
gateway forwards each JSON-RPC call to the agent that owns it and relays the
answer, so auth, guardrails and telemetry apply to agent traffic the same way
they apply to inference.

## Quick Start

```bash
cp .env.example .env
docker compose up
```

No API key is needed: the mock agent uses a mock LLM.

## What the gateway serves

| Endpoint                                        | Description                                                           |
| ----------------------------------------------- | --------------------------------------------------------------------- |
| `GET /.well-known/agent-card.json`              | The gateway's own card: every agent's skills, ids prefixed `<alias>_` |
| `POST /a2a`                                     | JSON-RPC 2.0 relay for the eleven A2A methods                         |
| `GET /a2a/agents`                               | The registry: alias, url, card, reachable, lastSeen                   |
| `GET /.well-known/oauth-protected-resource/a2a` | RFC 9728 metadata, served when `AUTH_ENABLED=true`                    |

## Usage

Read the merged card and the registry. The card carries both agents' skills,
`mock_*` and `mock-b_*`, plus one interface per agent with its alias as the
tenant; the registry lists both:

```bash
curl -s http://localhost:8080/.well-known/agent-card.json | jq '{skills: [.skills[].id], interfaces: .supportedInterfaces}'
curl -s http://localhost:8080/a2a/agents | jq '.agents[] | {alias, url, reachable}'
```

Send a message. The agent is named with `params.tenant` (the alias, which the
card also advertises as the tenant of the agent's interface) or with
`message.metadata.agent`:

```bash
curl -s -X POST http://localhost:8080/a2a \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc": "2.0",
    "id": 1,
    "method": "SendMessage",
    "params": {
      "tenant": "mock",
      "message": {
        "messageId": "m-1",
        "role": "ROLE_USER",
        "parts": [{"text": "echo hello"}]
      }
    }
  }' | jq .
```

The task id comes back as `mock:<task id>`. Poll it the way you would poll the
agent directly; the prefix is all the gateway needs to route the call:

```bash
curl -s -X POST http://localhost:8080/a2a \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","id":2,"method":"GetTask","params":{"id":"mock:<task id>"}}' | jq .
```

Subscribe to a task's events. The gateway pipes the agent's SSE stream through
until the agent closes it or `A2A_STREAM_IDLE_TIMEOUT` passes without an event:

```bash
curl -N -X POST http://localhost:8080/a2a \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","id":3,"method":"SubscribeToTask","params":{"id":"mock:<task id>"}}'
```

Skip the poll: `SendStreamingMessage` answers in one RPC, naming the agent the
way `SendMessage` does, and streams the reply over the same pipe. The task id
of a message sent this way comes back in the stream as `mock:<task id>`
already:

```bash
curl -N -X POST http://localhost:8080/a2a \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","id":4,"method":"SendStreamingMessage","params":{"tenant":"mock","message":{"messageId":"m-3","role":"ROLE_USER","parts":[{"text":"echo hello"}]}}}'
```

The two streaming methods are the exception to `post_call`: guardrails still
see their request on `pre_call`, but the SSE stream is relayed as it arrives
and is never buffered for a response-side policy.

## Two agents

With more than one agent registered the gateway never guesses. A call that
names no agent is rejected with `-32602` listing the aliases:

```bash
curl -s -X POST http://localhost:8080/a2a \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","id":4,"method":"SendMessage","params":{"message":{"messageId":"m-2","role":"ROLE_USER","parts":[{"text":"echo hello"}]}}}' | jq .error
```

A call whose hints disagree, e.g. `tenant: "mock-b"` with a `mock:` task id,
is rejected with `-32602` too. Send the same message with `"tenant": "mock-b"` and the task id comes back
as `mock-b:<task id>`; follow-up calls route on that prefix alone.

`ListTasks` is the one method that may name no agent: it fans out to every
registered agent and merges the pages, so both agents' tasks come back with
their own prefixes:

```bash
curl -s -X POST http://localhost:8080/a2a \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","id":5,"method":"ListTasks","params":{}}' | jq '[.result.tasks[].id]'
```

Stop one agent and the other keeps serving. `ListTasks` skips the agent that
fails and still returns `mock`'s tasks, a call to `mock-b` comes back as
`-32603`, and the registry marks it unreachable on the next card refresh
(`A2A_CARD_REFRESH_INTERVAL`):

```bash
docker compose stop mock-agent-b
curl -s -X POST http://localhost:8080/a2a \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","id":6,"method":"ListTasks","params":{}}' | jq '[.result.tasks[].id]'
docker compose start mock-agent-b
```

## Auth (opt-in)

`AUTH_ENABLED=true` puts `POST /a2a` behind the OIDC middleware. The demo realm
from the [keycloak example](../auth/keycloak/docker-compose/README.md) is
reused through [docker-compose.auth.yml](docker-compose.auth.yml), which adds
Keycloak to the stack and switches the gateway's auth on - the realm file is
not duplicated:

```bash
docker compose -f docker-compose.yml -f docker-compose.auth.yml up -d
```

A message without a token is rejected with `401` and a `WWW-Authenticate`
challenge whose `resource_metadata` points at the RFC 9728 document:

```bash
curl -i -X POST http://localhost:8080/a2a \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","id":7,"method":"SendMessage","params":{"tenant":"mock","message":{"messageId":"m-a","role":"ROLE_USER","parts":[{"text":"echo hello"}]}}}'
# HTTP/1.1 401 Unauthorized
# WWW-Authenticate: Bearer realm="inference-gateway", resource_metadata="http://localhost:8080/.well-known/oauth-protected-resource/a2a"
```

The metadata document itself takes no token - a client without one fetches it
to learn where to run its grant:

```bash
curl http://localhost:8080/.well-known/oauth-protected-resource/a2a
# {"resource":"http://localhost:8080/a2a",
#  "authorization_servers":["http://keycloak:8080/realms/inference-gateway-realm"],
#  "bearer_methods_supported":["header"]}
```

The agent card stays open: `GET /.well-known/agent-card.json` deliberately
answers without a token, so a client can still discover the agents' skills and
tenants first. Everything else wants the bearer, `POST /a2a` and
`GET /a2a/agents` included. Request one from the realm's service account:

```bash
TOKEN="$(curl -s http://localhost:8081/realms/inference-gateway-realm/protocol/openid-connect/token \
  -d grant_type=client_credentials \
  -d client_id=inference-gateway-client \
  -d client_secret=very-secret | jq -r .access_token)"

curl -s -X POST http://localhost:8080/a2a \
  -H "Authorization: Bearer ${TOKEN}" \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc": "2.0",
    "id": 8,
    "method": "SendMessage",
    "params": {
      "tenant": "mock",
      "message": {
        "messageId": "m-4",
        "role": "ROLE_USER",
        "parts": [{"text": "echo hello"}]
      }
    }
  }' | jq .
```

`resource` defaults to the request scheme (honouring `X-Forwarded-Proto`) and
`Host`; behind an ingress that rewrites either, set `A2A_RESOURCE_URL` to the
canonical public `/a2a` URL - it is published on the agent card, in the
challenge and in the metadata document, and if the IdP stamps that resource
into `aud` ([RFC 8707](https://datatracker.ietf.org/doc/html/rfc8707)), list
the same value in `AUTH_OIDC_AUDIENCE`. The verified token's claims also reach
guardrails as `input.identity`. What a given `401` means is in the [auth
example's troubleshooting](../auth/keycloak/docker-compose/README.md#troubleshooting).

## Guardrails (opt-in)

`GUARDRAILS_ENABLED=true` compiles the Rego policies mounted into
`GUARDRAILS_POLICY_DIR` (`./policies` in the compose file) and runs their
`pre_call` on every relayed call, streaming or not. The shipped
[block_mock_b.rego](policies/block_mock_b.rego) refuses everything the call
addresses to the `mock-b` agent, a decision read from the raw JSON-RPC body
that arrives on `input.path == "/a2a"` as `input.request.body`:

```bash
GUARDRAILS_ENABLED=true docker compose up -d

curl -i -X POST http://localhost:8080/a2a \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","id":20,"method":"SendMessage","params":{"tenant":"mock-b","message":{"messageId":"m-5","role":"ROLE_USER","parts":[{"text":"echo hello"}]}}}'
# HTTP/1.1 403 Forbidden
# {"jsonrpc":"2.0","id":20,"error":{"code":-32001,"message":"agent mock-b is off limits on this gateway"}}
```

Calls that name `mock` still relay, and non-streaming ones additionally pass
`post_call` with the JSON-RPC result as `input.request.body` - only the two
streaming methods skip it. The four actions, the `tool_args` / `tool_output`
phases and the external guardrail service are the
[guardrails example's](../../guardrails/docker-compose/README.md) subject; on
this route a block always answers with `HTTP 403` and code `-32001`.

## Telemetry (opt-in)

Every relayed call is counted and timed in two Prometheus series when
`TELEMETRY_ENABLED=true` (the compose file already maps the metrics port):

```bash
TELEMETRY_ENABLED=true docker compose up -d
```

Send the calls from the sections above, then read the two series off the
Prometheus endpoint:

```bash
curl -s http://localhost:9464/metrics | grep '^a2a_'
```

- `a2a_requests_total` - one increment per relayed call, labelled `alias`,
  `method` and `status` (`ok`, or the JSON-RPC error code the client got, e.g.
  `-32602` for a call that names no agent)
- `a2a_request_duration_seconds` - the duration histogram of the same calls,
  labelled `alias` and `method`; for a streaming call it measures the time to
  open the stream

A guardrails block never appears here: the middleware refuses the call before
the relay runs, so the series only cover calls that reached an agent. Prometheus
and Grafana wiring is the
[monitoring example's](../../monitoring/docker-compose/README.md) subject.

## Debugging with a2a-debugger

The [A2A Debugger](https://github.com/inference-gateway/a2a-debugger) reads the
gateway card like any other agent's:

```bash
docker run --rm -it --network host ghcr.io/inference-gateway/a2a-debugger:latest \
  --server-url http://localhost:8080 tasks list
```

## Adding your own agents

Add `alias=url` entries to `A2A_AGENTS`, comma separated. Without `alias=` the
alias is derived from the URL host. Per-agent credentials go into the URL as
basic auth (`alias=https://user:pass@agent.example.com`); the caller's bearer
token is never forwarded to agents.
