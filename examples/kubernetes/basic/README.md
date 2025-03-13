# Basic Kubernetes Example

In this basic example, we will deploy the Inference Gateway.

Feel free to explore the [ConfigMap](inference-gateway/configmap.yaml) and [Secret](inference-gateway/secret.yaml) configurations of the Inference Gateway to set up your desired providers.

1. Create the local cluster:

```bash
task cluster-create
```

2. Deploy the Inference Gateway onto Kubernetes:

```bash
task deploy
```

3. Proxy the Inference Gateway, to access it locally:

```bash
task proxy
```

4. Check the available LLMs:

```bash
curl -X GET http://localhost:8080/v1/models | jq .
```

1. Interact with the Inference Gateway using the specific provider API(note the prefix is `/v1/chat/completions/*`):

```bash
curl -X POST http://localhost:8080/v1/chat/completions?provider=groq -d '{"model": "llama-3.2-3b-preview", "messages": [{"role": "user", "content": "Explain the importance of fast language models. Keep it short and concise."}]}' | jq .
```

\*\* You can refer to the [Taskfile.yaml](./Taskfile.yaml) at any point for detailed information about the tasks used in this example.

\*\* If the response is cut off mid-stream while the token is still being transmitted, it may be caused by the inference gateway's read timeout. Consider increasing or adjusting the timeout value and redeploying the gateway.
