# Example REST Endpoints

Assuming you've deployed the Inference Gateway, you can interact with the language models through the REST endpoints. Below are some examples of how to interact with the Inference Gateway using curl commands.

### GET Endpoints

| Description            | Curl Command                                        |
| ---------------------- | --------------------------------------------------- |
| List all models        | `curl -X GET http://localhost:8080/llms`            |
| List Ollama models     | `curl -X GET http://localhost:8080/llms/ollama`     |
| List Groq models       | `curl -X GET http://localhost:8080/llms/groq`       |
| List OpenAI models     | `curl -X GET http://localhost:8080/llms/openai`     |
| List Google models     | `curl -X GET http://localhost:8080/llms/google`     |
| List Cloudflare models | `curl -X GET http://localhost:8080/llms/cloudflare` |
| List Cohere models     | `curl -X GET http://localhost:8080/llms/cohere`     |
| List Anthropic models  | `curl -X GET http://localhost:8080/llms/anthropic`  |

### POST Endpoints

| Domain                            | Curl Command                                                                                                                                                                                                                                             |
| --------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| ollama.local                      | `curl -X POST http://localhost:8080/llms/ollama/generate -d '{"model":"phi3:3.8b","messages":[{"role":"system","content":"You are a helpful assistant."},{"role":"user","content":"Why is the sky blue? keep it short and concise."}]}'`                 |
| api.groq.com                      | `curl -X POST http://localhost:8080/llms/groq/generate -d '{"model":"llama-3.3-70b-versatile","messages":[{"role":"system","content":"You are a helpful assistant."},{"role":"user","content":"Why is the sky blue? keep it short and concise."}]}'`     |
| generativelanguage.googleapis.com | `curl -X POST http://localhost:8080/llms/google/generate -d '{"model":"gemini-1.5-flash","messages":[{"role":"user","content":"Why is the sky blue? keep it short and concise."}]}'`                                                                     |
| api.openai.com                    | `curl -X POST http://localhost:8080/llms/openai/generate -d '{"model":"gpt-4o-mini","messages":[{"role":"system","content":"You are a helpful assistant."},{"role":"user","content":"Why is the sky blue? keep it short and concise."}]}'`               |
| api.cloudflare.com                | `curl -X POST http://localhost:8080/llms/cloudflare/generate -d '{"model":"llama-3.1-8b-instruct","messages":[{"role":"system","content":"You are a helpful assistant."},{"role":"user","content":"Why is the sky blue? keep it short and concise."}]}'` |
| api.cohere.com                    | `curl -X POST http://localhost:8080/llms/cohere/generate -d '{"model":"command-r","messages":[{"role":"system","content":"You are a helpful assistant."},{"role":"user","content":"Why is the sky blue? keep it short and concise."}]}'`                 |
| api.anthropic.com                 | `curl -X POST http://localhost:8080/llms/anthropic/generate -d '{"model":"claude-3-opus-20240229","messages":[{"role":"system","content":"You are a helpful assistant."},{"role":"user","content":"Why is the sky blue? keep it short and concise."}]}'` |
