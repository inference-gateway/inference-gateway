# Examples using Kubernetes

This directory contains examples that demonstrate how to deploy the Inference Gateway on Kubernetes with the
[Inference Gateway Operator](https://github.com/inference-gateway/operator).

> **Note:** The Inference Gateway Helm chart is deprecated. Each example now installs the operator and applies
> a `Gateway` custom resource (`gateway.yaml`). North-south traffic is served via the Kubernetes Gateway API
> (Envoy Gateway) using `spec.routing`, rather than ingress-nginx.

- [Basic](basic/README.md)
- [Hybrid Environment](hybrid/README.md)
- [TLS](tls/README.md)
- [Authentication](authentication/README.md)
- [Agent Building](agent/README.md)
- [Monitoring](monitoring/README.md)
- [Model Context Protocol (MCP)](mcp/README.md)

Every example shares the same shape:

1. `task deploy-infrastructure` — provision a local k3d cluster and install the Gateway API CRDs, Envoy
   Gateway and the operator (plus any example-specific dependencies such as cert-manager, Prometheus or
   Keycloak).
2. `task deploy-inference-gateway` — apply `gateway.yaml`.
3. Reach the gateway by port-forwarding the Envoy data plane (each README shows the exact command), so no
   `/etc/hosts` changes are required:

   ```bash
   ENVOY_SVC=$(kubectl get svc -n envoy-gateway-system \
     -l gateway.envoyproxy.io/owning-gateway-name=inference-gateway \
     -o jsonpath='{.items[0].metadata.name}')
   kubectl -n envoy-gateway-system port-forward "svc/${ENVOY_SVC}" 8080:80
   curl -H 'Host: api.inference-gateway.local' http://localhost:8080/v1/models
   ```

See each example's README for the full, example-specific walkthrough.
