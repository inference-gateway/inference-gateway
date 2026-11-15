# Routing Docker Compose Example

Gateway-native round-robin model routing: the logical alias `fast-chat` is
resolved across a Groq and an OpenAI deployment, rotating per request.

## Overview

- Inference Gateway service with `ROUTING_ENABLED=true`
- `routing.yaml` pool file mounted at `/etc/inference-gateway/routing.yaml`
  (`ROUTING_CONFIG_PATH`)

The pool file defines the aliases; each alias needs at least two deployments to
rotate over, and every deployment `provider` must be one the gateway already
has configured (its API key set the usual way, e.g. `GROQ_API_KEY`).

## Prerequisites

- Docker
- Docker Compose
- A Groq and an OpenAI API key (or edit `routing.yaml` to list two other
  already-configured providers)

## Setup Instructions

1. Create a `.env` file based on the provided example:

```bash
cp .env.example .env
```

1. Edit the `.env` file to configure the pool's providers:

   ```bash
   GROQ_API_KEY=your_groq_api_key_here
   OPENAI_API_KEY=your_openai_api_key_here
   ```

1. Start the Inference Gateway:

```bash
docker compose up -d
```

1. Verify the gateway is running and see the configured aliases:

```bash
docker compose ps
docker compose logs inference-gateway | grep -i routing
```

## Testing the Gateway

Send a chat completion with the logical alias (no `provider/` prefix) and
inspect the response headers with `curl -i`:

```bash
curl -i -X POST http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{"model": "fast-chat", "messages": [{"role": "user", "content": "Hi"}]}'
```

Repeat the request: round-robin rotates through the deployments and the pick is
reported in two response headers - first

```text
X-Selected-Provider: groq
X-Selected-Model: llama-3.3-70b-versatile
```

then

```text
X-Selected-Provider: openai
X-Selected-Model: gpt-4o-mini
```

Round-robin state is per gateway replica, so the rotation order is per replica,
not coordinated across several replicas.

An explicit `?provider=` bypasses the pool: the alias is passed to that
provider unresolved and the `X-Selected-*` headers are absent. (In this example
the alias is not a real OpenAI model, so the provider errors - that is the
expected demonstration of the bypass.)

```bash
curl -i -X POST 'http://localhost:8080/v1/chat/completions?provider=openai' \
  -H "Content-Type: application/json" \
  -d '{"model": "fast-chat", "messages": [{"role": "user", "content": "Hi"}]}'
```

Routing applies to `/v1/chat/completions` only, and only when `?provider=` is
absent; `provider/model` prefixes and un-routed model names behave as always.

## Additional Resources

- [Routing example index](../README.md)
- [Main Documentation](../../../README.md)
- [Configuration Guide](../../../Configurations.md)
