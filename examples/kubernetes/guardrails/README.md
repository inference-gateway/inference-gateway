# Guardrails Example (Kubernetes)

Deploys the Inference Gateway with [OPA/Rego](https://www.openpolicyagent.org/) guardrails
enabled, via the [Inference Gateway Operator](https://github.com/inference-gateway/operator)
and the Kubernetes Gateway API.

Rego policies live in a `ConfigMap` and are mounted into the gateway at
`spec.guardrails.policyDir`. Each `.rego` file must be in `package guardrails` and expose a
`main` rule returning `{"action": "allow"}` or `{"action": "block", "message": "..."}`.

> **Note:** `spec.guardrails` requires an operator build that supports the guardrails field.
> The gateway itself is configured through the `GUARDRAILS_*` env vars documented in
> [`Configurations.md`](../../../Configurations.md); the operator maps `spec.guardrails` onto
> them and mounts `policiesConfigMap` at `policyDir`.

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
- `spec.guardrails.failMode` - `closed` blocks on evaluation errors, `open` lets requests through.
- `spec.guardrails.policyDir` - where policies are mounted inside the container.
- `spec.guardrails.policiesConfigMap` - the ConfigMap whose `.rego` keys are mounted there.

Edit the ConfigMap in `gateway.yaml` to change enforcement, then re-apply.

## Cleanup

```bash
task clean
```
