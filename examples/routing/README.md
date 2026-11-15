# Routing

Gateway-native model pools: with `ROUTING_ENABLED=true` and a pool file at
`ROUTING_CONFIG_PATH`, a `/v1/chat/completions` request whose `model` names a
logical alias is resolved round-robin over the alias's deployments and reported
in the `X-Selected-Provider` / `X-Selected-Model` response headers. Explicit
`?provider=` and `provider/model` prefixes always bypass the pool. See the
[API endpoints section](../../README.md#api-endpoints) of the main README.

## Flavours

- [docker-compose](docker-compose/README.md) - the gateway with the example
  pool file `routing.yaml` mounted, rotating the `fast-chat` alias between
  Groq and OpenAI
