# Model Context Protocol (MCP) Integration Example

This example demonstrates the Inference Gateway's MCP integration on Kubernetes using the
[Inference Gateway Operator](https://github.com/inference-gateway/operator). The gateway connects to five
MCP servers (time, search, filesystem, pizza, calculator) and exposes their tools through the OpenAI-compatible
API, and as an MCP server of its own on `POST /mcp`.

> **⚠️ Important Notice**: The MCP servers included in this example
> are simplified implementations designed for demonstration and testing purposes only. They should **NOT**
> be used in production environments without proper security hardening, input validation, authentication,
> authorization, and error handling.
>
> **Note:** The gateway is deployed through the operator. MCP is configured
> under `spec.mcp` in `gateway.yaml`.

## Table of Contents

- [Model Context Protocol (MCP) Integration Example](#model-context-protocol-mcp-integration-example)
  - [Table of Contents](#table-of-contents)
  - [Architecture](#architecture)
  - [Prerequisites](#prerequisites)
  - [Quick Start](#quick-start)
  - [Testing](#testing)
  - [MCP Inspector](#mcp-inspector)
  - [Configuration](#configuration)
  - [Cleanup](#cleanup)

## Architecture

- **Gateway**: An `inference-gateway` `Gateway` custom resource with MCP enabled (`spec.mcp.enabled: true`,
  `spec.mcp.expose: true`). `expose` also turns the gateway into an MCP server on `POST /mcp`, aggregating
  every backend server.
- **MCP servers**: `MCP` custom resources in [`mcp-servers.yaml`](mcp-servers.yaml). The operator runs each
  image as a Deployment behind a `<name>-service` Service, and the gateway finds them through **service
  discovery** (`spec.mcp.serviceDiscovery`), which picks up every `MCP` resource in its namespace - add a
  label `selector` to narrow it. Adding a server is one more `MCP` resource; `gateway.yaml` stays as is. The
  gateway speaks MCP `2026-07-28` - stateless, no `initialize`, no session - to all of them.
  - `time` (`:8081`), `search` (`:8082`) and `filesystem` (`:8083`): minimal Go servers built from the
    `*-server/` directories.
  - `pizza` (`:8084`): the official TypeScript SDK v2 (`createMcpHandler`).
  - `calculator` (`:8085`): the official Go SDK with a stateless Streamable HTTP handler.

  The images are built locally and imported into k3d with a `:dev` tag. The pizza and calculator images are
  built from [`../../docker-compose/mcp`](../../docker-compose/mcp/), which shares their sources with the
  Docker Compose example.

- **Tool names**: discovered servers are addressed by their cluster FQDN, and the operator does not pass an
  alias, so the gateway derives one from the host: tools are named
  `mcp_time-service_inference-gateway_svc_cluster_local_time`. The operator's default `selector` tool mode
  only shows the model two meta-tools, so the length is harmless there; `direct` mode would exceed the
  64-character function-name limit of OpenAI-compatible providers.
- **MCP Inspector**: A web UI (`mcp-inspector`) connected to the gateway's own `/mcp` endpoint, so it sees
  every server's tools at once.
- **Routing**: HTTP traffic via the Kubernetes Gateway API (Envoy Gateway `v1.9`, which installs Gateway API
  `v1.6`), configured under `spec.gatewayAPI`. The example pins operator `v0.25.1` and k3s `v1.37`.

## Prerequisites

- [Task](https://taskfile.dev/installation/)
- kubectl, helm, docker, ctlptl
- curl and jq (for the tests)

## Quick Start

Deploy everything (cluster, Gateway API, Envoy Gateway, operator, MCP servers and the gateway):

```bash
task deploy
```

Add your provider API keys to the `inference-gateway-secrets` Secret in `gateway.yaml` (Groq is recommended
for tool-calling), then re-apply and restart:

```bash
kubectl apply -f gateway.yaml
task restart
```

## Testing

Run the in-cluster integration tests (gateway health, MCP tools discovery, and `POST /mcp`):

```bash
task test
```

To call the gateway from your machine, port-forward the Envoy data plane and query the MCP tools endpoint:

```bash
task port-forward-gateway
```

```bash
curl -s http://localhost:8080/v1/mcp/tools -H "Host: api.inference-gateway.local" | jq '.data[] | .name'
```

Or use the gateway as an MCP server. Every request carries MCP `2026-07-28` in `params._meta`, mirrored
into headers:

```bash
curl -s -X POST http://localhost:8080/mcp \
  -H "Host: api.inference-gateway.local" \
  -H "Content-Type: application/json" \
  -H "MCP-Protocol-Version: 2026-07-28" \
  -H "Mcp-Method: tools/list" \
  -d '{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{"_meta":{"io.modelcontextprotocol/protocolVersion":"2026-07-28","io.modelcontextprotocol/clientInfo":{"name":"curl","version":"1.0"},"io.modelcontextprotocol/clientCapabilities":{}}}}' \
  | jq '.result.tools[] | .name'
```

## MCP Inspector

Forward the Inspector UI (`6274`) and its sandbox for MCP Apps (`6275`):

```bash
task port-forward
```

In another terminal, print the Inspector URL - it carries the session token - and open it in a browser:

```bash
task inspector-url
```

The Inspector connects to `http://inference-gateway:8080/mcp` in the modern protocol era and lists every
`mcp_<alias>_<tool>`.

## Configuration

- **MCP servers**: add or edit `MCP` resources in `mcp-servers.yaml` (`spec.image`, `spec.server.port`,
  `spec.server.path`, and `spec.server.command` - the operator runs `/mcp-server` otherwise). Service
  discovery picks them up without touching `gateway.yaml`. A server must speak MCP `2026-07-28`; see
  [Requirements for Your MCP Server](../../docker-compose/mcp/README.md#requirements-for-your-mcp-server).
- **MCP client timeouts**: configured under `spec.mcp.timeouts` in `gateway.yaml`.
- **Providers**: API keys are read from the `inference-gateway-secrets` Secret.

## Cleanup

```bash
task clean
```
