# Configure TLS for Inference Gateway with cert-manager

This example demonstrates TLS termination at the ingress controller level, which is the recommended approach for most Kubernetes deployments for several reasons:

### Ingress TLS Termination (Current Implementation)

- **Performance Optimization**: TLS termination at the ingress layer reduces CPU overhead on your application pods, as encryption/decryption happens once at the edge of your cluster.
- **Simplified Certificate Management**: Certificates are managed in one place (ingress), rather than across multiple services.
- **Kubernetes-Native Traffic Inspection**: Enables easier monitoring, logging, and troubleshooting of internal traffic.
- **Internal Network Security**: Communication within a Kubernetes cluster is typically considered secure based on network policies and pod security contexts.

### End-to-End Encryption (Alternative Approach)

If your security requirements demand encryption for internal service-to-service communication, you can implement end-to-end encryption. This would require:

1. Creating separate certificates for each service with cert-manager
2. Mounting these certificates to the Inference Gateway pods
3. Configuring the Inference Gateway to use HTTPS for connections to backend LLM services
4. Setting appropriate trust chains between services

However, this approach comes with trade-offs:

- **Increased Resource Usage**: Double TLS handshakes (client→gateway and gateway→LLM) increase CPU consumption and latency
- **Higher Complexity**: Certificate management across services adds operational overhead
- **Troubleshooting Challenges**: Encrypted internal traffic is harder to inspect for debugging

For most deployments, TLS termination at the ingress controller provides sufficient security while maximizing performance and simplicity. Consider end-to-end encryption only for highly sensitive environments with specific compliance requirements.

## Prerequisites

- docker
- ctlptl - CLI for declaratively setting up local Kubernetes clusters
- k3d - Lightweight Kubernetes distribution
- helm - Package manager for Kubernetes
- kubectl
- curl (for testing HTTPS connections)

## Components

This setup includes:

- Inference Gateway - The main application that proxies LLM requests
- cert-manager - Kubernetes add-on for certificate management
- Ingress Controller - For routing external traffic to the Inference Gateway

## Implementation Steps

1. Create the local cluster with port mapping for HTTPS:

```bash
ctlptl apply -f Cluster.yaml
```

2. Install the NGINX Ingress Controller:

```bash
helm repo add ingress-nginx https://kubernetes.github.io/ingress-nginx
helm repo update
helm install ingress-nginx ingress-nginx/ingress-nginx \
  --create-namespace \
  --namespace ingress-nginx \
  --version 4.12.0 \
  --set controller.publishService.enabled=true \
  --wait
```

3. Install cert-manager:

```bash
helm repo add jetstack https://charts.jetstack.io
helm repo update
helm install cert-manager jetstack/cert-manager \
  --namespace cert-manager \
  --create-namespace \
  --version v1.17.1 \
  --set installCRDs=true \
  --wait
```

4. Create a self-signed ClusterIssuer (for development):

```bash
kubectl apply -f cert-manager/clusterissuer.yaml
```

5. Deploy the Inference Gateway:

```bash
kubectl create namespace inference-gateway --dry-run=client -o yaml | kubectl apply --server-side=true -f -
kubectl apply -f inference-gateway/
```

6. Update your local `/etc/hosts` file to map api.inference-gateway.local to `127.0.0.1`:

```bash
echo "127.0.0.1 api.inference-gateway.local" >> /etc/hosts
```

\*\* Note: if you're using the devcontainer (which is also recommended) - you already have this entry in the /etc/hosts file.

7. Test the Inference Gateway with HTTPS:

```bash
curl -X GET -k https://api.inference-gateway.local/v1/models
```

## Cleanup

```bash
ctlptl delete -f Cluster.yaml --cascade=true
```
