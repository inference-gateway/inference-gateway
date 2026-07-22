# Basic Deployment Example

This example demonstrates the simplest deployment of the Inference Gateway using the
[Inference Gateway Operator](https://github.com/inference-gateway/operator) and the Kubernetes Gateway API.

> **Note:** All Kubernetes examples now deploy the gateway through the operator
> by applying a `Gateway` custom resource.

## Table of Contents

- [Basic Deployment Example](#basic-deployment-example)
  - [Table of Contents](#table-of-contents)
  - [Architecture](#architecture)
  - [Prerequisites](#prerequisites)
  - [Quick Start](#quick-start)
  - [Configuration](#configuration)
  - [Cleanup](#cleanup)

## Architecture

- **Operator**: The Inference Gateway Operator watches `Gateway` custom resources and reconciles the
  underlying Deployment, Service and autoscaling.
- **Gateway**: An `inference-gateway` `Gateway` resource (`core.inference-gateway.com/v1alpha1`).
- **Routing**: North-south traffic is served via the Kubernetes Gateway API (`spec.routing`), implemented by
  Envoy Gateway (the `envoy` GatewayClass).

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

2. Add your provider API keys to the `inference-gateway-secrets` Secret in `gateway.yaml`, then deploy the
   gateway:

   ```bash
   task deploy-inference-gateway
   ```

3. Wait for the gateway and its Gateway API resources to become ready:

   ```bash
   kubectl get gateway.core.inference-gateway.com -n inference-gateway -w
   kubectl get gateway.gateway.networking.k8s.io -n inference-gateway
   kubectl get httproute -n inference-gateway
   ```

4. Port-forward the Envoy data plane for the gateway and send a test request:

   ```bash
   ENVOY_SVC=$(kubectl get svc -n envoy-gateway-system \
     -l gateway.envoyproxy.io/owning-gateway-name=inference-gateway \
     -o jsonpath='{.items[0].metadata.name}')
   kubectl -n envoy-gateway-system port-forward "svc/${ENVOY_SVC}" 8080:80 &

   curl -H 'Host: api.inference-gateway.local' http://localhost:8080/v1/models
   ```

## Configuration

The gateway is configured declaratively in `gateway.yaml` via the `Gateway` spec — providers, telemetry,
autoscaling (HPA), resources and routing. Provider API keys are read from the `inference-gateway-secrets`
Secret. See the [operator documentation](https://github.com/inference-gateway/operator) for the full
`Gateway` API reference.

## Cleanup

```bash
task clean
```
