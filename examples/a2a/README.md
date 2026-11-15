# A2A

The gateway as an Agent-to-Agent server: `A2A_ENABLED=true` gives the gateway one
merged agent card and one `/a2a` endpoint, and every JSON-RPC call is relayed to
the agent it names. The example spins up mock agents so the relay, task-id
prefixing and card merge are visible end to end. The gateway stays a stateless
proxy - it stores no tasks, messages or events.

## Flavours

- [docker-compose](docker-compose/README.md) - the gateway relaying to mock
  agent services
- [kubernetes](kubernetes/README.md) - the same relay through the
  [Inference Gateway Operator](https://github.com/inference-gateway/operator)
  and a `Gateway` custom resource, with `agents.yaml` defining the agents
