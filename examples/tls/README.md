# TLS

Two scenarios: terminating TLS in front of the gateway, and mutual TLS where the
gateway both demands a client certificate and presents one to a provider that
requires it.

- [TLS termination](kubernetes/README.md) - certificates from cert-manager
  terminated at the Gateway API (kubernetes flavour)
- [Mutual TLS](mtls/) - a client certificate required by the gateway, and one
  presented to the upstream provider ([docker-compose](mtls/docker-compose/README.md),
  [kubernetes](mtls/kubernetes/README.md) - the kubernetes one is the only
  example without the [operator](https://github.com/inference-gateway/operator),
  which has no client-CA field yet)
