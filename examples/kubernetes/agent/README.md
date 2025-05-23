# Agent Example

This example demonstrates an agent-based deployment pattern with the Inference Gateway using:

- Custom logs analyzer agent
- Helm chart for gateway deployment
- Test deployment for agent monitoring

## Table of Contents

- [Agent Example](#agent-example)
  - [Table of Contents](#table-of-contents)
  - [Architecture](#architecture)
  - [Prerequisites](#prerequisites)
  - [Quick Start](#quick-start)
  - [Configuration](#configuration)
    - [Agent Settings](#agent-settings)
    - [Test Deployment](#test-deployment)
  - [Cleanup](#cleanup)

## Architecture

- **Gateway**: Inference Gateway deployed via helm chart
- **Agent**: Custom logs analyzer with cluster-wide access
- **Test Deployment**: Failing deployment for agent monitoring

## Prerequisites

- [Task](https://taskfile.dev/installation/)
- kubectl
- helm
- ctlptl (for cluster management)

## Quick Start

1. First deploy the cluster and registry:

```bash
task deploy-infrastructure
```

2. Deploy Inference Gateway:

```bash
task deploy-inference-gateway
```

3. Configure the API for the provider used in this example - Groq:

```bash
kubectl -n inference-gateway apply --server-side -f - <<EOF
apiVersion: v1
kind: Secret
metadata:
  name: inference-gateway
  namespace: inference-gateway
type: Opaque
stringData:
  GROQ_API_KEY: ""
EOF
```

Replace `GROQ_API_KEY` with your actual API key and apply it.
And restart the gateway to apply the changes:

```bash
kubectl -n inference-gateway rollout restart deployment inference-gateway
kubectl -n inference-gateway rollout status deployment inference-gateway
```

4. Build and push logs analyzer image (after registry is ready):

```bash
task build-logs-analyzer-agent
```

5. Deploy the logs analyzer and test deployment:

```bash
task deploy-logs-analyzer-agent
```

6. Monitor agent logs:

```bash
kubectl logs -f deployment/logs-analyzer -n logs-analyzer
```

## Configuration

### Agent Settings

- Edit YAMLs in `logs-analyzer/` directory
- Configure log collection patterns as needed
- Rebuild and redeploy after changes:

```bash
task build-logs-analyzer
task deploy-agent
```

### Test Deployment

- Edit YAMLs in `failing-deployment/` directory
- Simulate different failure scenarios

## Cleanup

```bash
task clean
```
