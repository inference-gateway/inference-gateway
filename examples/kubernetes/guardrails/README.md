# Guardrails Example (Kubernetes)

Deploys the Inference Gateway with [OPA/Rego](https://www.openpolicyagent.org/) guardrails
enabled, via the [Inference Gateway Operator](https://github.com/inference-gateway/operator)
and the Kubernetes Gateway API.

Rego policies live in a `ConfigMap` referenced by `spec.guardrails.configMapRef` and are
mounted into the gateway's policy directory. Each `.rego` file must be in `package guardrails` and expose a
`main` rule returning `{"action": "allow"}` or `{"action": "block", "message": "..."}`.

> **Note:** `spec.guardrails` requires an operator build that supports the guardrails field.
> The gateway itself is configured through the `GUARDRAILS_*` env vars documented in
> [`Configurations.md`](../../../Configurations.md); the operator maps `spec.guardrails` onto
> them and mounts `configMapRef` at `GUARDRAILS_POLICY_DIR`.

## Prerequisites

- [Task](https://taskfile.dev/installation/)
- kubectl
- helm
- ctlptl (for cluster management)

## Quick Start

1. Deploy the infrastructure (cluster, Gateway API CRDs, Envoy Gateway and the operator):

   ```bash
   task deploy-infrastructure
   ```

2. Set your provider API key in `gateway.yaml` (the `inference-gateway-secrets` Secret),
   then deploy the gateway:

   ```bash
   task deploy-inference-gateway
   ```

## Configuration

- `spec.guardrails.enabled` - turns the guardrails middleware on.
- `spec.guardrails.failMode` - `deny` blocks on evaluation errors, `allow` lets requests through.
- `spec.guardrails.configMapRef.name` - the ConfigMap whose `.rego` keys are mounted as policies.

Edit the ConfigMap in `gateway.yaml` to change enforcement, then re-apply.

## Cleanup

```bash
task clean
```
