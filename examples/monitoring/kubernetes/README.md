# Monitoring Example with Enhanced Function/Tool Call Metrics

This example demonstrates monitoring integration with the Inference Gateway using:

- **Prometheus** for metrics collection
- **Grafana** for visualization with enhanced dashboards
- **Jaeger** as the OTLP trace backend for the OpenTelemetry spans the gateway exports
- The [Inference Gateway Operator](https://github.com/inference-gateway/operator) with telemetry and
  autoscaling enabled via the `Gateway` custom resource
- **Function/Tool Call Metrics** tracking MCP tool executions

> **Note:** The gateway is deployed through the operator with
> `spec.telemetry.metrics`, `spec.telemetry.traces` and `spec.hpa` enabled.

## Table of Contents

- [Monitoring Example with Enhanced Function/Tool Call Metrics](#monitoring-example-with-enhanced-functiontool-call-metrics)
  - [Table of Contents](#table-of-contents)
  - [Architecture](#architecture)
  - [Prerequisites](#prerequisites)
  - [Quick Start](#quick-start)
  - [Configuration](#configuration)
  - [Tracing](#tracing)
  - [Cleanup](#cleanup)

## Architecture

- **Metrics Collection**: Prometheus scrapes the gateway's `/metrics` endpoint (port 9464). The operator
  enables it via `spec.telemetry.metrics` - a thin `inference-gateway-metrics` Service (in `gateway.yaml`)
  carries the label the `ServiceMonitor` selects on.
- **Visualization**: Grafana dashboards display the metrics.
- **Gateway**: An `inference-gateway` `Gateway` custom resource with autoscaling (`spec.hpa`) enabled.
- **Local LLM**: Ollama provider included for generating traffic.

## Prerequisites

- [Task](https://taskfile.dev/installation/)
- kubectl
- helm
- ctlptl (for cluster management)

## Quick Start

1. Deploy the infrastructure (cluster, Gateway API CRDs, Envoy Gateway, the operator and the
   Prometheus/Grafana operators):

   ```bash
   task deploy-infrastructure
   ```

2. Deploy the gateway with monitoring and autoscaling:

   ```bash
   task deploy-inference-gateway
   ```

3. Access the Grafana dashboards (admin / admin):

   ```bash
   kubectl -n monitoring port-forward svc/grafana-service 3000:3000
   ```

   Then open <http://localhost:3000/d/inference-gateway/inference-gateway-metrics>.

4. Deploy Ollama and pull the models used by the simulations:

   ```bash
   task deploy-ollama
   task pull-models
   ```

5. In a separate terminal, port-forward the gateway, then generate traffic:

   ```bash
   task port-forward
   ```

   ```bash
   task simulate-requests
   task simulate-tool-call-requests
   ```

## Configuration

- **Monitoring**: edit the YAMLs in `prometheus/` (Prometheus instance, RBAC, ServiceMonitor) and `grafana/`
  (instance, datasource, dashboards).
- **Gateway telemetry & autoscaling**: configured in `gateway.yaml` under `spec.telemetry` and `spec.hpa`.

## Tracing

The gateway exports OpenTelemetry spans when the operator spec carries a traces
exporter. `gateway.yaml` sets:

```yaml
spec:
  telemetry:
    traces:
      exporter:
        otlp:
          endpoint: 'http://jaeger.monitoring.svc:4318'
```

The operator renders this into `TELEMETRY_TRACING_ENABLED=true` and
`TELEMETRY_TRACING_OTLP_ENDPOINT=http://jaeger.monitoring.svc:4318` on the gateway pod
(the gateway appends `/v1/traces` to a path-less endpoint). Deploy the backend the
endpoint points at - one Jaeger all-in-one Deployment and Service is enough for a
local cluster:

```bash
kubectl apply -n monitoring -f - <<'EOF'
apiVersion: apps/v1
kind: Deployment
metadata:
  name: jaeger
  labels:
    app: jaeger
spec:
  replicas: 1
  selector:
    matchLabels:
      app: jaeger
  template:
    metadata:
      labels:
        app: jaeger
    spec:
      containers:
        - name: jaeger
          image: jaegertracing/all-in-one:latest
          env:
            - name: COLLECTOR_OTLP_ENABLED
              value: 'true'
---
apiVersion: v1
kind: Service
metadata:
  name: jaeger
  labels:
    app: jaeger
spec:
  selector:
    app: jaeger
  ports:
    - name: ui
      port: 16686
      targetPort: 16686
    - name: otlp-http
      port: 4318
      targetPort: 4318
EOF
```

Re-apply the gateway (`task deploy-inference-gateway`) if it was already running, then
browse the traces:

```bash
kubectl -n monitoring port-forward svc/jaeger 16686:16686
```

Open <http://localhost:16686>, select the `inference-gateway` service and press **Find
Traces**. Every request except `/health` and `/v1/metrics` opens an `inference-gateway`
server span, annotated with `gen_ai.provider.name` and `gen_ai.request.model` - MCP tool
calls add `execute_tool <name>` spans, and the W3C trace context is propagated to MCP
servers and A2A agents so their spans nest under the gateway's. The
[docker-compose example](../docker-compose/README.md#-tracing) has the complete curl
walkthrough, including the MCP time server setup.

## Cleanup

```bash
task clean
```
