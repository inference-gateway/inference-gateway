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

7. Cleanup:

```bash
ctlptl delete -f Cluster.yaml --cascade=true
```
