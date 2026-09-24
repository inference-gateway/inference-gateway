# Calculator Go MCP Server

A demonstration MCP server built on the official
[Go SDK](https://github.com/modelcontextprotocol/go-sdk). It has a single tool
with a typed input and output, so it shows the parts of a tool result the
Inference Gateway passes through untouched:

- `structuredContent`, matching the `outputSchema` the SDK derives from the
  output struct
- `isError`, set when the tool fails (e.g. division by zero)

The Streamable HTTP handler runs with `Stateless: true`: the Go SDK serves MCP
`2026-07-28` - the revision the gateway speaks to its upstream servers - only
in stateless mode.

## Tools

- **calculate** - `a` and `b` combined by `operation` (`add`, `subtract`,
  `multiply` or `divide`); returns `{"result": <number>}`

## Endpoints

| Endpoint  | Method | Description             |
| --------- | ------ | ----------------------- |
| `/mcp`    | POST   | MCP (Streamable HTTP)   |
| `/health` | GET    | Liveness, answers `200` |

## Usage

```bash
go run .
```

The server listens on port `8085`. With the gateway:

```bash
MCP_SERVERS=calculator=http://mcp-calculator-server:8085/mcp
```

The gateway lists the tool as `mcp_calculator_calculate`.
