# Example REST Endpoints

Assuming you've deployed the Inference Gateway, you can interact with the language models through the REST endpoints. Below are some examples of how to interact with the Inference Gateway using curl commands.

### GET Endpoints

| Description            | Curl Command                                                      |
| ---------------------- | ----------------------------------------------------------------- |
| List all models        | `curl -X GET http://localhost:8080/v1/models`                     |
| List Ollama models     | `curl -X GET http://localhost:8080/v1/models?provider=ollama`     |
| List Groq models       | `curl -X GET http://localhost:8080/v1/models?provider=groq`       |
| List OpenAI models     | `curl -X GET http://localhost:8080/v1/models?provider=openai`     |
| List Cloudflare models | `curl -X GET http://localhost:8080/v1/models?provider=cloudflare` |
| List Cohere models     | `curl -X GET http://localhost:8080/v1/models?provider=cohere`     |
| List Anthropic models  | `curl -X GET http://localhost:8080/v1/models?provider=anthropic`  |
| List DeepSeek models   | `curl -X GET http://localhost:8080/v1/models?provider=deepseek`   |

### POST Endpoints

| Domain             | Curl Command                                                                                                                                                                                                               |
| ------------------ | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| ollama.local       | `curl -X POST http://localhost:8080/v1/chat/completions -d '{"model":"ollama/deepseek-r1:1.5b","messages":[{"role":"system","content":"You are a helpful assistant."},{"role":"user","content":"Hi"}]}'`                   |
| api.groq.com       | `curl -X POST http://localhost:8080/v1/chat/completions -d '{"model":"groq/llama-3.3-70b-versatile","messages":[{"role":"system","content":"You are a helpful assistant."},{"role":"user","content":"Hi"}]}'`              |
| api.openai.com     | `curl -X POST http://localhost:8080/v1/chat/completions -d '{"model":"openai/gpt-4o-mini","messages":[{"role":"system","content":"You are a helpful assistant."},{"role":"user","content":"Hi"}]}'`                        |
| api.cloudflare.com | `curl -X POST http://localhost:8080/v1/chat/completions -d '{"model":"cloudflare/@cf/meta/llama-3.1-8b-instruct","messages":[{"role":"system","content":"You are a helpful assistant."},{"role":"user","content":"Hi"}]}'` |
| api.cohere.com     | `curl -X POST http://localhost:8080/v1/chat/completions -d '{"model":"cohere/command-r","messages":[{"role":"system","content":"You are a helpful assistant."},{"role":"user","content":"Hi"}]}'`                          |
| api.anthropic.com  | `curl -X POST http://localhost:8080/v1/chat/completions -d '{"model":"anthropic/claude-3-opus-20240229","messages":[{"role":"system","content":"You are a helpful assistant."},{"role":"user","content":"Hi"}]}'`          |
| api.deepseek.com   | `curl -X POST http://localhost:8080/v1/chat/completions -d '{"model":"deepseek/deepseek-reasoner","messages":[{"role":"system","content":"You are a helpful assistant."},{"role":"user","content":"Hi"}]}'`                |

You can set the stream as an optional flag in the request body to enable streaming of tokens. The default value is `false`.

```bash
curl -X POST http://localhost:8080/v1/chat/completions?provider=ollama -d '{
  "model": "deepseek-r1:1.5b",
  "messages": [
    {
      "role": "system",
      "content": "You are a helpful assistant."
    },
    {
      "role": "user",
      "content": "Hi, how are you doing today?"
    }
  ],
  "max_tokens": 40
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

### Tool Calls

You can provide tools that the LLM can use to perform specific functions. Here are some examples:

```bash
curl -X POST http://localhost:8080/v1/chat/completions?provider=groq -d '{
  "model": "deepseek-r1-distill-llama-70b",
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
curl -X POST http://localhost:8080/v1/chat/completions?provider=groq -d '{
  "model": "deepseek-r1-distill-llama-70b",
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
