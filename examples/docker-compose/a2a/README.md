# Agent-to-Agent (A2A) Example

This example runs the Inference Gateway as an A2A server in front of the
[mock agent](https://github.com/inference-gateway/mock-agent). An A2A client
talks to the gateway only; the gateway forwards each JSON-RPC call to the agent
that owns it and relays the answer, so auth, guardrails and telemetry apply to
agent traffic the same way they apply to inference.

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

Read the merged card and the registry:

```bash
curl -s http://localhost:8080/.well-known/agent-card.json | jq .
curl -s http://localhost:8080/a2a/agents | jq .
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

`ListTasks` without a `tenant` fans out to every registered agent and merges the
pages.

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
