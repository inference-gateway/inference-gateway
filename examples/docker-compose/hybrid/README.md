# Example using docker-compose

## Prerequisites

- Docker
- Docker Compose

## Quick Guide

Copy `.env.example` to `.env` and adjust the values (`.env` is added to gitignore and will not be tracked).

1. Bring the environment up:

```bash
docker compose up -d
```

2. List the available models of a specific API, for example Groq:

```bash
curl -X GET http://localhost:8080/v1/models?provider=groq | jq '.'
```

Or the local models:

```bash
curl -X GET http://localhost:8080/v1/models?provider=ollama | jq '.'
```

3. Use a specific API models, for example Groq:

```bash
curl -X POST http://localhost:8080/v1/chat/completions \
  -H 'Content-Type: application/json' \
  -d '{
    "model": "groq/llama3-8b-8192",
    "messages": [
      {
        "role": "system",
        "content": "You are a helpful assistant."
      },
      {
        "role": "user",
        "content": "Explain the importance of fast language models. Keep it short and concise."
      }
    ]
  }' | jq '.'
```

4. Or with streaming using Ollama:

```bash
# Download the models first
docker compose run --rm -it ollama-model-downloader
```

```bash
# List them
curl -X GET http://localhost:8080/v1/models?provider=ollama | jq '.'
```

```bash
curl -X POST http://localhost:8080/v1/chat/completions \
  -H 'Content-Type: application/json,text/event-stream' \
  -H 'Accept: application/json,text/event-stream' \
  -d '{
    "model": "ollama/qwen3:0.6b",
    "messages": [
      {
        "role": "system",
        "content": "You are a helpful assistant."
      },
      {
        "role": "user",
        "content": "Hi, can you tell me a joke?"
      }
    ],
    "stream": true
  }'
```
