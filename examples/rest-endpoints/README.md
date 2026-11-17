# Example REST Endpoints

One curl per endpoint against a gateway on `http://localhost:8080`. Every variable
named below is documented in [Configurations.md](../../Configurations.md).

## Run the gateway

Against a real provider:

```bash
OLLAMA_CLOUD_API_KEY=<your key> go run ./cmd/gateway
```

Without any key, against the [tokenless](https://github.com/inference-gateway/tokenless) mock standing in for
OpenAI and Anthropic:

```bash
go run github.com/inference-gateway/tokenless/cmd/tokenless@latest --port 9090 &
OPENAI_API_URL=http://localhost:9090/v1 OPENAI_API_KEY=mock \
ANTHROPIC_API_URL=http://localhost:9090/v1 ANTHROPIC_API_KEY=mock \
DECISIONS_ENABLED=true IMAGES_ENABLED=true \
go run ./cmd/gateway
```

Swap `ollama_cloud/glm-5.3-flash` for `openai/gpt-5.6` in the chat examples when running against the mock.

## Models

```bash
curl http://localhost:8080/v1/models
```

- `?provider=<id>` lists one provider, e.g. `?provider=ollama_cloud`.
- `?include=context_window,pricing,modalities` adds listing metadata. Values the gateway cannot resolve are `null`.

```bash
curl 'http://localhost:8080/v1/models?provider=openai&include=context_window,pricing,modalities' | jq .
```

## Chat completions

```bash
curl http://localhost:8080/v1/chat/completions -H 'Content-Type: application/json' -d '{
  "model": "ollama_cloud/glm-5.3-flash",
  "messages": [{"role": "user", "content": "Hi"}]
}' | jq .
```

The model is `<provider>/<model>`. Add `"stream": true` (and `curl -N`) for SSE.

### Tool calls

The model answers with `tool_calls` - run the tool yourself and send its result back as a `tool` message:

```bash
curl http://localhost:8080/v1/chat/completions -H 'Content-Type: application/json' -d '{
  "model": "ollama_cloud/glm-5.3-flash",
  "messages": [{"role": "user", "content": "What is the weather in Toronto?"}],
  "tools": [{"type": "function", "function": {
    "name": "get_current_weather",
    "description": "Get the current weather of a city",
    "parameters": {"type": "object", "properties": {"city": {"type": "string"}}, "required": ["city"]}
  }}]
}' | jq '.choices[0].message.tool_calls'
```

### Images in messages

```bash
curl http://localhost:8080/v1/chat/completions -H 'Content-Type: application/json' -d '{
  "model": "openai/gpt-5.6",
  "messages": [{"role": "user", "content": [
    {"type": "text", "text": "What color is this pixel?"},
    {"type": "image_url", "image_url": {"url": "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8DwHwAFBQIAX8jx0gAAAABJRU5ErkJggg=="}}
  ]}]
}' | jq .
```

Image parts are forwarded untouched. With `VISION_ENABLED=true` they are stripped for models listed as text-only - see
[Vision/Multimodal Support](../../README.md#visionmultimodal-support).

## Anthropic Messages - `POST /v1/messages`

Forwarded byte for byte (only the `model` prefix is stripped), so `cache_control` and `"stream": true` work as
against Anthropic. Only `anthropic` serves it.

```bash
curl http://localhost:8080/v1/messages -H 'Content-Type: application/json' -d '{
  "model": "anthropic/claude-sonnet-5-5",
  "max_tokens": 1024,
  "messages": [{"role": "user", "content": "Hi"}]
}' | jq .
```

## OpenAI Responses - `POST /v1/responses`

Forwarded byte for byte. Only `openai` serves it.

```bash
curl http://localhost:8080/v1/responses -H 'Content-Type: application/json' -d '{
  "model": "openai/gpt-5.6",
  "input": "Hi"
}' | jq .
```

## Decisions - `POST /v1/decisions`

Needs `DECISIONS_ENABLED=true`. Served by `openai`, and translated for `ollama`, `ollama_cloud` and `llamacpp`
decision models (models whose output modalities include `decisions`). Questions are `predicate`, `choice` (with
`choices`) or `score` (with 2 to 10 `levels`).

```bash
curl http://localhost:8080/v1/decisions -H 'Content-Type: application/json' -d '{
  "model": "openai/gpt-6-luna",
  "input": "WIN AN IPHONE NOW! Click here to claim your prize.",
  "questions": [{"type": "predicate", "name": "is_spam", "instructions": "Is this spam?"}]
}' | jq .
```

## Image generation - `POST /v1/images/generations`

Needs `IMAGES_ENABLED=true`. Served by `openai`. A `"quality": "high"` image can take minutes - raise
`CLIENT_RESPONSE_HEADER_TIMEOUT`, `SERVER_READ_TIMEOUT` and `SERVER_WRITE_TIMEOUT` (e.g. `600s`) or the gateway
answers `502`/`504` first.

```bash
curl http://localhost:8080/v1/images/generations -H 'Content-Type: application/json' -d '{
  "model": "openai/gpt-image-2",
  "prompt": "A mountain lake at sunset",
  "quality": "low"
}' | jq -r '.data[0].b64_json' | base64 -d > lake.png
```

### Image edits - `POST /v1/images/edits`

Multipart upload with `image`, `prompt` and optional `mask`, `n`, `size`, `quality`. Served by `openai` and
`elevenlabs`.

```bash
curl 'http://localhost:8080/v1/images/edits?provider=openai' \
  -F image=@lake.png -F prompt='Add a flock of birds to the sky' -F model=gpt-image-2 | jq 'del(.data[].b64_json)'
```

## Speech - `POST /v1/audio/speech`

Needs `AUDIO_ENABLED=true`. Served by `openai`, `elevenlabs` (a voice id as `voice`, `mp3`/`opus`/`pcm`/`wav`) and
the built-in `local/qwen3-tts` engine. An optional base64 `reference_audio` clones a voice, `language` picks the
spoken language.

```bash
curl http://localhost:8080/v1/audio/speech -H 'Content-Type: application/json' -d '{
  "model": "elevenlabs/eleven_v3",
  "input": "Welcome aboard the Inference Gateway.",
  "voice": "JBFqnCBsd6RMkjVDRZzb"
}' -o speech.mp3
```

## Sound effects - `POST /v1/audio/sfx`

Needs `AUDIO_ENABLED=true`. Served by `elevenlabs` and `mirelo`. Set `video` (https URL or `data:video/...` URI) to
score a clip instead of a prompt.

```bash
curl http://localhost:8080/v1/audio/sfx -H 'Content-Type: application/json' -d '{
  "model": "elevenlabs/eleven_text_to_sound_v2",
  "prompt": "distant thunder rolling over a valley",
  "duration_seconds": 5
}' -o thunder.mp3
```

## Music - `POST /v1/audio/music`

Needs `AUDIO_ENABLED=true`. Served by `elevenlabs`.

```bash
curl http://localhost:8080/v1/audio/music -H 'Content-Type: application/json' -d '{
  "model": "elevenlabs/music_v2_5",
  "prompt": "chill lo-fi beat with a mellow piano loop",
  "duration_seconds": 15,
  "instrumental": true
}' -o lofi.mp3
```

## Videos - `POST /v1/videos`

Needs `VIDEOS_ENABLED=true`. Served by `elevenlabs`, `mirelo` (dubbing a `video` part) and the built-in
`local/musetalk` lip-sync engine. Creating returns a job - poll it until `status` is `completed`, then download. The
job `id` carries the provider (`elevenlabs:gen_abc123`), so send it back verbatim.

```bash
curl http://localhost:8080/v1/videos \
  -F model=elevenlabs/creatify-aurora -F input_reference=@portrait.png -F audio=@speech.mp3 | jq .
curl http://localhost:8080/v1/videos/elevenlabs:gen_abc123 | jq .status
curl http://localhost:8080/v1/videos/elevenlabs:gen_abc123/content -o video.mp4
```

## Provider passthrough - `/proxy/{provider}/{path...}`

Any method and path, appended to the provider's `<PROVIDER>_API_URL` with its configured key injected. Send
`Accept: text/event-stream` to stream.

```bash
curl http://localhost:8080/proxy/openai/models | jq .
```
