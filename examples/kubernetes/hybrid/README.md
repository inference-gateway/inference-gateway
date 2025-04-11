# Hybrid Deployment Example

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

- Configure via environment variables in Secret
- Set API keys for desired providers

## Cleanup

```bash
task clean
```
