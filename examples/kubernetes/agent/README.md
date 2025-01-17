# Building and Running a Kubernetes AI Agent

In this example we will deploy an AI agent onto a local Kubernetes cluster. The agent will tell us what's wrong based on the logs of the cluster, and probably suggest a solution or a fix to the issue. This is just an example, don't use it in production.

1. Let's first create a local kubernetes cluster:

```bash
ctlptl apply -f Cluster.yaml
```

2. Configure Groq Cloud as a provider with an API token and deploy the Inference Gateway:

```bash
kubectl apply -f inference-gateway/namespace.yaml
kubectl apply -f inference-gateway/secret.yaml
kubectl apply -f inference-gateway/serviceaccount.yaml
kubectl apply -f inference-gateway/
kubectl -n inference-gateway rollout status deployment/inference-gateway
```

1. Build the Logs Analyzer AI agent:

```bash
cd logs-analyzer
docker build -t localhost:5000/dummyrepo/logs-analyzer:latest .
docker push localhost:5000/dummyrepo/logs-analyzer:latest
```

4. Deploy the logs Analyzer AI agent:

```bash
cd ..
kubectl apply -f logs-analyzer/namespace.yaml
kubectl apply -f logs-analyzer/clusterrole.yaml
kubectl apply -f logs-analyzer/clusterrolebinding.yaml
kubectl apply -f logs-analyzer/serviceaccount.yaml
kubectl apply -f logs-analyzer/deployment.yaml
kubectl -n logs-analyzer rollout status deployment/logs-analyzer
```

5. Produce an error in the cluster, for example let's deploy a pod that will fail:

```bash
kubectl apply -f failing-deployment/deployment.yaml
```

6. Inspect the logs of the analyzer:

```bash
kubectl -n logs-analyzer logs -f deployment/logs-analyzer --all-containers
```

The agent should tell you what's wrong with the cluster and suggest a fix.

Wait for a few minutes and you should see something like this:

```md
Analysis result:
**Error Summary:** Nginx failed to start in Pod "fail" (Namespace: default) due to low memory configurations.

**Root Cause:** Insufficient memory allocation.

**Potential Solutions:**

1. Increase memory limit (e.g., `kubectl edit deploy -n default`).
2. Optimize nginx configuration (e.g., reduce worker processes).
3. Scale up resources (e.g., add more nodes to the cluster).

**Recommendations:** Monitor memory usage and adjust configurations accordingly to prevent similar issues.
```

Not the most useful agent, but you get the idea.

I think it could've been better if the log message was more descriptive, but you can improve it and also the prompt by modifying the `logs-analyzer/main.go` file.

you can also try to give the LLM more context or implement Consensus by letting multiple LLMs analyze the same log and pick the best possible consistent answer to display to to the user in the logs.

That same solution you can also email to the user, or send it to a slack channel, or even create a Jira ticket with the solution and assign it to the user, there is a lot of things you can do with AI agent.

1. Finally let's cleanup the cluster:

```bash
ctlptl delete -f Cluster.yaml --cascade=true
```
