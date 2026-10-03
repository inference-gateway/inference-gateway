# Agent-to-Agent (A2A) on Kubernetes

This example runs the Inference Gateway as an A2A server in front of the
[mock agent](https://github.com/inference-gateway/mock-agent) in a local k3d
cluster. An A2A client talks to the gateway only; the gateway publishes one
merged agent card and forwards each JSON-RPC call to the agent that owns it, so
auth, guardrails and telemetry apply to agent traffic the same way they apply to
inference.

> **Note:** unlike the other Kubernetes examples, this one applies plain
> manifests instead of a `Gateway` custom resource. The operator has no
> `spec.a2a` yet
> ([operator#268](https://github.com/inference-gateway/operator/issues/268)), so
> the `A2A_*` variables are set on a `Deployment` directly (`a2a.yaml`).

## Prerequisites

- [ctlptl](https://github.com/tilt-dev/ctlptl) and [k3d](https://k3d.io)
- [kubectl](https://kubernetes.io/docs/tasks/tools/)
- [Task](https://taskfile.dev)

No API key is needed: the mock agent uses a mock LLM.

## Quick Start

```bash
task deploy
task test
```

`task deploy` provisions the cluster and applies `a2a.yaml`: the mock agent, and
the gateway with

```yaml
A2A_ENABLED: 'true'
A2A_AGENTS: 'mock=http://mock-agent:8080'
```

`task test` checks the merged card, the registry and a relayed `SendMessage`,
all from inside the cluster. The manifest pins the gateway to `:latest`, so A2A
is served from the first release that ships it onward.

## What the gateway serves

| Endpoint                                        | Description                                                           |
| ----------------------------------------------- | --------------------------------------------------------------------- |
| `GET /.well-known/agent-card.json`              | The gateway's own card: every agent's skills, ids prefixed `<alias>_` |
| `POST /a2a`                                     | JSON-RPC 2.0 relay for the eleven A2A methods                         |
| `GET /a2a/agents`                               | The registry: alias, url, card, reachable, lastSeen                   |
| `GET /.well-known/oauth-protected-resource/a2a` | RFC 9728 metadata, served when `AUTH_ENABLED=true`                    |

## Usage

Reach the gateway from your machine:

```bash
task port-forward
```

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

The task id comes back as `mock:<task id>`; the prefix is all the gateway needs
to route a follow-up call:

```bash
curl -s -X POST http://localhost:8080/a2a \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","id":2,"method":"GetTask","params":{"id":"mock:<task id>"}}' | jq .
```

## Adding your own agents

Deploy the agent in the cluster and add an `alias=url` entry to `A2A_AGENTS` in
`a2a.yaml`, comma separated, then `task deploy-a2a`. Without `alias=` the alias
is derived from the URL host. Per-agent credentials go into the URL as basic
auth (`alias=https://user:pass@agent.example.com`); the caller's bearer token is
never forwarded to agents.

## Tear down

```bash
task clean
```
