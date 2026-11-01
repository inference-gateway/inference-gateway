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
- **Certificates**: a `Secret` built from the files `certs.sh` issues. The script and the provider's
  nginx config are shared with [the docker-compose example](../../docker-compose/mtls/README.md),
  which documents the certificates themselves. Swap in cert-manager (see the
  [TLS example](../tls/README.md)) for anything beyond a demo.

Mutual TLS is newer than the published image, so the gateway is built from this checkout and
imported into the cluster.

## Prerequisites

- [Task](https://taskfile.dev/installation/)
- kubectl
- ctlptl and k3d (for cluster management)
- Docker and openssl

## Quick Start

1. Create the cluster and import the gateway image:

   ```bash
   task deploy-infrastructure
   ```

2. Issue the certificates and deploy everything:

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
