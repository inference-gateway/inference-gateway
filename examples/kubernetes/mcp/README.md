# Model Context Protocol (MCP) Integration Example

This example demonstrates the Inference Gateway's MCP integration on Kubernetes using the
[Inference Gateway Operator](https://github.com/inference-gateway/operator). The gateway connects to three
MCP servers (time, search, filesystem) and exposes their tools through the OpenAI-compatible API.

> **⚠️ Important Notice**: The MCP servers included in this example (time, search, and filesystem servers)
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
  `spec.mcp.expose: true`).
- **MCP servers**: Three minimal Go servers — `time-server` (`:8081`), `search-server` (`:8082`) and
  `filesystem-server` (`:8083`) — deployed as plain Deployments/Services and built locally into the k3d
  cluster. They are wired into the gateway as **static MCP servers** (`spec.mcp.servers`), the operator's
  documented pattern for externally-hosted MCPs.
- **MCP Inspector**: A web UI (`mcp-inspector`) for exploring the MCP servers.
- **Routing**: HTTP traffic via the Kubernetes Gateway API (Envoy Gateway).

> The operator can also manage MCP servers natively via the `MCP` custom resource plus label-based service
> discovery (`spec.mcp.serviceDiscovery`). This example uses static servers because the sample servers are
> bespoke; see the operator docs for the `MCP` CR approach.

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

Run the in-cluster integration tests (gateway health and MCP tools discovery):

```bash
task test
```

To call the gateway from your machine, port-forward the Envoy data plane and query the MCP tools endpoint:

```bash
task port-forward-gateway
```

```bash
curl -s http://localhost:8080/v1/mcp/tools -H "Host: api.inference-gateway.local" | jq '.tools[] | .name'
```

## MCP Inspector

Forward the inspector UI and open it in a browser:

```bash
task port-forward
```

Then open <http://localhost:6274>.

## Configuration

- **MCP servers**: edit the manifests in `time-server/`, `search-server/` and `filesystem-server/`, and the
  matching `spec.mcp.servers` entries in `gateway.yaml`.
- **MCP client timeouts**: configured under `spec.mcp.timeouts` in `gateway.yaml`.
- **Providers**: API keys are read from the `inference-gateway-secrets` Secret.

## Cleanup

```bash
task clean
```
