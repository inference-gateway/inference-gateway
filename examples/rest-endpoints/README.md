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
curl -X POST http://localhost:8080/llms/groq/generate -d '{
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
  "provider": "Groq",
  "response": {
    "content": "Okay, the user is asking about the current weather in Toronto. I need to figure out how to respond. \n\nFirst, I should use the function provided in the tools. The function is called get_current_weather, and it requires a city parameter.\n\nSo, I'll call this function with Toronto as the city. I'll format the tool_call with the function name and the arguments as a JSON object.\n\nI'll make sure the JSON is correctly formatted, using double quotes around the strings. \n\nFinally, I'll enclose everything within the <tool_call> tags as specified. \n\nThat should give the user the weather information they're looking for.\n",
    "model": "deepseek-r1-distill-llama-70b",
    "role": "assistant",
    "tool_calls": [
      {
        "id": "call_8k4k",
        "type": "function",
        "function": {
          "name": "get_current_weather",
          "arguments": "{\"city\": \"Toronto\"}"
        }
      }
    ]
  }
}
```

So you can response with the function call content as follow:

```bash
curl -X POST http://localhost:8080/llms/groq/generate -d '{
  "model": "deepseek-r1-distill-llama-70b",
  "messages": [
    {"role": "system", "content": "You are a helpful assistant."},
    {"role": "user", "content": "What is the current weather in Toronto?"},
    {"role": "assistant", "content": "Okay, the user is asking about the current weather in Toronto. I need to figure out how to respond. \n\nFirst, I should use the function provided in the tools. The function is called get_current_weather, and it requires a city parameter.\n\nSo, I will call this function with Toronto as the city. I will format the tool_call with the function name and the arguments as a JSON object.\n\nI will make sure the JSON is correctly formatted, using double quotes around the strings. \n\nFinally, I will enclose everything within the <tool_call> tags as specified. \n\nThat should give the user the weather information they are looking for.\n"},
    {"role": "tool", "content": "89F", "tool_call_id": "call_8k4k"}
  ]
}' | jq .
```

Then the LLM will respond with the weather information as follow:

```json
{
  "provider": "Groq",
  "response": {
    "content": "<think>\nAlright, let's break down what happened here. The user asked about the current weather in Toronto. I decided to use the get_current_weather function, passing \"Toronto\" as the argument.\n\nI set up the tool_call correctly with the function name and the city in a JSON object. Then, I sent that off to get the weather data. The response came back with \"89F\" as the temperature.\n\nSo, now I need to explain this to the user in a helpful way. They wanted the temperature, and it's 89 degrees Fahrenheit. That's pretty warm, maybe even hot, depending on the season.\n\nI should convert that to Celsius for better understanding since Toronto might use that scale. 89F converts to about 31-32°C. That gives a clearer picture of how warm it is.\n\nI also want to make sure the user feels comfortable asking for more details if they need anything else. So, I'll end with a friendly note offering further assistance.\n</think>\n\nThe current temperature in Toronto is **89°F**. If you'd like more details about the weather, feel free to ask!",
    "model": "deepseek-r1-distill-llama-70b",
    "role": "assistant"
  },
  "event_type": "content-delta"
}
```

Then you would append it to the conversation and so goes on.
