# TLS Example

This example demonstrates secure TLS communication with the Inference Gateway using:

- The [Inference Gateway Operator](https://github.com/inference-gateway/operator) and a `Gateway` custom resource
- The Kubernetes Gateway API (Envoy Gateway) terminating TLS at the listener
- cert-manager for automatic certificate issuance

> **Note:** The Helm chart is deprecated. The gateway is now deployed through the operator, and TLS is
> configured via `spec.routing.gateway.tls`.

## Table of Contents

- [TLS Example](#tls-example)
  - [Table of Contents](#table-of-contents)
  - [Architecture](#architecture)
  - [Prerequisites](#prerequisites)
  - [Quick Start](#quick-start)
  - [Configuration](#configuration)
  - [Cleanup](#cleanup)

## Architecture

- **Certificate Management**: cert-manager issues the listener certificate from a self-signed `ClusterIssuer`
  (`cert-manager/clusterissuer.yaml`).
- **Gateway**: An `inference-gateway` `Gateway` custom resource reconciled by the operator.
- **Routing & TLS**: The operator creates a Gateway API `Gateway` whose HTTPS listener terminates TLS using
  the cert-manager-issued secret. cert-manager's Gateway API support is enabled by default (cert-manager
  v1.15+); the Gateway API CRDs are installed before cert-manager so it watches `Gateway` resources.

## Prerequisites

- [Task](https://taskfile.dev/installation/)
- kubectl
- helm
- ctlptl (for cluster management)

## Quick Start

1. Deploy the infrastructure (cluster, Gateway API CRDs, cert-manager, Envoy Gateway and the operator):

   ```bash
   task deploy-infrastructure
   ```

2. Deploy the gateway with TLS:

   ```bash
   task deploy-inference-gateway
   ```

3. Wait for the certificate and Gateway to become ready:

   ```bash
   kubectl get certificate -n inference-gateway -w
   kubectl get gateway.gateway.networking.k8s.io -n inference-gateway
   ```

4. Port-forward the Envoy HTTPS listener and test the secure endpoint:

   ```bash
   ENVOY_SVC=$(kubectl get svc -n envoy-gateway-system \
     -l gateway.envoyproxy.io/owning-gateway-name=inference-gateway \
     -o jsonpath='{.items[0].metadata.name}')
   kubectl -n envoy-gateway-system port-forward "svc/${ENVOY_SVC}" 8443:443 &

   curl -k --resolve api.inference-gateway.local:8443:127.0.0.1 \
     https://api.inference-gateway.local:8443/v1/models
   ```

   `-k` accepts the self-signed certificate; `--resolve` points the hostname at the port-forward.

## Configuration

- **Certificate / issuer**: edit `cert-manager/clusterissuer.yaml` (a Let's Encrypt issuer is included,
  commented, for production).
- **Gateway TLS**: configured in `gateway.yaml` under `spec.routing.gateway.tls` (`issuer` selects the
  cert-manager `ClusterIssuer`; `secretName` is where the issued certificate is stored).

## Cleanup

```bash
task clean
```
