# Mutual TLS Example

Mutual TLS in both directions: workloads must present a certificate to the gateway, and the gateway
presents one to a provider that demands it.

## Table of Contents

- [Mutual TLS Example](#mutual-tls-example)
  - [Table of Contents](#table-of-contents)
  - [Architecture](#architecture)
  - [Prerequisites](#prerequisites)
  - [Quick Start](#quick-start)
  - [Configuration](#configuration)
  - [Cleanup](#cleanup)

## Architecture

- **inference-gateway**: serves HTTPS on 8443 and requires a client certificate signed by the
  example CA (`SERVER_TLS_CLIENT_CA_PATH`). A plain `Deployment`, not a `Gateway` custom resource:
  the operator's `spec.server.tls` has no client CA field yet
  ([operator#310](https://github.com/inference-gateway/operator/issues/310)).
- **mock-provider**: nginx with `ssl_verify_client on`, standing in for a provider behind mutual
  TLS. The gateway reaches it with `CLIENT_TLS_CA_PATH` plus
  `CLIENT_TLS_CERT_PATH`/`CLIENT_TLS_KEY_PATH`.
- **client**: a curl pod with the same certificates mounted, so the example can call the gateway
  the way a workload in the cluster would.
- **Certificates**: issued by [cert-manager](https://cert-manager.io). A self-signed
  `ClusterIssuer` bootstraps the example CA (`cert-manager/issuers.yaml`), a namespaced `Issuer`
  signs from it, and three `Certificate` resources (`cert-manager/certificates.yaml`) produce the
  gateway's server certificate, the gateway's client certificate for provider calls, and the mock
  provider's server certificate. Each `Secret` holds `tls.crt`, `tls.key` and `ca.crt`, projected
  into the pods under the file names the gateway environment and the nginx config use. In
  production, replace the self-signed CA with your own PKI or an ACME issuer - nothing else about
  the example changes.
- The provider's nginx config is shared with
  [the docker-compose example](../../docker-compose/mtls/README.md), which issues the same
  certificates with openssl and documents what each one is for.

The gateway's server certificate carries an `IP:0.0.0.0` SAN. A provider call loops back through the
gateway's own `/proxy/{provider}` endpoint at `SERVER_HOST`, so the gateway verifies its own
certificate for the address it binds. Certificates are read once at startup, so `renewBefore`
renewals only take effect on a new pod - restart the deployment, or mount them through a sidecar
that reloads.

Mutual TLS is newer than the published image, so the gateway is built from this checkout and
imported into the cluster.

## Prerequisites

- [Task](https://taskfile.dev/installation/)
- kubectl
- ctlptl and k3d (for cluster management)
- Helm (cert-manager is installed from its chart)
- Docker

## Quick Start

1. Create the cluster, install cert-manager and import the gateway image:

   ```bash
   task deploy-infrastructure
   ```

2. Issue the certificates with cert-manager and deploy everything:

   ```bash
   task deploy-inference-gateway
   ```

3. Run the cases from the client pod:

   ```bash
   task test
   ```

   | Case                  | Expected                                                         |
   | --------------------- | ---------------------------------------------------------------- |
   | with a client cert    | 200, answered by the provider behind mutual TLS                  |
   | without a client cert | the TLS handshake fails - the gateway never sees an HTTP request |
   | models                | 200, the provider's model list fetched over mutual TLS           |

By hand:

```bash
kubectl -n inference-gateway exec deployment/client -- \
  curl -s --cacert /certs/ca.crt --cert /certs/client.crt --key /certs/client.key \
  https://inference-gateway:8443/v1/models
```

## Configuration

The gateway environment lives in `gateway.yaml`. Every variable is documented in the
[docker-compose example](../../docker-compose/mtls/README.md#configuration) and in the
[Configuration Guide](../../../Configurations.md).

## Cleanup

```bash
task clean
```
