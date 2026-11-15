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
