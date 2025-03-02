# Monitoring with OTEL and Grafana Example

This example demonstrates how to deploy the Inference Gateway with OpenTelemetry monitoring and visualize metrics using Grafana in a local Kubernetes cluster.

## Prerequisites

- Docker
- ctlptl - CLI for declaratively setting up local Kubernetes clusters
- k3d - Lightweight Kubernetes distribution
- kubectl
- Task - Task runner
- jq (optional, for parsing JSON responses)

## Components

This setup includes:

- Inference Gateway - The main application that proxies LLM requests
- OpenTelemetry Collector - Collects metrics from the Inference Gateway
- Prometheus - Time-series database for storing metrics
- Grafana - Visualization platform for metrics

## Implementation Steps

1. Create the local cluster:

```bash
task cluster-create
```

2. Enable telemetry in the Inference Gateway [configmap.yaml](inference-gateway/configmap.yaml):

```yaml
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: inference-gateway
  namespace: inference-gateway
  labels:
    app: inference-gateway
data:
  # General settings
  APPLICATION_NAME: "inference-gateway"
  ENVIRONMENT: "production"
  ENABLE_TELEMETRY: "true"
  ENABLE_AUTH: "false"
  ...
```

3. Deploy monitoring components and the Inference Gateway:

```bash
task deploy
```

4. Access the grafa dashboard:

```bash
task proxy-grafana
```

5. Open the browser and navigate to `http://localhost:3000`. Use the following credentials to log in:

- Username: `admin`
- Password: `admin`

6. Simulate a request to the Inference Gateway:

```bash
curl -X POST http://localhost:8080/llms/groq/generate -d '{
  "model": "llama-3.3-70b-versatile",
  "messages": [
    {"role": "system", "content": "You are a helpful assistant."},
    {"role": "user", "content": "Why is the sky blue? Keep it short and concise."}
  ]
}' | jq .
```

7. View the metrics in the Grafana dashboard.

8. When you're done, clean up the resources:

```bash
task cluster-delete
```
