# Example REST Endpoints

Assuming you've deployed the Inference Gateway, you can interact with the
language models through the REST endpoints. Below are some examples of how to
interact with the Inference Gateway using curl commands.

## GET Endpoints

| Description              | Curl Command                                                        |
| ------------------------ | ------------------------------------------------------------------- |
| List all models          | `curl -X GET http://localhost:8080/v1/models`                       |
| List Ollama models       | `curl -X GET http://localhost:8080/v1/models?provider=ollama`       |
| List Ollama Cloud models | `curl -X GET http://localhost:8080/v1/models?provider=ollama_cloud` |
| List Groq models         | `curl -X GET http://localhost:8080/v1/models?provider=groq`         |
| List OpenAI models       | `curl -X GET http://localhost:8080/v1/models?provider=openai`       |
| List Cloudflare models   | `curl -X GET http://localhost:8080/v1/models?provider=cloudflare`   |
| List Cohere models       | `curl -X GET http://localhost:8080/v1/models?provider=cohere`       |
| List Anthropic models    | `curl -X GET http://localhost:8080/v1/models?provider=anthropic`    |
| List DeepSeek models     | `curl -X GET http://localhost:8080/v1/models?provider=deepseek`     |
| List Google models       | `curl -X GET http://localhost:8080/v1/models?provider=google`       |
| List Mistral models      | `curl -X GET http://localhost:8080/v1/models?provider=mistral`      |
| List MiniMax models      | `curl -X GET http://localhost:8080/v1/models?provider=minimax`      |
| List Moonshot models     | `curl -X GET http://localhost:8080/v1/models?provider=moonshot`     |
| List Nvidia models       | `curl -X GET http://localhost:8080/v1/models?provider=nvidia`       |
| List llama.cpp models    | `curl -X GET http://localhost:8080/v1/models?provider=llamacpp`     |
| List ElevenLabs models   | `curl -X GET http://localhost:8080/v1/models?provider=elevenlabs`   |
| List Z.ai models         | `curl -X GET http://localhost:8080/v1/models?provider=zai`          |
| List WebWright models    | `curl -X GET http://localhost:8080/v1/models?provider=webwright`    |

### Context Windows

