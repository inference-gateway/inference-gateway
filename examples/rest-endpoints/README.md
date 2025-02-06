# Example REST Endpoints

Assuming you've deployed the Inference Gateway, you can interact with the language models through the REST endpoints. Below are some examples of how to interact with the Inference Gateway using curl commands.

### GET Endpoints

| Description            | Curl Command                                        |
| ---------------------- | --------------------------------------------------- |
| List all models        | `curl -X GET http://localhost:8080/llms`            |
| List Ollama models     | `curl -X GET http://localhost:8080/llms/ollama`     |
| List Groq models       | `curl -X GET http://localhost:8080/llms/groq`       |
| List OpenAI models     | `curl -X GET http://localhost:8080/llms/openai`     |
| List Cloudflare models | `curl -X GET http://localhost:8080/llms/cloudflare` |
| List Cohere models     | `curl -X GET http://localhost:8080/llms/cohere`     |
| List Anthropic models  | `curl -X GET http://localhost:8080/llms/anthropic`  |

### POST Endpoints

| Domain             | Curl Command                                                                                                                                                                                                                                             |
| ------------------ | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | --- |
| ollama.local       | `curl -X POST http://localhost:8080/llms/ollama/generate -d '{"model":"phi3:3.8b","messages":[{"role":"system","content":"You are a helpful assistant."},{"role":"user","content":"Why is the sky blue? keep it short and concise."}]}'`                 |
| api.groq.com       | `curl -X POST http://localhost:8080/llms/groq/generate -d '{"model":"llama-3.3-70b-versatile","messages":[{"role":"system","content":"You are a helpful assistant."},{"role":"user","content":"Why is the sky blue? keep it short and concise."}]}'`     |     |
| api.openai.com     | `curl -X POST http://localhost:8080/llms/openai/generate -d '{"model":"gpt-4o-mini","messages":[{"role":"system","content":"You are a helpful assistant."},{"role":"user","content":"Why is the sky blue? keep it short and concise."}]}'`               |
| api.cloudflare.com | `curl -X POST http://localhost:8080/llms/cloudflare/generate -d '{"model":"llama-3.1-8b-instruct","messages":[{"role":"system","content":"You are a helpful assistant."},{"role":"user","content":"Why is the sky blue? keep it short and concise."}]}'` |
| api.cohere.com     | `curl -X POST http://localhost:8080/llms/cohere/generate -d '{"model":"command-r","messages":[{"role":"system","content":"You are a helpful assistant."},{"role":"user","content":"Why is the sky blue? keep it short and concise."}]}'`                 |
| api.anthropic.com  | `curl -X POST http://localhost:8080/llms/anthropic/generate -d '{"model":"claude-3-opus-20240229","messages":[{"role":"system","content":"You are a helpful assistant."},{"role":"user","content":"Why is the sky blue? keep it short and concise."}]}'` |

You can set the stream as an optional flag in the request body to enable streaming of tokens. The default value is `false`.

```bash
curl -X POST http://localhost:8080/llms/ollama/generate -d '{
  "model": "phi3:3.8b",
  "messages": [
    {
      "role": "system",
      "content": "You are a helpful assistant."
    },
    {
      "role": "user",
      "content": "Why is the sky blue? keep it short and concise."
    }
  ],
  "stream": true,
  "ssevents": true
}' | jq .
```

### Tool Calls

You can provide tools that the LLM can use to perform specific functions. Here are some examples:

```bash
curl -X POST http://localhost:8080/llms/ollama/generate -d '{
  "model": "llama3.2:1b",
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
