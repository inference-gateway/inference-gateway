# Agent Example

This example demonstrates an agent-based deployment pattern with the Inference Gateway using:

- A custom logs-analyzer agent with cluster-wide read access
- The [Inference Gateway Operator](https://github.com/inference-gateway/operator) and a `Gateway` custom resource
- A failing test deployment for the agent to analyze

> **Note:** The gateway is deployed through the operator. The logs-analyzer is
> a bespoke workload (it needs a cluster-wide RBAC role to read pod logs), so it remains a plain Deployment
> rather than an operator `Agent` resource.

## Table of Contents

- [Agent Example](#agent-example)
  - [Table of Contents](#table-of-contents)
  - [Architecture](#architecture)
  - [Prerequisites](#prerequisites)
  - [Quick Start](#quick-start)
  - [Configuration](#configuration)
  - [Cleanup](#cleanup)

## Architecture

- **Gateway**: An `inference-gateway` `Gateway` custom resource reconciled by the operator. It is reached
  in-cluster at `http://inference-gateway.inference-gateway:8080`, so no external routing is configured.
- **Agent**: A custom logs-analyzer that lists pods cluster-wide, detects errors in their logs, and asks
  the gateway (Groq) to analyze them. It has its own ServiceAccount and ClusterRole.
- **Test Deployment**: A deliberately failing deployment for the agent to find.

> The operator only needs the Gateway API CRDs installed so its controller can start; this example does not
> deploy Envoy Gateway because the gateway is not exposed externally.

## Prerequisites

- [Task](https://taskfile.dev/installation/)
- kubectl
- helm
- ctlptl (for cluster management)

## Quick Start

1. Deploy the cluster, registry, Gateway API CRDs and the operator:

   ```bash
   task deploy-infrastructure
   ```

2. Add your Groq API key to the `inference-gateway-secrets` Secret in `gateway.yaml`, then deploy the gateway:

   ```bash
   task deploy-inference-gateway
   ```

3. Build and push the logs-analyzer image (the cluster registry is ready after step 1):

   ```bash
   task build-logs-analyzer-agent
   ```

4. Deploy the logs analyzer and the failing test deployment:

   ```bash
   task deploy-logs-analyzer-agent
   ```

5. Watch the agent analyze failing pods:

   ```bash
   kubectl logs -f deployment/logs-analyzer -n logs-analyzer
   ```

## Configuration

- **Agent**: edit the YAMLs in `logs-analyzer/` (Deployment, ServiceAccount, ClusterRole/Binding). Rebuild
  and redeploy with `task build-logs-analyzer-agent && task deploy-logs-analyzer-agent`.
- **Test deployment**: edit the YAMLs in `failing-deployment/` to simulate different failures.
- **Provider**: the Groq API key is read from the `inference-gateway-secrets` Secret in `gateway.yaml`.

## Cleanup

```bash
task clean
```
