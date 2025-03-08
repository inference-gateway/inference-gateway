# Monitoring with OTEL and Grafana Example

This example demonstrates how to deploy the Inference Gateway with OpenTelemetry monitoring via Prometheus Exporter and visualize metrics using Grafana in a local Kubernetes cluster.

## Prerequisites

- docker
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

Optionally deploy ollama for local LLMs:

```bash
kubectl create namespace ollama --dry-run=client -o yaml | kubectl apply --server-side -f -
kubectl apply -f ollama/
kubectl rollout status -n ollama deployment/ollama
```

Configure the Inference Gateway to use the local ollama service:

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
  ...
    OLLAMA_API_URL: "http://ollama.ollama:11434" # <-- Change to http://ollama.ollama:11434
  ...
```

1. Create the local cluster:

```bash
ctlptl apply -f Cluster.yaml

# Install Grafana and Prometheus Operators
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

Go to `Dashboards > monitoring > Inference Gateway Metrics` or just use the following link: [http://localhost:3000/d/inference-gateway/inference-gateway-metrics](http://localhost:3000/d/inference-gateway/inference-gateway-metrics).

6. Proxy the Inference Gateway service to your local machine:

```bash
kubectl port-forward svc/inference-gateway 8080:8080 -n inference-gateway
```

Send a bunch of requests to difference providers models:

```bash
declare -A PROVIDER_MODELS
PROVIDER_MODELS=(
  ["groq"]="llama-3.3-70b-versatile"
  ["cohere"]="command-r"
  ["ollama"]="tinyllama:latest"
)
PROVIDERS=("groq" "cohere" "ollama")
for PROVIDER in "${PROVIDERS[@]}"; do
  MODEL=${PROVIDER_MODELS[$PROVIDER]}
  echo "Testing $PROVIDER provider with model: $MODEL"
  curl -s -X POST http://localhost:8080/llms/$PROVIDER/generate -d "{
    \"model\": \"$MODEL\",
    \"messages\": [
      {\"role\": \"system\", \"content\": \"You are a helpful assistant.\"},
      {\"role\": \"user\", \"content\": \"Why is the sky blue? Keep it short and concise.\"}
    ]
  }" | jq '.'
  echo -e "\n------------------------------------\n"
  sleep 2
done
```

7. View the metrics in the Grafana dashboard.

8. When you're done, clean up the resources:

```bash
ctlptl delete -f Cluster.yaml --cascade=true
```
