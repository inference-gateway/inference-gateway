# Hybrid Deployment Example

This example demonstrates a hybrid deployment of the Inference Gateway using:

- A local Ollama provider running in the cluster
- Cloud-based providers
- The [Inference Gateway Operator](https://github.com/inference-gateway/operator) and a `Gateway` custom resource

> **Note:** The Helm chart is deprecated. The gateway is now deployed through the operator. The Ollama base
> URL and cloud provider API keys are configured under `spec.providers` in `gateway.yaml`.

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

## Architecture

- **Gateway**: An `inference-gateway` `Gateway` custom resource reconciled by the operator.
- **Local LLM**: Ollama provider for local model execution (`spec.providers[].env` sets `OLLAMA_API_URL`).
- **Cloud Providers**: API keys read from the `inference-gateway-secrets` Secret.
- **Routing**: HTTP traffic via the Kubernetes Gateway API (Envoy Gateway).

## Prerequisites

- [Task](https://taskfile.dev/installation/)
- kubectl
- helm
- ctlptl (for cluster management)

## Quick Start

1. Deploy the infrastructure (cluster, Gateway API CRDs, Envoy Gateway and the operator):

   ```bash
   task deploy-infrastructure
   ```

2. Deploy the Ollama provider:

   ```bash
   task deploy-ollama
   ```

   You can also watch the model download progress — it will take a while:

   ```bash
   task watch-ollama-download
   ```

   Once you see "success", proceed to the next step.

3. Deploy the gateway:

   ```bash
   task deploy-inference-gateway
   ```

4. Port-forward the Envoy data plane and test the local provider:

   ```bash
   ENVOY_SVC=$(kubectl get svc -n envoy-gateway-system \
     -l gateway.envoyproxy.io/owning-gateway-name=inference-gateway \
     -o jsonpath='{.items[0].metadata.name}')
   kubectl -n envoy-gateway-system port-forward "svc/${ENVOY_SVC}" 8080:80 &

   curl -X POST http://localhost:8080/v1/chat/completions \
     -H "Content-Type: application/json" \
     -H "Host: api.inference-gateway.local" \
     -d '{"model":"ollama/deepseek-r1:1.5b","messages":[{"role":"user","content":"Hello"}]}'
   ```

   The response should look similar to:

   ```json
   {
     "choices": [
       {
         "finish_reason": "stop",
         "index": 0,
         "message": {
           "content": "<think>\n\n</think>\n\nHello! How can I assist you today? 😊",
           "role": "assistant"
         }
       }
     ],
     "created": 1747937295,
     "id": "chatcmpl-131",
     "model": "deepseek-r1:1.5b",
     "object": "chat.completion",
     "usage": {
       "completion_tokens": 16,
       "prompt_tokens": 4,
       "total_tokens": 20
     }
   }
   ```

## Configuration

### Local Provider

- Edit the YAMLs in the `ollama/` directory to configure the model and resource requirements.
- The gateway reaches Ollama via `OLLAMA_API_URL` (set on the `Ollama` provider in `gateway.yaml`).

### Cloud Providers

Add your cloud provider API keys to the `inference-gateway-secrets` Secret in `gateway.yaml` (e.g.
`GROQ_API_KEY`, `ANTHROPIC_API_KEY`), then re-apply:

```bash
kubectl apply -f gateway.yaml
```

The operator rolls out the gateway automatically when the spec or its referenced secrets change.

## Cleanup

```bash
task clean
```
