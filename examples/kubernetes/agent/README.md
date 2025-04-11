# Agent Example

This example demonstrates an agent-based deployment pattern with the Inference Gateway using:

- Custom logs analyzer agent
- Helm chart for gateway deployment
- Test deployment for agent monitoring

## Architecture

- **Gateway**: Inference Gateway deployed via helm chart
- **Agent**: Custom logs analyzer with cluster-wide access
- **Test Deployment**: Failing deployment for agent monitoring

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

2. Deploy Inference Gateway:

```bash
task deploy-inference-gateway
```

3. Monitor agent logs:

```bash
kubectl logs -f deployment/logs-analyzer -n agent-monitoring
```

## Configuration

### Agent Settings

- Edit YAMLs in `logs-analyzer/` directory
- Configure log collection patterns as needed

### Test Deployment

- Edit YAMLs in `failing-deployment/` directory
- Simulate different failure scenarios

## Cleanup

```bash
task clean
```
