# Agent-to-Agent (A2A) on Kubernetes

This example runs the Inference Gateway as an A2A server in front of the
[mock agent](https://github.com/inference-gateway/mock-agent) in a local k3d
cluster. An A2A client talks to the gateway only; the gateway publishes one
merged agent card and forwards each JSON-RPC call to the agent that owns it, so
auth, guardrails and telemetry apply to agent traffic the same way they apply to
inference.

The gateway never talks to the Kubernetes API: the operator discovers the
`Agent` custom resources and renders `A2A_AGENTS` for it. That needs operator
`v0.27.0` or newer, which is where `Gateway.spec.a2a` landed.

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

`task deploy` provisions the cluster, installs the Gateway API CRDs and the
operator, then applies `gateway.yaml` and `agents.yaml`. The `Gateway` turns A2A
on and selects agents by label:

```yaml
spec:
  a2a:
    enabled: true
    serviceDiscovery:
      enabled: true
      selector:
        matchLabels:
          a2a: demo
```

The operator lists the matching `Agent` CRs and renders one `A2A_AGENTS` entry
per agent, `<metadata.name>=<url>`, onto the gateway Deployment - visible on the
`Gateway` status:

```console
$ kubectl -n inference-gateway get gateway inference-gateway -o jsonpath='{.status.a2aAgents}'
["mock-agent=http://mock-agent.inference-gateway.svc.cluster.local:8080"]
```

Creating or deleting a matching `Agent` re-renders that variable and rolls the
gateway, so the alias set always matches the cluster.

`task test` checks the rendered status, the merged card, the registry and a
relayed `SendMessage`, all from inside the cluster. `gateway.yaml` pins the
gateway to `:latest`, so A2A is served from the first release that ships it
onward.

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
      "tenant": "mock-agent",
      "message": {
        "messageId": "m-1",
        "role": "ROLE_USER",
        "parts": [{"text": "echo hello"}]
      }
    }
  }' | jq .
```

The task id comes back as `mock-agent:<task id>`; the prefix is all the gateway
needs to route a follow-up call:

```bash
curl -s -X POST http://localhost:8080/a2a \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","id":2,"method":"GetTask","params":{"id":"mock-agent:<task id>"}}' | jq .
```

## Adding your own agents

Copy the `Agent` in `agents.yaml`, give it your image and keep the `a2a: demo`
label, then `task deploy-agents`. The CR name becomes the alias, so it must
match `^[a-z0-9_-]+$`.

Agents that live outside the cluster are static entries on the `Gateway`
instead, unioned with the discovered ones:

```yaml
spec:
  a2a:
    agents:
      - name: remote
        url: https://user:pass@agent.example.com
```

Per-agent credentials go into the URL as basic auth; the caller's bearer token
is never forwarded to agents.

## Tear down

```bash
task clean
```
