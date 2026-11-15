# MCP

Model Context Protocol tool servers behind the gateway: set `MCP_SERVERS` and the
gateway discovers each server's tools and offers them to the LLMs. The example
server sources (time, calculator, filesystem, search and the pizza server) live
inside the docker-compose flavour and are reused by the kubernetes flavour.

What the gateway does with tools a client declares itself is documented in the
[tools example](../tools/README.md).

## Flavours

- [docker-compose](docker-compose/README.md) - the gateway next to the example
  MCP servers, including the pizza server on the TypeScript SDK
- [kubernetes](kubernetes/README.md) - the same server images through the
  [Inference Gateway Operator](https://github.com/inference-gateway/operator),
  plus an MCP inspector config
