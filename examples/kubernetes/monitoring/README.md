# Monitoring Example

This example demonstrates monitoring integration with the Inference Gateway using:

- Prometheus for metrics collection
- Grafana for visualization
- Helm chart for gateway deployment with monitoring enabled

## Table of Contents

- [Monitoring Example](#monitoring-example)
  - [Table of Contents](#table-of-contents)
  - [Architecture](#architecture)
  - [Prerequisites](#prerequisites)
  - [Quick Start](#quick-start)
  - [Configuration](#configuration)
    - [Monitoring Setup](#monitoring-setup)
    - [Gateway Monitoring](#gateway-monitoring)
  - [Cleanup](#cleanup)

## Architecture

- **Metrics Collection**: Prometheus scrapes gateway metrics
- **Visualization**: Grafana dashboards display metrics
- **Gateway**: Inference Gateway deployed via helm chart with monitoring enabled
- **Local LLM**: Ollama provider included for testing

## Prerequisites

- [Task](https://taskfile.dev/installation/)
- kubectl
- helm
- ctlptl (for cluster management)

## Quick Start

1. Deploy infrastructure:

```bash
task deploy-infrastructure
```

2. Deploy Inference Gateway with monitoring:

```bash
task deploy-inference-gateway
```

3. Access Grafana dashboards:

```bash
kubectl -n monitoring port-forward svc/grafana-service 3000:3000
```

Or use the deployed ingress, add `grafana.inference-gateway.local` DNS to your /etc/hosts and open: http://grafana.inference-gateway.local/d/inference-gateway/inference-gateway-metrics

Login credentials:

Username: admin
Password: admin

4. Deploy Ollama and simulate requests responses being sent to the gateway:

```
task deploy-ollama
```

```
task simulate-requests
```

## Configuration

### Monitoring Setup

- Edit YAMLs in `prometheus/` and `grafana/` directories
- Configure scrape intervals and dashboards as needed

### Gateway Monitoring

- Monitoring settings configured via helm values in Taskfile.yaml
- ServiceMonitor CRD enables Prometheus scraping

## Cleanup

```bash
task clean
```
