# Hybrid Deployment Example

## Table of Contents

- [Hybrid Deployment Example](#hybrid-deployment-example)
  - [Table of Contents](#table-of-contents)
  - [Architecture](#architecture)
  - [Prerequisites](#prerequisites)
  - [Quick Start](#quick-start)
  - [Configuration](#configuration)
    - [Local Provider](#local-provider)
    - [Cloud Providers](#cloud-providers)
  - [Cleanup](#cleanup)

This example demonstrates a hybrid deployment of the Inference Gateway using:

- Local Ollama provider
- Cloud-based providers
- Helm chart for gateway deployment

## Architecture

- **Gateway**: Inference Gateway deployed via helm chart
- **Local LLM**: Ollama provider for local model execution
- **Cloud Providers**: Configured via environment variables

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

3. Test local provider:

```bash
curl -X POST http://api.inference-gateway.local/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{"model":"ollama/deepseek-r1:1.5b","messages":[{"role":"user","content":"Hello"}]}'
```

## Configuration

### Local Provider

- Edit YAMLs in `ollama/` directory
- Configure model and resource requirements

### Cloud Providers

Set envFrom.secretRef in the `inference-gateway` deployment to reference a secret for configuring API keys for cloud providers.

- Example secret creation:

```bash
kubectl -n inference-gateway create secret generic inference-gateway \
  --from-literal=GROQ_API_KEY=your_api_key \
  --from-literal=ANTHROPIC_API_KEY=another_value
```

And restart the gateway to apply the changes:

```bash
kubectl -n inference-gateway rollout restart deployment inference-gateway
kubectl -n inference-gateway rollout status deployment inference-gateway
```

## Cleanup

```bash
task clean
```
