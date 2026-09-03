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

### Context Windows

Pass `include=context_window` to enrich each model with its effective context
window. The value is resolved from the serving runtime when possible (llama.cpp
`/props`, Ollama `/api/show`), falling back to the window the provider publishes
in its model listing; models the gateway cannot resolve carry an explicit
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
      "id": "mistral/mistral-large",
      "object": "model",
      "created": 1750000000,
      "owned_by": "mistralai",
      "served_by": "mistral",
      "context_window": { "tokens": 32768, "source": "provider" }
    },
    {
      "id": "openai/gpt-4o",
      "object": "model",
      "created": 1750000000,
      "owned_by": "openai",
      "served_by": "openai",
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
listing. Monetary values are decimal strings to avoid floating-point precision
loss. Rates the provider does not publish (e.g. zero cache rates) are omitted
entirely; models whose provider publishes no per-token pricing carry an explicit
`null`.

```bash
curl -X GET 'http://localhost:8080/v1/models?include=pricing' | jq .
```

Response:

```json
{
  "object": "list",
  "data": [
    {
      "id": "deepseek/deepseek-chat",
      "object": "model",
      "created": 1750000000,
      "owned_by": "deepseek",
      "served_by": "deepseek",
      "pricing": {
        "currency": "USD",
        "input_per_token": "0.00000027",
        "output_per_token": "0.00000110",
        "cache_read_per_token": "0.00000007",
        "cache_write_per_token": "0.00000027",
        "source": "provider"
      }
    },
    {
      "id": "groq/llama-3.3-70b",
      "object": "model",
      "created": 1750000000,
      "owned_by": "meta",
      "served_by": "groq",
      "pricing": {
        "currency": "USD",
        "input_per_token": "0.00000059",
        "output_per_token": "0.00000079",
        "source": "provider"
      }
    },
    {
      "id": "openai/gpt-4o",
      "object": "model",
      "created": 1750000000,
      "owned_by": "openai",
      "served_by": "openai",
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

Pass `include=modalities` to enrich each model with the input modalities it
supports natively, one or more of `text`, `image`, `audio`, and `video`. The
values are sourced from the community-maintained
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
      "id": "openai/gpt-4o",
      "object": "model",
      "created": 1750000000,
      "owned_by": "openai",
      "served_by": "openai",
      "modalities": ["text", "image"]
    },
    {
      "id": "deepseek/deepseek-chat",
      "object": "model",
      "created": 1750000000,
      "owned_by": "deepseek",
      "served_by": "deepseek",
      "modalities": ["text"]
    },
    {
      "id": "ollama/deepseek-r1:1.5b",
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

| Domain                            | Curl Command                                                                                                                                                                                                               |
| --------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| ollama.local                      | `curl -X POST http://localhost:8080/v1/chat/completions -d '{"model":"ollama/deepseek-r1:1.5b","messages":[{"role":"system","content":"You are a helpful assistant."},{"role":"user","content":"Hi"}]}'`                   |
| ollama.com                        | `curl -X POST http://localhost:8080/v1/chat/completions -d '{"model":"ollama_cloud/gpt-oss:20b","messages":[{"role":"system","content":"You are a helpful assistant."},{"role":"user","content":"Hi"}]}'`                  |
| api.groq.com                      | `curl -X POST http://localhost:8080/v1/chat/completions -d '{"model":"groq/llama-3.3-70b-versatile","messages":[{"role":"system","content":"You are a helpful assistant."},{"role":"user","content":"Hi"}]}'`              |
| api.openai.com                    | `curl -X POST http://localhost:8080/v1/chat/completions -d '{"model":"openai/gpt-4o-mini","messages":[{"role":"system","content":"You are a helpful assistant."},{"role":"user","content":"Hi"}]}'`                        |
| api.cloudflare.com                | `curl -X POST http://localhost:8080/v1/chat/completions -d '{"model":"cloudflare/@cf/meta/llama-3.1-8b-instruct","messages":[{"role":"system","content":"You are a helpful assistant."},{"role":"user","content":"Hi"}]}'` |
| api.cohere.com                    | `curl -X POST http://localhost:8080/v1/chat/completions -d '{"model":"cohere/command-r","messages":[{"role":"system","content":"You are a helpful assistant."},{"role":"user","content":"Hi"}]}'`                          |
| api.anthropic.com                 | `curl -X POST http://localhost:8080/v1/chat/completions -d '{"model":"anthropic/claude-3-opus-20240229","messages":[{"role":"system","content":"You are a helpful assistant."},{"role":"user","content":"Hi"}]}'`          |
| api.deepseek.com                  | `curl -X POST http://localhost:8080/v1/chat/completions -d '{"model":"deepseek/deepseek-v4-flash","messages":[{"role":"system","content":"You are a helpful assistant."},{"role":"user","content":"Hi"}]}'`                |
| generativelanguage.googleapis.com | `curl -X POST http://localhost:8080/v1/chat/completions -d '{"model":"google/models/gemini-2.5-flash-lite","messages":[{"role":"system","content":"You are a helpful assistant."},{"role":"user","content":"Hi"}]}'`       |
| api.mistral.ai                    | `curl -X POST http://localhost:8080/v1/chat/completions -d '{"model":"mistral/pixtral-large-latest","messages":[{"role":"system","content":"You are a helpful assistant."},{"role":"user","content":"Hi"}]}'`              |
| api.minimax.io                    | `curl -X POST http://localhost:8080/v1/chat/completions -d '{"model":"minimax/MiniMax-M3","messages":[{"role":"system","content":"You are a helpful assistant."},{"role":"user","content":"Hi"}]}'`                        |
| api.moonshot.ai                   | `curl -X POST http://localhost:8080/v1/chat/completions -d '{"model":"moonshot/moonshot-v1-8k","messages":[{"role":"system","content":"You are a helpful assistant."},{"role":"user","content":"Hi"}]}'`                   |
| integrate.api.nvidia.com          | `curl -X POST http://localhost:8080/v1/chat/completions -d '{"model":"nvidia/moonshotai/kimi-k2.6","messages":[{"role":"system","content":"You are a helpful assistant."},{"role":"user","content":"Hi"}]}'`               |
| llamacpp.local                    | `curl -X POST http://localhost:8080/v1/chat/completions -d '{"model":"llamacpp/Qwen2.5-0.5B-Instruct","messages":[{"role":"system","content":"You are a helpful assistant."},{"role":"user","content":"Hi"}]}'`            |

You can set the stream as an optional flag in the request body to enable
streaming of tokens. The default value is `false`.

```bash
curl -X POST http://localhost:8080/v1/chat/completions -d '{
  "model": "groq/deepseek-r1-distill-llama-70b",
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
  "id": "chatcmpl-753",
  "object": "chat.completion",
  "created": 1741879542,
  "model": "deepseek-r1:1.5b",
  "choices": [
    {
      "index": 0,
      "message": {
        "content": "<think>\nOkay, so the user greeted me and said \"Hi, how are you doing today?\" They're just starting to say that. I should respond in a friendly way.\n\nMaybe I can acknowledge their greeting and offer my help with something. Since they mentioned working on math problems or solving puzzles, I'll stick to that.\n\nI want to make sure I'm approaching it the right way. It's not a question yet, but if I see more of them, maybe I can offer more assistance. So responding with an emoji like 😊 would be nice.\n</think>\n\nHello! How are you doing today? 😊",
        "role": "assistant"
      },
      "finish_reason": "length"
    }
  ],
  "usage": {
    "prompt_tokens": 40,
    "completion_tokens": 40,
    "total_tokens": 80
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
  "model": "anthropic/claude-sonnet-4-5",
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
  "model": "claude-sonnet-4-5",
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
  "model": "anthropic/claude-sonnet-4-5",
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
  "model": "anthropic/claude-sonnet-4-5",
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
  "model": "openai/gpt-4o",
  "input": "Hi, how are you doing today?",
  "instructions": "You are a helpful assistant."
}' | jq .
```

Response:

```json
{
  "id": "resp_67b5cf1e3e3481928c7a3b2f1a2b3c4d",
  "object": "response",
  "created_at": 1741879542,
  "status": "completed",
  "model": "gpt-4o-2024-08-06",
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
  "model": "openai/gpt-4o",
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
CLIENT_TIMEOUT=600s
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

### Image Edits and Variations

Two additional Images endpoints accept `multipart/form-data` uploads and reuse
the same `ImagesResponse` shape, toggle (`IMAGES_ENABLED`), and provider support
rules as `/images/generations`:

- `POST /v1/images/edits` - edit or extend a source image. Requires the `image`
  file and a `prompt`; optional `mask`, `model`, `n` (1-10), `size`, `quality`,
  and `response_format` (`url` or `b64_json`).
- `POST /v1/images/variations` - create variations of a source image. Requires
  the `image` file; optional `model`, `n` (1-10), `size`, and `response_format`.

The gateway streams the uploaded `image` (and optional `mask`) straight through
to the provider without buffering the whole payload, so large images stay cheap.

```bash
# Edit an image
curl -X POST "http://localhost:8080/v1/images/edits?provider=openai" \
  -H "Authorization: Bearer $TOKEN" \
  -F image="@sunset.png" \
  -F mask="@mask.png" \
  -F prompt="Add a flock of birds to the sky" \
  -F model="gpt-image-1" \
  -F n=1 \
  -F size="1024x1024" \
  -F response_format="url"

# Create a variation of an image
curl -X POST "http://localhost:8080/v1/images/variations?provider=openai" \
  -H "Authorization: Bearer $TOKEN" \
  -F image="@sunset.png" \
  -F model="dall-e-2" \
  -F n=2 \
  -F size="1024x1024" \
  -F response_format="b64_json"
```

Both return an `ImagesResponse`:

```json
{
  "created": 1730000000,
  "data": [{ "url": "https://.../generated.png" }]
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

```bash
curl -X POST http://localhost:8080/v1/audio/speech -d '{
  "model": "local/qwen3-tts",
  "input": "Ahoy! Welcome aboard the Inference Gateway.",
  "voice": "alloy",
  "response_format": "wav"
}' -o speech.wav
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

## Tool Calls

You can provide tools that the LLM can use to perform specific functions. Here
are some examples:

```bash
curl -X POST http://localhost:8080/v1/chat/completions -d '{
  "model": "groq/deepseek-r1-distill-llama-70b",
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
        "reasoning": "Okay, the user is asking about the current weather in Toronto. I need to figure out how to respond using the tools provided. \n\nLooking at the tools section, there's a function called get_current_weather which takes a city name as a parameter. That seems perfect for this query.\n\nSo, I should call this function with \"Toronto\" as the city argument. I'll structure the response in the required XML format, making sure to use the correct tags and JSON structure inside.\n\nI should double-check the function name and parameters to ensure accuracy. The function name is get_current_weather, and the parameter is city as a string. So, the arguments JSON should have \"city\" set to \"Toronto\".\n\nPutting it all together, I'll create the XML tool_call with the function name and the arguments JSON. That should give the user the current weather in Toronto.\n",
        "role": "assistant",
        "tool_calls": [
          {
            "function": {
              "arguments": "{\"city\": \"Toronto\"}",
              "name": "get_current_weather"
            },
            "id": "call_0e2n",
            "type": "function"
          }
        ]
      }
    }
  ],
  "created": 1742314696,
  "id": "chatcmpl-f14e1a5f-0c05-4c94-be1c-6d23f0c1ecb6",
  "model": "deepseek-r1-distill-llama-70b",
  "object": "chat.completion",
  "usage": {
    "completion_tokens": 202,
    "prompt_tokens": 150,
    "total_tokens": 352
  }
}
```

So you can response with the function call content as follow:

```bash
curl -X POST http://localhost:8080/v1/chat/completions -d '{
  "model": "groq/deepseek-r1-distill-llama-70b",
  "messages": [
    {"role": "system", "content": "You are a helpful assistant."},
    {"role": "user", "content": "What is the current weather in Toronto?"},
    {"role": "assistant", "content": "Okay, the user is asking about the current weather in Toronto. I need to figure out how to respond. \n\nFirst, I should use the function provided in the tools. The function is called get_current_weather, and it requires a city parameter.\n\nSo, I will call this function with Toronto as the city. I will format the tool_call with the function name and the arguments as a JSON object.\n\nI will make sure the JSON is correctly formatted, using double quotes around the strings. \n\nFinally, I will enclose everything within the <tool_call> tags as specified. \n\nThat should give the user the weather information they are looking for.\n"},
    {"role": "tool", "content": "89F", "tool_call_id": "call_0e2n"}
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
        "content": "<think>\nAlright, so the user previously asked about the current weather in Toronto, and I responded by using the get_current_weather tool. Now, the user has provided a response that seems a bit unclear: \"89F\". I need to figure out what this means and how to proceed.\n\nFirst, I notice that the user's message includes some unusual characters: <｜tool▁outputs▁begin｜>89F<｜tool▁outputs▁end｜>. This looks like some sort of formatting or markup, possibly from a tool or system they're using. The \"89F\" part is likely the temperature they received, which is 89 degrees Fahrenheit. \n\nGiven that, it seems like they might be confirming the weather data or perhaps indicating that they received the information but want more details. Alternatively, they might be testing how I handle such inputs.\n\nI should consider that they might be interested in additional aspects of the weather, such as the conditions, humidity, wind speed, or perhaps the forecast. Since they provided a temperature, maybe they want to know if that's accurate or if there's more to the weather situation.\n\nI also need to make sure my response is helpful and provides value beyond just the temperature. Maybe I can ask if they need more details about the weather in Toronto or if they have specific aspects they're curious about.\n\nIt's important to keep the conversation flowing smoothly, so I should phrase my response in a friendly and open manner, encouraging them to specify what they need.\n</think>\n\nIt seems like you're referring to the current weather in Toronto, where the temperature is 89°F. If you'd like more details about the weather, such as conditions, humidity, or the forecast, feel free to ask!",
        "role": "assistant"
      }
    }
  ],
  "created": 1742314733,
  "id": "chatcmpl-12a36044-74b5-4fd7-973b-b31333d1118e",
  "model": "deepseek-r1-distill-llama-70b",
  "object": "chat.completion",
  "usage": {
    "completion_tokens": 353,
    "prompt_tokens": 173,
    "total_tokens": 526
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
  "model": "openai/gpt-4o",
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
    "model": "openai/gpt-4o-mini",
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
    \"model\": \"anthropic/claude-3-5-sonnet-20241022\",
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

**Supported vision models:**

- OpenAI: `gpt-4o`, `gpt-4-turbo`, `gpt-4-vision-preview`
- Anthropic: `claude-3-5-sonnet-*`, `claude-3-opus-*`, `claude-3-sonnet-*`
- Google: Models with `vision` or `multimodal` in the name
- Mistral: `pixtral-*` models
- Groq/DeepSeek/Cohere: Models with `vision` or `multimodal` in the name

**Image detail levels:**

- `auto` (default): Automatically choose the detail level
- `low`: Faster processing, lower detail
- `high`: More detailed analysis, slower processing

**Note:** Attempting to send image content to a non-vision model will result in
a `400 Bad Request` error with a clear message indicating that the model does
not support vision capabilities.
