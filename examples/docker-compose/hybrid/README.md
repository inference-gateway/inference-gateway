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

The local model servers (Ollama and llama.cpp) are optional and off by default.
Pick one with its Compose profile:

```bash
docker compose --profile ollama up -d    # gateway + Ollama
docker compose --profile llamacpp up -d  # gateway + llama.cpp
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
    "model": "groq/llama-3.1-8b-instant",
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

4. Or with streaming using Ollama (start it first with `docker compose --profile ollama up -d`):

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

## Optional: local llama.cpp server

An optional [`llama.cpp`](https://github.com/ggml-org/llama.cpp) server
(`llama-server`, OpenAI-compatible) is included behind a Compose profile, so it
is **not** started by `docker compose up`. It is opt-in because the first start
downloads a GGUF model from HuggingFace, which can take a while.

1. Start the gateway together with the llama.cpp server:

```bash
docker compose --profile llamacpp up -d
```

The default model is `Qwen/Qwen2.5-0.5B-Instruct-GGUF:Q4_K_M` (tiny, no
HuggingFace token required). To use a different one, set `LLAMACPP_MODEL` in
`.env` to any HuggingFace GGUF repo (e.g. `LLAMACPP_MODEL=ggml-org/gemma-3-1b-it-GGUF`).

2. Follow the download / startup progress (first run only):

```bash
docker compose logs -f llamacpp
```

3. List the loaded model:

```bash
curl -X GET http://localhost:8080/v1/models?provider=llamacpp | jq '.'
```

4. Call it once the model has loaded:

```bash
curl -X POST http://localhost:8080/v1/chat/completions \
  -H 'Content-Type: application/json' \
  -d '{
    "model": "llamacpp/Qwen2.5-0.5B-Instruct",
    "messages": [
      {
        "role": "user",
        "content": "Hi, can you tell me a joke?"
      }
    ]
  }' | jq '.'
```
