# Example REST Endpoints

Assuming you've deployed the Inference Gateway, you can interact with the language models through the REST endpoints. Below are some examples of how to interact with the Inference Gateway using curl commands.

### GET Endpoints

| Description            | Curl Command                                                   |
| ---------------------- | -------------------------------------------------------------- |
| List all models        | `curl -X GET http://localhost:8080/llms`                       |
| List Ollama models     | `curl -X GET http://localhost:8080/llms/ollama/v1/models`      |
| List Groq models       | `curl -X GET http://localhost:8080/llms/groq/openai/v1/models` |
| List OpenAI models     | `curl -X GET http://localhost:8080/llms/openai/v1/models`      |
| List Google models     | `curl -X GET http://localhost:8080/llms/google/v1beta/models`  |
| List Cloudflare models | `curl -X GET http://localhost:8080/llms/cloudflare/ai/models`  |

### POST Endpoints

| Domain                            | Curl Command                                                                                                                                                                                                                                 |
| --------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| ollama.local                      | `curl -X POST http://localhost:8080/llms/ollama/api/generate -d '{"model": "phi3:3.8b", "prompt": "Why is the sky blue? keep it short and concise."}'`                                                                                       |
| api.groq.com                      | `curl -X POST http://localhost:8080/llms/groq/openai/v1/chat/completions -d '{"model": "llama-3.3-70b-versatile", "messages": [{"role": "user", "content": "Explain the importance of fast language models. Keep it short and concise."}]}'` |
| generativelanguage.googleapis.com | `curl -X POST http://localhost:8080/llms/google/v1beta/models/gemini-1.5-flash:generateContent -d '{"contents": [{"parts":[{"text": "Explain how AI works. Keep it short and concise."}]}]}'`                                                |
| api.openai.com                    | `curl -X POST http://localhost:8080/llms/openai/v1/models/davinci/completions -d '{"prompt": "Once upon a time", "max_tokens": 100'`                                                                                                         |
| api.cloudflare.com                | `curl -X POST http://localhost:8080/llms/cloudflare/ai/run/@cf/meta/llama-3.1-8b-instruct  -d '{ "prompt": "Where did the phrase Hello World come from. Keep it short and concise." }'`                                                      |
