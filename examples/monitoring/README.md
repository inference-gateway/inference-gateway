# Monitoring

Metrics collection and visualization: the gateway exposes Prometheus metrics on
`/v1/metrics` (telemetry must be enabled) and the examples wire Prometheus and
Grafana around it. Both flavours include a ready-made gateway dashboard.

## Flavours

- [docker-compose](docker-compose/README.md) - the gateway, Prometheus and
  Grafana with provisioned dashboards and datasources
- [kubernetes](kubernetes/README.md) - the Prometheus and Grafana operators
  (`observability` namespace) plus Ollama as a scrapeable workload, through the
  [Inference Gateway Operator](https://github.com/inference-gateway/operator)
