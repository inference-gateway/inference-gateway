# Monitoring with OTEL and Grafana Example

This example demonstrates how to deploy the Inference Gateway with OpenTelemetry monitoring and visualize metrics using Grafana in a local Kubernetes cluster.

## Prerequisites

- Docker
- ctlptl - CLI for declaratively setting up local Kubernetes clusters
- k3d - Lightweight Kubernetes distribution
- helm - Package manager for Kubernetes
- kubectl
- jq (optional, for parsing JSON responses)

## Components

This setup includes:

- Inference Gateway - The main application that proxies LLM requests
- Prometheus - Time-series database for storing metrics
- Grafana - Visualization platform for metrics

## Implementation Steps

1. Create the local cluster:

```bash
ctlptl apply -f Cluster.yaml

# Install Grafana and Prometheus
helm repo add grafana https://grafana.github.io/helm-charts
helm repo add prometheus-community https://prometheus-community.github.io/helm-charts
helm repo update

helm upgrade --install \
  grafana-operator grafana/grafana-operator \
  --namespace kube-system \
  --create-namespace \
  --version v5.16.0 \
  --set watch.namespaces={monitoring} \
  --wait
helm upgrade --install \
  prometheus-operator prometheus-community/kube-prometheus-stack \
  --namespace kube-system \
  --create-namespace \
  --version 69.6.0 \
  --set prometheus.prometheusSpec.serviceMonitorSelectorNilUsesHelmValues=false \
  --set-string prometheus.prometheusSpec.serviceMonitorNamespaceSelector.matchLabels.monitoring=true \
  --set prometheus.enabled=false \
  --set alertmanager.enabled=false \
  --set kubeStateMetrics.enabled=false \
  --set nodeExporter.enabled=false \
  --set grafana.enabled=false \
  --wait
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
  ENABLE_TELEMETRY: "true" # <-- Enable telemetry
  ENABLE_AUTH: "false"
  ...
```

3. Deploy the Inference Gateway:

```bash
kubectl create namespace inference-gateway --dry-run=client -o yaml | kubectl apply --server-side -f -
kubectl apply -f inference-gateway/
kubectl rollout status -n inference-gateway deployment/inference-gateway
kubectl label namespace inference-gateway monitoring="true" --overwrite # This is important so that the Prometheus Operator can discover the service monitors
```

And the monitoring components:

```bash
kubectl create namespace monitoring --dry-run=client -o yaml | kubectl apply --server-side -f -
kubectl apply -f grafana/
kubectl apply -f prometheus/
sleep 1
kubectl rollout status -n monitoring deployment/grafana-deployment
kubectl rollout status -n monitoring statefulset/prometheus-prometheus
kubectl label namespace monitoring monitoring="true" --overwrite # This is important so that the Prometheus Operator can discover the service monitors
```

4. Access the grafana dashboard:

```bash
kubectl -n monitoring port-forward svc/grafana-service 3000:3000
```

5. Open the browser and navigate to `http://localhost:3000`. Use the following credentials to log in:

- Username: `admin`
- Password: `admin`

Go to `Dashboards > monitoring > Inference Gateway Metrics` or just use the following link: `http://localhost:3000/d/inference-gateway/inference-gateway-metrics`.

6. Proxy the Inference Gateway service to your local machine:

```bash
kubectl port-forward svc/inference-gateway 8080:8080 -n inference-gateway
```

Send a bunch of requests:

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
ctlptl delete -f Cluster.yaml --cascade=true
```
