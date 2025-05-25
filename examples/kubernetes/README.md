# Examples using Kubernetes

This directory contains examples that demonstrate how to use the Inference Gateway with Kubernetes.

- [Basic](basic/README.md)
- [Hybrid Environment](hybrid/README.md)
- [TLS](tls/README.md)
- [Authentication](authentication/README.md)
- [Agent Building](agent/README.md)
- [Monitoring](monitoring/README.md)
- [User Interface (UI)](ui/README.md)
- [Model Context Protocol (MCP)](mcp/README.md)

In order for the examples to work flawlessly you should add the following to your `/etc/hosts` file:

```bash
127.0.0.1 api.inference-gateway.local
127.0.0.1 keycloak.inference-gateway.local
127.0.0.1 grafana.inference-gateway.local
```