Pass `include=context_window` to enrich each model with its effective context
window. The value is resolved from the serving runtime when possible (llama.cpp
`/props`, Ollama `/api/show`), falling back to the window the provider publishes
in its model listing, and then to the community-maintained
[models.dev](https://github.com/sst/models.dev) table bundled with the gateway
(`source: "community"`). Only models absent from all three - local runtimes the
gateway cannot reach, or models missing from the dataset - carry an explicit
`null`.

```bash
curl -X GET 'http://localhost:8080/v1/models?include=context_window' | jq .
```

Response:

```json
{
  "object": "list",
  "data": [
    {
      "id": "llamacpp/qwen3-coder",
      "object": "model",
      "created": 1750000000,
      "owned_by": "qwen",
      "served_by": "llamacpp",
      "context_window": { "tokens": 32768, "source": "runtime" }
    },
    {
      "id": "mistral/mistral-large-latest",
      "object": "model",
      "created": 1750000000,
      "owned_by": "mistralai",
      "served_by": "mistral",
      "context_window": { "tokens": 262144, "source": "provider" }
    },
    {
      "id": "openai/gpt-5.6",
      "object": "model",
      "created": 1750000000,
      "owned_by": "openai",
      "served_by": "openai",
      "context_window": { "tokens": 1050000, "source": "community" }
    },
    {
      "id": "ollama/gpt-oss:20b",
      "object": "model",
      "created": 1750000000,
      "owned_by": "ollama",
      "served_by": "ollama",
      "context_window": null
    }
  ]
}
```

The parameter combines with `provider`, e.g.
`curl -X GET 'http://localhost:8080/v1/models?provider=ollama&include=context_window'`.
Without `include=context_window` the payload is unchanged and stays byte-for-byte
OpenAI-compatible.

### Pricing

Pass `include=pricing` to enrich each model with its normalized public per-token
pricing, resolved from rates the upstream provider publishes in its model
listing, falling back to the community-maintained
[models.dev](https://github.com/sst/models.dev) table bundled with the gateway
(`source: "community"`). Monetary values are decimal strings to avoid
floating-point precision loss. Rates that are not published (e.g. zero cache
rates) are omitted entirely, and only models absent from both the provider
listing and the community table carry an explicit `null`.

```bash
curl -X GET 'http://localhost:8080/v1/models?include=pricing' | jq .
```

Response:

```json
{
  "object": "list",
  "data": [
    {
      "id": "deepseek/deepseek-v4-pro",
      "object": "model",
      "created": 1750000000,
      "owned_by": "deepseek",
      "served_by": "deepseek",
      "pricing": {
        "currency": "USD",
        "input_per_token": "0.00000066",
        "output_per_token": "0.00000198",
        "cache_read_per_token": "0.000000022",
        "source": "provider"
      }
    },
    {
      "id": "groq/openai/gpt-oss-120b",
      "object": "model",
      "created": 1750000000,
      "owned_by": "meta",
      "served_by": "groq",
      "pricing": {
        "currency": "USD",
        "input_per_token": "0.00000015",
        "output_per_token": "0.0000006",
        "source": "provider"
      }
    },
    {
      "id": "openai/gpt-5.6",
      "object": "model",
      "created": 1750000000,
      "owned_by": "openai",
      "served_by": "openai",
      "pricing": {
        "currency": "USD",
        "input_per_token": "0.000004",
        "output_per_token": "0.00002",
        "cache_read_per_token": "0.0000004",
        "cache_write_per_token": "0.000005",
        "source": "community"
      }
    },
    {
      "id": "ollama/gpt-oss:20b",
      "object": "model",
      "created": 1750000000,
      "owned_by": "ollama",
      "served_by": "ollama",
      "pricing": null
    }
  ]
}
```

The parameter combines with `provider`, e.g.
`curl -X GET 'http://localhost:8080/v1/models?provider=deepseek&include=pricing'`.
Without `include=pricing` the payload is unchanged and stays byte-for-byte
OpenAI-compatible.

### Modalities

Pass `include=modalities` to enrich each model with the modalities it supports
natively, as an object with `input` and `output` arrays whose values are
`text`, `image`, `audio`, or `video`. The values are sourced from the
community-maintained
[models.dev](https://github.com/sst/models.dev) dataset; models the gateway
cannot resolve (local runtimes, or models absent from the dataset) carry an
explicit `null`.

```bash
curl -X GET 'http://localhost:8080/v1/models?include=modalities' | jq .
```

Response:

```json
{
  "object": "list",
  "data": [
    {
      "id": "openai/gpt-5.6",
      "object": "model",
      "created": 1750000000,
      "owned_by": "openai",
      "served_by": "openai",
      "modalities": { "input": ["text", "image"], "output": ["text"] }
    },
    {
      "id": "deepseek/deepseek-v4-pro",
      "object": "model",
      "created": 1750000000,
      "owned_by": "deepseek",
      "served_by": "deepseek",
      "modalities": { "input": ["text"], "output": ["text"] }
    },
    {
      "id": "ollama/gpt-oss:20b",
      "object": "model",
      "created": 1750000000,
      "owned_by": "ollama",
      "served_by": "ollama",
      "modalities": null
    }
  ]
}
```

The parameter combines with `provider`, e.g.
`curl -X GET 'http://localhost:8080/v1/models?provider=openai&include=modalities'`.
Without `include=modalities` the payload is unchanged and stays byte-for-byte
OpenAI-compatible.

## POST Endpoints

| Domain                            | Curl Command                                                                                                                                                                                                                   |
| --------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| ollama.local                      | `curl -X POST http://localhost:8080/v1/chat/completions -d '{"model":"ollama/gpt-oss:20b","messages":[{"role":"system","content":"You are a helpful assistant."},{"role":"user","content":"Hi"}]}'`                            |
| ollama.com                        | `curl -X POST http://localhost:8080/v1/chat/completions -d '{"model":"ollama_cloud/kimi-k3","messages":[{"role":"system","content":"You are a helpful assistant."},{"role":"user","content":"Hi"}]}'`                          |
| api.groq.com                      | `curl -X POST http://localhost:8080/v1/chat/completions -d '{"model":"groq/openai/gpt-oss-120b","messages":[{"role":"system","content":"You are a helpful assistant."},{"role":"user","content":"Hi"}]}'`                      |
| api.openai.com                    | `curl -X POST http://localhost:8080/v1/chat/completions -d '{"model":"openai/gpt-5.6","messages":[{"role":"system","content":"You are a helpful assistant."},{"role":"user","content":"Hi"}]}'`                                |
| api.cloudflare.com                | `curl -X POST http://localhost:8080/v1/chat/completions -d '{"model":"cloudflare/@cf/zai-org/glm-5.3","messages":[{"role":"system","content":"You are a helpful assistant."},{"role":"user","content":"Hi"}]}'`                |
| api.cohere.com                    | `curl -X POST http://localhost:8080/v1/chat/completions -d '{"model":"cohere/command-a-plus-05-2026","messages":[{"role":"system","content":"You are a helpful assistant."},{"role":"user","content":"Hi"}]}'`                 |
| api.anthropic.com                 | `curl -X POST http://localhost:8080/v1/chat/completions -d '{"model":"anthropic/claude-opus-5-5","messages":[{"role":"system","content":"You are a helpful assistant."},{"role":"user","content":"Hi"}]}'`                     |
| api.deepseek.com                  | `curl -X POST http://localhost:8080/v1/chat/completions -d '{"model":"deepseek/deepseek-v4-pro","messages":[{"role":"system","content":"You are a helpful assistant."},{"role":"user","content":"Hi"}]}'`                      |
| generativelanguage.googleapis.com | `curl -X POST http://localhost:8080/v1/chat/completions -d '{"model":"google/models/gemini-4-argon","messages":[{"role":"system","content":"You are a helpful assistant."},{"role":"user","content":"Hi"}]}'`                  |
| api.mistral.ai                    | `curl -X POST http://localhost:8080/v1/chat/completions -d '{"model":"mistral/mistral-large-latest","messages":[{"role":"system","content":"You are a helpful assistant."},{"role":"user","content":"Hi"}]}'`                  |
| api.minimax.io                    | `curl -X POST http://localhost:8080/v1/chat/completions -d '{"model":"minimax/MiniMax-M3.1-Flash","messages":[{"role":"system","content":"You are a helpful assistant."},{"role":"user","content":"Hi"}]}'`                    |
| api.moonshot.ai                   | `curl -X POST http://localhost:8080/v1/chat/completions -d '{"model":"moonshot/kimi-k3","messages":[{"role":"system","content":"You are a helpful assistant."},{"role":"user","content":"Hi"}]}'`                              |
| integrate.api.nvidia.com          | `curl -X POST http://localhost:8080/v1/chat/completions -d '{"model":"nvidia/nvidia/nemotron-3.5-super-vl-120b-a12b","messages":[{"role":"system","content":"You are a helpful assistant."},{"role":"user","content":"Hi"}]}'` |
| llamacpp.local                    | `curl -X POST http://localhost:8080/v1/chat/completions -d '{"model":"llamacpp/qwen3-coder","messages":[{"role":"system","content":"You are a helpful assistant."},{"role":"user","content":"Hi"}]}'`                          |
| api.z.ai                          | `curl -X POST http://localhost:8080/v1/chat/completions -d '{"model":"zai/glm-5.3","messages":[{"role":"system","content":"You are a helpful assistant."},{"role":"user","content":"Hi"}]}'`                                   |
| webwright.ai                      | `curl -X POST http://localhost:8080/v1/chat/completions -d '{"model":"webwright/wren-r1","messages":[{"role":"system","content":"You are a helpful assistant."},{"role":"user","content":"Hi"}]}'`                             |

You can set the stream as an optional flag in the request body to enable
streaming of tokens. The default value is `false`.

```bash
curl -X POST http://localhost:8080/v1/chat/completions -d '{
  "model": "groq/openai/gpt-oss-120b",
  "messages": [
    {
      "role": "system",
      "content": "You are a helpful assistant."
    },
    {
      "role": "user",
      "content": "Hi, how are you doing today?"
    }
  ]
}' | jq .
```

Response:

```json
{
  "id": "chatcmpl-3f5a1d9e",
  "object": "chat.completion",
  "created": 1792570321,
  "model": "openai/gpt-oss-120b",
  "choices": [
    {
      "index": 0,
      "message": {
        "content": "Hello! I'm doing well, thanks. How can I help you today?",
        "role": "assistant"
      },
      "finish_reason": "stop"
    }
  ],
  "usage": {
    "prompt_tokens": 19,
    "completion_tokens": 12,
    "total_tokens": 31
  }
}
```

## Anthropic Messages API

The gateway also exposes a native Anthropic-compatible `POST /v1/messages`
endpoint. The request body is forwarded to the provider byte-for-byte (only the
`model` prefix is stripped), so Anthropic-specific fields like `cache_control`
pass through untouched and cache usage fields round-trip back to the client.
Only providers that natively implement the Messages API are supported
(currently `anthropic`); other providers return `400` in the Anthropic error
envelope.

```bash
curl -X POST http://localhost:8080/v1/messages -d '{
  "model": "anthropic/claude-sonnet-5-5",
  "max_tokens": 1024,
  "messages": [
    {"role": "user", "content": "Hi, how are you doing today?"}
  ]
}' | jq .
```

Response:

```json
{
  "id": "msg_01XFDUDYJgAACzvnptvVoYEL",
  "type": "message",
  "role": "assistant",
  "model": "claude-sonnet-5-5",
  "content": [{ "type": "text", "text": "Hi! I'm doing well, thanks." }],
  "stop_reason": "end_turn",
  "usage": {
    "input_tokens": 12,
    "output_tokens": 10,
    "cache_creation_input_tokens": 0,
    "cache_read_input_tokens": 0
  }
}
```

Set `"stream": true` to receive the Anthropic SSE event envelope
(`message_start`, `content_block_delta`, `message_stop`, ...) verbatim:

```bash
curl -N -X POST http://localhost:8080/v1/messages -d '{
  "model": "anthropic/claude-sonnet-5-5",
  "max_tokens": 1024,
  "stream": true,
  "messages": [
    {"role": "user", "content": "Hi"}
  ]
}'
```

Prompt caching works as it does against Anthropic directly - mark a breakpoint
with `cache_control` and read the cache usage from the response:

```bash
curl -X POST http://localhost:8080/v1/messages -d '{
  "model": "anthropic/claude-sonnet-5-5",
  "max_tokens": 1024,
  "system": [
    {
      "type": "text",
      "text": "You are a helpful assistant. <large static context here>",
      "cache_control": {"type": "ephemeral"}
    }
  ],
  "messages": [
    {"role": "user", "content": "Hi"}
  ]
}' | jq .usage
```

Errors generated by the gateway use the Anthropic error envelope, e.g. for a
provider without native Messages support:

```json
{
  "type": "error",
  "error": {
    "type": "not_supported_error",
    "message": "The Messages API is not supported by this provider yet."
  }
}
```

## OpenAI Responses API

The gateway exposes an OpenAI-compatible `POST /v1/responses` endpoint. The
request body is forwarded to the upstream provider byte-for-byte (only the
`model` prefix is stripped), so all Responses API fields like `input`,
`instructions`, and `tools` pass through untouched. Only providers that natively
implement the Responses API are supported (currently `openai`); other providers
return `400`.

```bash
curl -X POST http://localhost:8080/v1/responses -d '{
  "model": "openai/gpt-5.6",
  "input": "Hi, how are you doing today?",
  "instructions": "You are a helpful assistant."
}' | jq .
```

Response:

```json
{
  "id": "resp_67b5cf1e3e3481928c7a3b2f1a2b3c4d",
  "object": "response",
  "created_at": 1792570321,
  "status": "completed",
  "model": "gpt-5.6",
  "output": [
    {
      "type": "message",
      "id": "msg_67b5cf1e3e3481928c7a3b2f1a2b3c4d",
      "status": "completed",
      "role": "assistant",
      "content": [
        {
          "type": "output_text",
          "text": "Hello! I'm doing well, thank you! How can I help you today?",
          "annotations": []
        }
      ]
    }
  ],
  "usage": {
    "input_tokens": 12,
    "output_tokens": 10,
    "total_tokens": 22
  }
}
```

Set `"stream": true` to receive `ResponseStreamEvent` SSE frames verbatim:

```bash
curl -N -X POST http://localhost:8080/v1/responses -d '{
  "model": "openai/gpt-5.6",
  "input": "Hi",
  "stream": true
}'
```

Errors generated by the gateway use the standard error envelope, e.g. for a
provider without native Responses API support:

```json
{
  "error": "The Responses API is not supported by this provider yet. Use /v1/chat/completions instead."
}
```

## Image Generation

The gateway exposes an OpenAI-compatible `POST /v1/images/generations` endpoint
for generating images from text prompts. The request body is forwarded to the
upstream provider byte-for-byte (only the `model` prefix is stripped), so all
Images API fields like `prompt`, `n`, `size`, `quality`, and `response_format`
pass through untouched.

The endpoint is opt-in via `IMAGES_ENABLED=true` (default off). When disabled,
the handler returns `404`.

Only providers that natively implement the Images API are supported (currently
OpenAI); other providers return `400`.

Image generation is slow `gpt-image-2` at `"quality": "high"` took ~190s in
practice, far past the default client and server timeouts. Raise them or the
gateway returns `502`/`504` before the provider answers:

```bash
CLIENT_RESPONSE_HEADER_TIMEOUT=600s
SERVER_READ_TIMEOUT=600s
SERVER_WRITE_TIMEOUT=600s
```

```bash
curl -X POST http://localhost:8080/v1/images/generations -d '{
  "model": "openai/gpt-image-2",
  "prompt": "A serene mountain lake at sunset with pine trees reflected in the water, digital art",
  "n": 1,
  "size": "1024x1024",
  "quality": "high"
}' | jq '.data[0].b64_json' -r | base64 -d > lake.png
```

The `gpt-image-*` models always return base64 in `b64_json` there is no `url`
field and `response_format` does not apply. Response (image data elided):

```json
{
  "created": 1785508178,
  "data": [{ "b64_json": "iVBORw0KGgoAAAANSUhEUgAA..." }],
  "background": "opaque",
  "output_format": "png",
  "quality": "high",
  "size": "1024x1024",
  "usage": {
    "input_tokens": 22,
    "output_tokens": 7024,
    "total_tokens": 7046
  }
}
```

`quality` takes `low`, `medium`, or `high`. It is the main cost lever at
1024x1024, `low` is roughly 35x cheaper than `high`.

### Image Edits

`POST /v1/images/edits` accepts `multipart/form-data` uploads and reuses the
same `ImagesResponse` shape, toggle (`IMAGES_ENABLED`), and provider support
rules as `/images/generations`. It edits or extends a source image: it requires
the `image` file and a `prompt`, with optional `mask`, `model`, `n` (1-10),
`size`, `quality`, and `response_format` (`url` or `b64_json`).

The gateway streams the uploaded `image` (and optional `mask`) straight through
to the provider without buffering the whole payload, so large images stay cheap.

```bash
# Edit an image
curl -X POST "http://localhost:8080/v1/images/edits?provider=openai" \
  -H "Authorization: Bearer $TOKEN" \
  -F image="@sunset.png" \
  -F mask="@mask.png" \
  -F prompt="Add a flock of birds to the sky" \
  -F model="gpt-image-2" \
  -F n=1 \
  -F size="1024x1024"
```

The response is an `ImagesResponse`:

```json
{
  "created": 1792570901,
  "data": [{ "b64_json": "iVBORw0KGgoAAAANSUhEUgAA..." }]
}
```

Errors generated by the gateway use the standard error envelope, e.g. when the
Images API is not enabled:

```json
{
  "error": "The Images API is not enabled. Set IMAGES_ENABLED=true to enable it."
}
```

Or for a provider without native Images API support:

```json
{
  "error": "The Images API is not supported by this provider yet."
}
```

## Text to Speech

The gateway exposes an OpenAI-compatible `POST /v1/audio/speech` endpoint for
synthesizing speech from text. The request body is forwarded to the upstream
provider byte-for-byte (only the `model` prefix is stripped), and the response
is the raw binary audio with the upstream's `Content-Type` (e.g. `audio/wav`
for `"response_format": "wav"`).

The endpoint is opt-in via `AUDIO_ENABLED=true` (default off). When disabled,  
the handler returns `404`. Providers without a native speech API return `400`.

The supported backends today are the built-in `local/qwen3-tts` engine and the
`openai` and `elevenlabs` providers. Speech through the `llamacpp` provider is a
work in progress and not supported yet - the examples below that use
`llamacpp/...` describe the intended behaviour and are not expected to work.

```bash
curl -X POST http://localhost:8080/v1/audio/speech -d '{
  "model": "local/qwen3-tts",
  "input": "Ahoy! Welcome aboard the Inference Gateway.",
  "voice": "alloy",
  "response_format": "wav"
}' -o speech.wav
```

### ElevenLabs

ElevenLabs' speech API is not OpenAI-compatible, so the gateway rewrites the
request instead of proxying it. `voice` is required and must be an ElevenLabs
voice id (not a name like `alloy`), and `response_format` is limited to `mp3`
(default), `opus`, or `pcm`. `speed` and `language` are forwarded.

```bash
curl -X POST http://localhost:8080/v1/audio/speech -d '{
  "model": "elevenlabs/eleven_v3",
  "input": "Ahoy! Welcome aboard the Inference Gateway.",
  "voice": "JBFqnCBsd6RMkjVDRZzb",
  "response_format": "mp3"
}' -o speech.mp3
```

An unsupported format returns `400`:

```json
{
  "error": "elevenlabs does not support response_format \"wav\", supported formats: mp3, opus, pcm"
}
```

### Voice Cloning

The optional `reference_audio` field carries a base64-encoded voice sample for
zero-shot cloning; the generated speech mimics the voice in the sample. Use a
clean mono recording between 1 and 30 seconds (WAV is the safest container).
The field is forwarded as-is, so it works with providers whose speech backend
supports audio-conditioned cloning, such as a self-hosted Qwen3-TTS-compatible
server configured as the `llamacpp` provider via `LLAMACPP_API_URL`. OpenAI's
Speech API does not support cloning and only accepts its built-in voices.

```bash
curl -X POST http://localhost:8080/v1/audio/speech -d "{
  \"model\": \"llamacpp/qwen3-tts\",
  \"input\": \"This is my cloned voice speaking.\",
  \"voice\": \"custom\",
  \"response_format\": \"wav\",
  \"reference_audio\": \"$(base64 < my-voice-sample.wav)\"
}" -o cloned.wav
```

### Language

The optional `language` field is an ISO 639-1 code (default `en`) hinting at  
the utterance language. The local engine accepts `zh`, `en`, `de`, `it`, `pt`,  
`es`, `ja`, `ko`, `fr`, and `ru`; any other code is rejected with `400`. For  
proxied providers the field is forwarded as-is. It combines with  
`reference_audio`, so a single request can clone a voice and have it speak  
another language:

```bash
curl -X POST http://localhost:8080/v1/audio/speech -d "{
  \"model\": \"local/qwen3-tts\",
  \"input\": \"Guten Tag! Willkommen an Bord der Inference Gateway.\",
  \"language\": \"de\",
  \"reference_audio\": \"$(base64 < my-voice-sample.wav)\"
}" -o german-cloned.wav
```

An unsupported code for the local engine returns `400`:

```json
{
  "error": "The local speech engine does not support language \"xx\". Supported languages: zh, en, de, it, pt, es, ja, ko, fr, ru."
}
```

### Local engine

The reserved `local/qwen3-tts` model is served by the gateway itself via
llama.cpp's `llama-tts` instead of a provider. Output is always WAV (omit
`response_format` or set it to `wav`); `reference_audio` cloning works the
same way. Assets download in the background at startup (see
`AUDIO_LOCAL_AUTO_DOWNLOAD`); until they are ready the endpoint returns `503`
with a `Retry-After` header.

```bash
curl -X POST http://localhost:8080/v1/audio/speech -d '{
  "model": "local/qwen3-tts",
  "input": "No provider needed for this one."
}' -o local.wav
```

Errors use the standard envelope, e.g. when the Audio API is not enabled:

```json
{
  "error": "The Audio API is not enabled. Set AUDIO_ENABLED=true to enable it."
}
```

## Sound Effects

`POST /v1/audio/sfx` is a gateway extension with no OpenAI counterpart: it
generates a non-speech clip - a sound effect or ambience - from a text prompt.
JSON in, raw audio out, same as `/audio/speech`. It shares the `AUDIO_ENABLED`
toggle and is currently served by the `elevenlabs` provider only; other
providers return `400`.

`prompt` is required. Optional fields: `duration_seconds`, `prompt_influence`
(0-1, how closely to follow the prompt), `loop` (seamless loop), and
`response_format` (`mp3` default, `opus`, or `pcm`).

```bash
curl -X POST http://localhost:8080/v1/audio/sfx -d '{
  "model": "elevenlabs/eleven_text_to_sound_v2",
  "prompt": "distant thunder rolling over a valley",
  "duration_seconds": 5,
  "loop": true
}' -o thunder.mp3
```

For a provider without sound-effect support:

```json
{
  "error": "Sound effect generation is not supported by this provider yet."
}
```

## Music Generation

`POST /v1/audio/music` is a gateway extension like `/audio/sfx`: it composes a
music clip from a text prompt. JSON in, raw audio out. It shares the
`AUDIO_ENABLED` toggle and is currently served by the `elevenlabs` provider
only; other providers return `400`.

`prompt` is required. Optional fields: `duration_seconds` (3-600 seconds,
omitted lets the provider pick a length that fits the prompt), `instrumental`
(guarantee no vocals), and `response_format` (`mp3` default, `opus`, `aac`,
`flac`, `wav`, or `pcm` - elevenlabs accepts `mp3`, `opus`, or `pcm`).

```bash
curl -X POST http://localhost:8080/v1/audio/music -d '{
  "model": "elevenlabs/music_v2_5",
  "prompt": "chill lo-fi hip hop beat with a mellow piano loop, 80 bpm",
  "duration_seconds": 15,
  "instrumental": true
}' -o lofi.mp3
```

For a provider without music support:

```json
{
  "error": "Music generation is not supported by this provider yet."
}
```

## Video Generation

The gateway exposes an OpenAI-compatible Videos API for asynchronous video
generation. `POST /v1/videos` takes `multipart/form-data` and returns a job
immediately; poll `GET /v1/videos/{video_id}` until `status` is `completed`,
then download the bytes from `GET /v1/videos/{video_id}/content`.

The endpoint is opt-in via `VIDEOS_ENABLED=true` (default off). When disabled,
the handlers return `404`. Only the `elevenlabs` provider supports it today;
other providers return `400`.

The job `id` carries the provider (`elevenlabs:gen_abc123`) because the gateway
keeps no job state. Send it back verbatim; `?provider=` still overrides it.

Form fields: `model` (required), `prompt`, `input_reference` (image used as the
first frame, or the portrait for avatar models), `seconds`, `size`
(`widthxheight`), and two non-standard fields: `audio` (a WAV or MP3 clip the
avatar lip-syncs to; when present `seconds` is ignored) and `reference_images`
(repeat the field once per image of the subject; `veo-3.1-*` takes up to 3,
`bytedance-seedance-v2*` up to 9 and `bytedance-seedance-v2.5` up to 30; it
cannot be combined with `input_reference`).

```bash
# Create a job: animate a portrait to speak the given audio
curl -X POST http://localhost:8080/v1/videos \
  -F model="elevenlabs/creatify-aurora" \
  -F prompt="Medium shot, presenter facing the camera" \
  -F input_reference="@portrait.png" \
  -F audio="@speech.mp3" \
  -F size="720x1280" | jq .
```

Response:

```json
{
  "id": "elevenlabs:gen_abc123",
  "object": "video",
  "model": "creatify-aurora",
  "status": "queued",
  "created_at": 1758500000
}
```

Poll the job:

```bash
curl -X GET http://localhost:8080/v1/videos/elevenlabs:gen_abc123 | jq .
```

```json
{
  "id": "elevenlabs:gen_abc123",
  "object": "video",
  "model": "creatify-aurora",
  "status": "completed",
  "progress": 100,
  "created_at": 1758500000,
  "completed_at": 1758500090
}
```

Download the render once completed:

```bash
curl -X GET http://localhost:8080/v1/videos/elevenlabs:gen_abc123/content -o video.mp4
```

A job that is still `queued`, `in_progress`, or `failed` has no bytes to serve
and `/content` returns `404`:

```json
{
  "error": "The rendered video is not available yet. Poll GET /v1/videos/{video_id} until the status is completed."
}
```

Errors use the standard envelope, e.g. when the Videos API is not enabled:

```json
{
  "error": "The Videos API is not enabled. Set VIDEOS_ENABLED=true to enable it."
}
```

## Tool Calls

You can provide tools that the LLM can use to perform specific functions. Here
are some examples:

```bash
curl -X POST http://localhost:8080/v1/chat/completions -d '{
  "model": "groq/openai/gpt-oss-120b",
  "messages": [
    {"role": "system", "content": "You are a helpful assistant."},
    {"role": "user", "content": "What is the current weather in Toronto?"}
  ],
  "tools": [
    {
      "type": "function",
      "function": {
        "name": "get_current_weather",
        "description": "Get the current weather of a city",
        "parameters": {
          "type": "object",
          "properties": {
            "city": {
              "type": "string",
              "description": "The name of the city"
            }
          },
          "required": ["city"]
        }
      }
    }
  ]
}' | jq .
```

Then the LLM will respond with a function call request as follow:

```json
{
  "choices": [
    {
      "finish_reason": "tool_calls",
      "index": 0,
      "message": {
        "content": "",
        "role": "assistant",
        "tool_calls": [
          {
            "function": {
              "arguments": "{\"city\": \"Toronto\"}",
              "name": "get_current_weather"
            },
            "id": "call_gsq01a2b4c9",
            "type": "function"
          }
        ]
      }
    }
  ],
  "created": 1792570327,
  "id": "chatcmpl-f14e1a5f-0c05-4c94-be1c-6d23f0c1ecb6",
  "model": "openai/gpt-oss-120b",
  "object": "chat.completion",
  "usage": {
    "completion_tokens": 18,
    "prompt_tokens": 91,
    "total_tokens": 109
  }
}
```

So you can response with the function call content as follow:

```bash
curl -X POST http://localhost:8080/v1/chat/completions -d '{
  "model": "groq/openai/gpt-oss-120b",
  "messages": [
    {"role": "system", "content": "You are a helpful assistant."},
    {"role": "user", "content": "What is the current weather in Toronto?"},
    {"role": "assistant", "tool_calls": [{"type": "function", "id": "call_gsq01a2b4c9", "function": {"name": "get_current_weather", "arguments": "{\"city\": \"Toronto\"}"}}]},
    {"role": "tool", "content": "89F", "tool_call_id": "call_gsq01a2b4c9"}
  ]
}' | jq .
```

Then the LLM will respond with the weather information as follow:

```json
{
  "choices": [
    {
      "finish_reason": "stop",
      "index": 0,
      "message": {
        "content": "The current weather in Toronto is 89°F. Let me know if you'd like conditions or the forecast for another city.",
        "role": "assistant"
      }
    }
  ],
  "created": 1792570362,
  "id": "chatcmpl-12a36044-74b5-4fd7-973b-b31333d1118e",
  "model": "openai/gpt-oss-120b",
  "object": "chat.completion",
  "usage": {
    "completion_tokens": 21,
    "prompt_tokens": 128,
    "total_tokens": 149
  }
}
```

Then you would append it to the conversation and so on.

## Multimodal Image Content

The gateway supports vision-capable models that can process both text and
images. You can send images using either data URLs or HTTP URLs.

### Example: Image with Text (HTTP URL)

```bash
curl -X POST http://localhost:8080/v1/chat/completions -d '{
  "model": "openai/gpt-5.6",
  "messages": [
    {
      "role": "user",
      "content": [
        {
          "type": "text",
          "text": "What is in this image?"
        },
        {
          "type": "image_url",
          "image_url": {
            "url": "https://upload.wikimedia.org/wikipedia/commons/thumb/d/dd/Gfp-wisconsin-madison-the-nature-boardwalk.jpg/2560px-Gfp-wisconsin-madison-the-nature-boardwalk.jpg"
          }
        }
      ]
    }
  ]
}' | jq .
```

### Example: Image with Base64 Data URL

You can also send images directly as base64-encoded data URLs. This is useful
when you have the image data locally or want to avoid hosting the image
externally.

```bash
curl -X POST http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "openai/gpt-5.6-luna",
    "messages": [{
      "role": "user",
      "content": [
        {"type": "text", "text": "What color is this pixel?"},
        {
          "type": "image_url",
          "image_url": {
            "url": "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8DwHwAFBQIAX8jx0gAAAABJRU5ErkJggg==",
            "detail": "auto"
          }
        }
      ]
    }]
  }' | jq .
```

**Note:** The example above uses a 1x1 pixel blue image. For real images, you would encode your actual image file:

```bash
# Example: Encode a local image to base64
base64 -i /path/to/your/image.jpg | tr -d '\n' > image_base64.txt

# Then use it in your request
IMAGE_BASE64=$(cat image_base64.txt)
curl -X POST http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d "{
    \"model\": \"anthropic/claude-sonnet-5-5\",
    \"messages\": [{
      \"role\": \"user\",
      \"content\": [
        {\"type\": \"text\", \"text\": \"Describe this image in detail\"},
        {
          \"type\": \"image_url\",
          \"image_url\": {
            \"url\": \"data:image/jpeg;base64,$IMAGE_BASE64\",
            \"detail\": \"high\"
          }
        }
      ]
    }]
  }" | jq .
```

**Supported vision models:** vision support is looked up in the gateway's
community modalities table (`providers/core/community_modalities.json` and its
`.overrides.json` companion), not inferred from the model name. See
[Vision/Multimodal Support](../../README.md#visionmultimodal-support) for how
the lookup behaves.

**Image detail levels:**

- `auto` (default): Automatically choose the detail level
- `low`: Faster processing, lower detail
- `high`: More detailed analysis, slower processing

**Note:** The gateway never rejects a request for carrying images. With
`VISION_ENABLED=false` (the default) image parts are forwarded untouched and
the provider decides how to handle them. With `VISION_ENABLED=true` image parts
are stripped only for models the table lists as text-only - models it does not
cover pass through unchanged.
