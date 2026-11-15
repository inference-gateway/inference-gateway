# Monitoring Example with Enhanced Function/Tool Call Metrics

This example demonstrates monitoring integration with the Inference Gateway using:

- **Prometheus** for metrics collection
- **Grafana** for visualization with enhanced dashboards
- The [Inference Gateway Operator](https://github.com/inference-gateway/operator) with telemetry and
  autoscaling enabled via the `Gateway` custom resource
- **Function/Tool Call Metrics** tracking MCP tool executions

> **Note:** The gateway is deployed through the operator with
> `spec.telemetry.metrics` and `spec.hpa` enabled.

## Table of Contents

- [Monitoring Example with Enhanced Function/Tool Call Metrics](#monitoring-example-with-enhanced-functiontool-call-metrics)
  - [Table of Contents](#table-of-contents)
  - [Architecture](#architecture)
  - [Prerequisites](#prerequisites)
  - [Quick Start](#quick-start)
  - [Configuration](#configuration)
  - [Cleanup](#cleanup)

## Architecture

- **Metrics Collection**: Prometheus scrapes the gateway's `/metrics` endpoint (port 9464). The operator
  enables it via `spec.telemetry.metrics`; a thin `inference-gateway-metrics` Service (in `gateway.yaml`)
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

## Cleanup

```bash
task clean
```
