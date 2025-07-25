# Monitoring Example with Enhanced Function/Tool Call Metrics

This example demonstrates monitoring integration with the Inference Gateway using:

- **Prometheus** for metrics collection
- **Grafana** for visualization with enhanced dashboards
- **Helm chart** for gateway deployment with monitoring enabled
- **Function/Tool Call Metrics** tracking MCP and A2A tool executions

## 🎯 What's New

This monitoring setup now includes comprehensive function/tool call metrics added in [PR #148](https://github.com/inference-gateway/inference-gateway/pull/148):

- `llm_tool_calls_total` - Counter for total function/tool calls
- `llm_tool_calls_success_total` - Counter for successful tool calls
- `llm_tool_calls_failure_total` - Counter for failed tool calls
- `llm_tool_call_duration` - Histogram for tool call execution duration

The enhanced Grafana dashboard provides 8 new panels for comprehensive tool call monitoring.

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
