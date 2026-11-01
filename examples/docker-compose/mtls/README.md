# Mutual TLS Example

Mutual TLS in both directions: clients must present a certificate to the gateway, and the gateway
presents one to a provider that demands it.

## Overview

- **inference-gateway**: serves HTTPS on 8443 and requires a client certificate signed by the
  example CA (`SERVER_TLS_CLIENT_CA_PATH`).
- **mock-provider**: nginx with `ssl_verify_client on`, standing in for a provider behind mutual
  TLS. The gateway reaches it with `CLIENT_TLS_CA_PATH` plus
  `CLIENT_TLS_CERT_PATH`/`CLIENT_TLS_KEY_PATH`.

The gateway is built from this checkout, since mutual TLS is newer than the published image.

## Prerequisites

- Docker and Docker Compose
- [Task](https://taskfile.dev/installation/)
- openssl

## Quick Start

```bash
task up
task test
task down
```

`task up` issues a throwaway CA, a server certificate and a client certificate into `certs/`
(`certs.sh`, skipped when they already exist), then builds and starts the stack.

## What `task test` shows

| Case                  | Expected                                                         |
| --------------------- | ---------------------------------------------------------------- |
| `with-client-cert`    | 200, answered by the provider behind mutual TLS                  |
| `without-client-cert` | the TLS handshake fails - the gateway never sees an HTTP request |
| `models`              | 200, the provider's model list fetched over mutual TLS           |

By hand:

```bash
curl --cacert certs/ca.crt --cert certs/client.crt --key certs/client.key \
  https://localhost:8443/v1/chat/completions \
  -H 'Content-Type: application/json' \
  -d '{"model":"openai/mock","messages":[{"role":"user","content":"hi"}]}'
```

## Configuration

| Variable                    | Role                                                                  |
| --------------------------- | --------------------------------------------------------------------- |
| `SERVER_TLS_CERT_PATH`      | Server certificate - serving HTTPS needs it and the key               |
| `SERVER_TLS_KEY_PATH`       | Its private key                                                       |
| `SERVER_TLS_CLIENT_CA_PATH` | CA that must have signed a client certificate, which becomes required |
| `CLIENT_TLS_CA_PATH`        | Extra CA trusted for provider calls, on top of the system roots       |
| `CLIENT_TLS_CERT_PATH`      | Certificate presented to a provider that requires mutual TLS          |
| `CLIENT_TLS_KEY_PATH`       | Its private key                                                       |

A certificate without its key, or a client CA without server TLS, is rejected at startup.

## Certificates

`certs.sh` issues one CA, one server certificate shared by the gateway and the provider, and one
client certificate used by both curl and the gateway.

Example-only shortcuts: a single self-signed CA, one certificate for two servers, and
world-readable keys. Use separate certificates and a real CA in production.

## Additional Resources

- [Kubernetes mutual TLS example](../../kubernetes/mtls/README.md)
- [Configuration Guide](../../../Configurations.md)
