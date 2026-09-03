# Hack

Local Kubernetes environment for developing the gateway itself. Unlike the
[operator examples](../examples/kubernetes), the gateway here is a plain Deployment
(`gateway.yaml`) so you can swap in a locally built image. Keycloak and cert-manager
manifests are reused from `examples/kubernetes/authentication`.

- [Hack](#hack)
  - [Prerequisites](#prerequisites)
  - [Available Tasks](#available-tasks)
    - [Cluster Management](#cluster-management)
    - [Gateway](#gateway)
    - [Keycloak Operations](#keycloak-operations)
    - [LLM Operations](#llm-operations)
  - [Typical Workflow](#typical-workflow)
  - [Cleanup](#cleanup)

## Prerequisites

- [Task](https://taskfile.dev/installation/)
- kubectl
- helm
- ctlptl (for cluster management)
- curl and jq

## Available Tasks

### Cluster Management

```bash
task deploy-infrastructure
```

Creates a local k3d cluster (k3s v1.36) with:

- Kubernetes Gateway API CRDs + Envoy Gateway (`envoy` GatewayClass)
- cert-manager with a self-signed ClusterIssuer
- Prometheus operator and Grafana operator (`observability` namespace)
- Keycloak (operator) with PostgreSQL and the `inference-gateway-realm` realm
- A CoreDNS rewrite so the gateway resolves `keycloak.inference-gateway.local` in-cluster

### Gateway

```bash
task deploy-inference-gateway
```

Publishes Keycloak's CA to the `inference-gateway` namespace and applies `gateway.yaml`
(Namespace, ConfigMap, Secret, Deployment, Service, Gateway and HTTPRoute). The gateway is
reachable through Envoy on `http://localhost` with the `Host: api.inference-gateway.local`
header.

To test a local build, push your image somewhere the cluster can pull from and change
`spec.template.spec.containers[0].image` in `gateway.yaml`.

### Keycloak Operations

```bash
task port-forward-keycloak      # keep running in another terminal
task fetch-access-token         # access token for the test user (user/password)
task print-access-token-payload
task keycloak-admin-password
```

### LLM Operations

```bash
task deploy-ollama              # Ollama with deepseek-r1:1.5b
task fetch-models
task generate-completions       # interactive prompt
```

## Typical Workflow

1. Start the infrastructure: `task deploy-infrastructure`
2. Deploy the gateway: `task deploy-inference-gateway`
3. In another terminal: `task port-forward-keycloak`
4. Deploy a provider: `task deploy-ollama`
5. Test: `task fetch-models`, `task generate-completions`

## Cleanup

```bash
task clean
```
