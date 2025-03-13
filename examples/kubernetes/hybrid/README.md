# Hybrid Kubernetes Example

In this example, we will deploy both Ollama and the Inference Gateway onto a local Kubernetes cluster. The Inference Gateway will facilitate sending requests to Ollama as well as to various cloud provider's Large Language Models (LLMs).

1. Create the local cluster:

```bash
task cluster-create
```

2. Deploy Ollama onto Kubernetes:

```bash
task deploy-ollama
```

3. Wait for the Ollama deployment to be completely rolled out and ready(could take a while due to 2GB download, roughly 3min on a standard internet bandwidth):

```bash
kubectl -n ollama logs -f deployment/ollama
```

Inspect the logs and see if the pull of the model is completed: `pull success`.

If you see in the logs `msg="downloading..."` it means the model is still being downloaded.

4. Deploy the Inference Gateway onto Kubernetes:

```bash
task deploy-inference-gateway
```

5. Proxy the Inference Gateway, to access it locally:

```bash
task proxy
```

6. Check the available Ollama local LLMs:

```bash
curl -X GET http://localhost:8080/v1/models?provider=ollama | jq '.'
```

7. Send a request to the Inference Gateway:

```bash
curl -X POST http://localhost:8080/v1/chat/completions?provider=ollama -d '{"model": "phi3:3.8b", "prompt": "Explain the importance of fast language models. Keep it short and concise."}' | jq .
```

8. Add a cloud provider's LLM to the Inference Gateway, by setting the API Key in the Kubernetes [secret](inference-gateway/secret.yaml):

```bash
...
  GROQ_API_KEY=<GROQ_API_KEY>
...
```

9. Deploy the new secret:

```bash
kubectl apply -f inference-gateway/secret.yaml
```

10. Restart the Inference Gateway so the changes take effect:

```bash
kubectl -n inference-gateway rollout restart deployment/inference-gateway
kubectl -n inference-gateway rollout status deployment/inference-gateway
```

11. Proxy the Inference Gateway, to access it locally:

```bash
task proxy
```

12. Send a similar request to the Inference Gateway, but this time using the cloud provider's LLM:

```bash
curl -X POST http://localhost:8080/v1/chat/completions?provider=groq -d '{"model": "llama-3.3-70b-versatile", "messages": [{"role": "user", "content": "Explain the importance of fast language models. Keep it short and concise."}]}' | jq .
```

And that's how you can interact with both local and cloud provider's Large Language Models using the Inference Gateway. All from a similar interface.

\*\* You can refer to the [Taskfile.yaml](./Taskfile.yaml) at any point for detailed information about the tasks used in this example.

\*\* If the response is cut off mid-stream while the token is still being transmitted, it may be caused by the inference gateway's read timeout. Consider increasing or adjusting the timeout value and redeploying the gateway.
