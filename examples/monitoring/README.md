# Monitoring

Metrics, tracing and visualization: the gateway exposes Prometheus metrics on
`/v1/metrics` (telemetry must be enabled) and the examples wire Prometheus and
Grafana around it. The docker-compose flavour additionally runs Jaeger as the OTLP
trace backend so the gateway's OpenTelemetry spans - and the tool calls of the MCP
time server it bundles - have somewhere to go. Both flavours include a ready-made
gateway dashboard.

## Flavours

- [docker-compose](docker-compose/README.md) - the gateway, Prometheus, Grafana and
  Jaeger with provisioned dashboards and datasources
- [kubernetes](kubernetes/README.md) - the Prometheus and Grafana operators
  (`monitoring` namespace) plus Ollama as a scrapeable workload, through the
  [Inference Gateway Operator](https://github.com/inference-gateway/operator)
