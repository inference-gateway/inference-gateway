# Examples

Examples are grouped by domain first: the scenario is the top level and the
deployment flavour (`docker-compose/` or `kubernetes/`) sits inside it. Each
domain has one README that describes the scenario once and links its flavours;
each flavour folder carries the full walkthrough.

| Domain                            | What it shows                                            | Flavours                                                                                                                                              |
| --------------------------------- | -------------------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------- |
| [Basic](basic/)                   | The gateway with a single model provider                 | [compose](basic/docker-compose/README.md), [kubernetes](basic/kubernetes/README.md)                                                                   |
| [Authentication](auth/)           | OIDC against Keycloak, Entra ID, Google Cloud or Cognito | per identity provider, listed in [auth](auth/README.md)                                                                                               |
| [Guardrails](guardrails/)         | OPA/Rego policies enforced on requests                   | [compose](guardrails/docker-compose/README.md), [kubernetes](guardrails/kubernetes/README.md)                                                         |
| [MCP](mcp/)                       | MCP tool servers behind the gateway                      | [compose](mcp/docker-compose/README.md), [kubernetes](mcp/kubernetes/README.md)                                                                       |
| [A2A](a2a/)                       | Agent-to-Agent server relaying to mock agents            | [compose](a2a/docker-compose/README.md), [kubernetes](a2a/kubernetes/README.md)                                                                       |
| [Hybrid](hybrid/)                 | Cloud and local providers together                       | [compose](hybrid/docker-compose/README.md), [kubernetes](hybrid/kubernetes/README.md)                                                                 |
| [Routing](routing/)               | Round-robin model pools with X-Selected-* headers        | [compose](routing/docker-compose/README.md)                                                                                                           |
| [Monitoring](monitoring/)         | Prometheus and Grafana around the gateway                | [compose](monitoring/docker-compose/README.md), [kubernetes](monitoring/kubernetes/README.md)                                                         |
| [TLS](tls/)                       | TLS termination, and mutual TLS under `mtls/`            | [kubernetes (tls)](tls/kubernetes/README.md), [compose (mtls)](tls/mtls/docker-compose/README.md), [kubernetes (mtls)](tls/mtls/kubernetes/README.md) |
| [Tools](tools/)                   | Function calling and MCP client-declared tools           | [agent (compose)](tools/agent/docker-compose/README.md), [mcp-client (compose)](tools/mcp-client/docker-compose/README.md)                            |
| [Agent](agent/)                   | A bespoke agent workload built on the gateway SDK        | [kubernetes](agent/kubernetes/README.md)                                                                                                              |
| [REST endpoints](rest-endpoints/) | curl against every endpoint                              | [README](rest-endpoints/README.md)                                                                                                                    |

## Prerequisites

- [Docker](https://docs.docker.com/get-docker/) and Docker Compose for the
  `docker-compose` flavours
- [Task](https://taskfile.dev/installation/), `kubectl`, `helm` and `ctlptl`
  for the `kubernetes` flavours

## Running a Docker Compose example

```bash
cd <domain>/docker-compose   # auth/<idp>, tools/<variant> and tls/mtls nest one level deeper
cp .env.example .env   # fill in your provider keys
docker compose up -d
```

## Running a Kubernetes example

Each `kubernetes` flavour installs the [Inference Gateway
Operator](https://github.com/inference-gateway/operator) and applies a `Gateway`
custom resource; traffic is served through the Kubernetes Gateway API (Envoy
Gateway) rather than ingress-nginx. Not every example does this - the
exceptions are listed below - but the usual shape is:

1. `task deploy-infrastructure` - provision a local k3d cluster and install the
   Gateway API CRDs, Envoy Gateway and the operator (plus any example-specific
   dependencies such as cert-manager, Prometheus or Keycloak).
2. `task deploy-inference-gateway` - apply `gateway.yaml`.
3. Reach the gateway by port-forwarding the Envoy data plane (each README shows
   the exact command), so no `/etc/hosts` changes are required:

   ```bash
   ENVOY_SVC=$(kubectl get svc -n envoy-gateway-system \
     -l gateway.envoyproxy.io/owning-gateway-name=inference-gateway \
     -o jsonpath='{.items[0].metadata.name}')
   kubectl -n envoy-gateway-system port-forward "svc/${ENVOY_SVC}" 8080:80
   curl -H 'Host: api.inference-gateway.local' http://localhost:8080/v1/models
   ```

Exceptions: [a2a/kubernetes](a2a/kubernetes/README.md) and
[agent/kubernetes](agent/kubernetes/README.md) install only the Gateway API
CRDs, with no Envoy Gateway - the a2a example reaches the gateway with
`task port-forward` (`service/inference-gateway`) and the agent example calls
it in-cluster; [tls/mtls/kubernetes](tls/mtls/kubernetes/README.md) calls the
gateway from a curl client pod inside the cluster instead.

The [../hack](../hack/README.md) folder is the related local dev cluster for
developing the gateway itself; it reuses the Keycloak manifests from the [auth
example](auth/keycloak/kubernetes/README.md).
