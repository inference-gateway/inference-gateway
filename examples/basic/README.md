# Basic

The gateway with a single model provider - the smallest useful setup and the
usual starting point. Pick a provider, set its API key in `.env` and send an
OpenAI-compatible request through the gateway.

## Flavours

- [docker-compose](docker-compose/README.md) - the gateway and the provider
  behind `docker compose up`
- [kubernetes](kubernetes/README.md) - the same deployment through the
  [Inference Gateway Operator](https://github.com/inference-gateway/operator) and
  a `Gateway` custom resource
