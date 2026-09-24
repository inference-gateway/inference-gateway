# Pizza Demo TypeScript MCP Server

A demonstration MCP server built on the official
[TypeScript SDK v2](https://github.com/modelcontextprotocol/typescript-sdk)
(`@modelcontextprotocol/server`). It has a single tool that returns mock data
about the top 5 pizzas in the world.

`createMcpHandler` serves MCP `2026-07-28` - the stateless revision the
Inference Gateway speaks to its upstream servers - and 2025-era clients from the
same `/mcp` endpoint, with no session to keep. The factory builds a fresh
`McpServer` for every request.

## Tools

- **get_top_pizzas** - the top 5 pizzas in the world with their origin,
  description, year created and key ingredients

## Endpoints

| Endpoint  | Method | Description             |
| --------- | ------ | ----------------------- |
| `/mcp`    | POST   | MCP (Streamable HTTP)   |
| `/health` | GET    | Liveness, answers `200` |

## Usage

```bash
npm install
npm run dev     # run from source with tsx
npm run build   # compile to dist/
npm start       # run the build
```

The server listens on port `8084` (override with `PORT`). `createMcpExpressApp`
rejects requests whose `Host` isn't `mcp-pizza-server` (Compose),
`pizza-service.inference-gateway.svc.cluster.local` (the Service the Kubernetes
example's `MCP` resource gets) or `localhost`, which
protects it from DNS rebinding; add a host to `allowedHosts` in `src/index.ts`
to reach it under another name.

Try it directly:

```bash
curl -X POST http://localhost:8084/mcp \
  -H "Content-Type: application/json" \
  -H "Accept: application/json, text/event-stream" \
  -H "MCP-Protocol-Version: 2026-07-28" \
  -H "Mcp-Method: tools/list" \
  -d '{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{"_meta":{"io.modelcontextprotocol/protocolVersion":"2026-07-28","io.modelcontextprotocol/clientInfo":{"name":"curl","version":"1.0"},"io.modelcontextprotocol/clientCapabilities":{}}}}'
```

## With the Inference Gateway

```bash
MCP_SERVERS=pizza=http://mcp-pizza-server:8084/mcp
```

The gateway lists the tool as `mcp_pizza_get_top_pizzas`.
