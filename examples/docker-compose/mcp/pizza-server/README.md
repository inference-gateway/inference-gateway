# Pizza Demo TypeScript MCP Server

This is a simplified demonstration MCP server built using the official [@modelcontextprotocol/sdk](https://github.com/modelcontextprotocol/typescript-sdk) TypeScript SDK. It showcases a single tool that returns mock data about the top 5 pizzas in the world.

## Features

- 🍕 Simple demonstration with pizza data
- 🚀 Built with official TypeScript MCP SDK
- 🔄 Dual transport support: Streamable HTTP + Legacy SSE
- 📡 Real-time session management
- 🏥 Health monitoring endpoints
- 🐳 Docker containerized

## Capabilities

### Tools

- **get-top-pizzas** - Returns mock data of the top 5 pizzas in the world with detailed information including origin, description, year created, and key ingredients

## Endpoints

| Endpoint    | Method          | Transport       | Description                                 |
| ----------- | --------------- | --------------- | ------------------------------------------- |
| `/mcp`      | GET/POST/DELETE | Streamable HTTP | Modern MCP endpoint with session management |
| `/sse`      | GET             | SSE             | Legacy SSE connection endpoint              |
| `/messages` | POST            | SSE             | Legacy SSE message handling                 |
| `/health`   | GET             | HTTP            | Health check and connection stats           |
| `/`         | GET             | HTTP            | Server info and capabilities                |

## Usage

### Development

```bash
# Install dependencies
npm install

# Run in development mode
npm run dev

# Build for production
npm run build

# Run production build
npm start
```

### Docker

```bash
# Build the image
docker build -t pizza-demo-mcp-server .

# Run the container
docker run -p 8084:8084 pizza-demo-mcp-server
```

### Testing with curl

#### Get server info

```bash
curl http://localhost:8084/
```

#### Health check

```bash
curl http://localhost:8084/health
```

#### Initialize MCP session (Streamable HTTP)

```bash
curl -X POST http://localhost:8084/mcp \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc": "2.0",
    "id": 1,
    "method": "initialize",
    "params": {
      "protocolVersion": "2025-03-26",
      "capabilities": {},
      "clientInfo": {
        "name": "test-client",
        "version": "1.0.0"
      }
    }
  }'
```

#### List tools

```bash
# After initialization, use the session ID from the response
curl -X POST http://localhost:8084/mcp \
  -H "Content-Type: application/json" \
  -H "mcp-session-id: YOUR_SESSION_ID" \
  -d '{
    "jsonrpc": "2.0",
    "id": 2,
    "method": "tools/list"
  }'
```

#### Call get-top-pizzas tool

```bash
curl -X POST http://localhost:8084/mcp \
  -H "Content-Type: application/json" \
  -H "mcp-session-id: YOUR_SESSION_ID" \
  -d '{
    "jsonrpc": "2.0",
    "id": 3,
    "method": "tools/call",
    "params": {
      "name": "get-top-pizzas"
    }
  }'
```

## Architecture

This server demonstrates the official MCP TypeScript SDK architecture:

1. **McpServer**: Core server instance that handles MCP protocol
2. **StreamableHTTPServerTransport**: Modern transport for HTTP-based communication with SSE streaming
3. **SSEServerTransport**: Legacy transport for backward compatibility
4. **Session Management**: Maintains state across multiple requests

## Integration with Inference Gateway

This server is designed to work seamlessly with the Inference Gateway's MCP middleware:

```env
MCP_SERVERS=http://pizza-demo-mcp-server:8084/mcp
```

The server supports both traditional JSON-RPC responses and SSE streaming, making it fully compatible with the enhanced MCP client and middleware implementations.

## Mock Data

The server returns information about these top 5 pizzas:

1. **Margherita** (Naples, Italy) - The classic with tomato sauce, fresh mozzarella, and basil
2. **Neapolitan** (Naples, Italy) - The original pizza with thin, soft crust and minimal toppings
3. **Pepperoni** (United States) - American classic with pepperoni sausage and cheese
4. **Four Cheese (Quattro Formaggi)** (Italy) - Rich pizza featuring four different cheeses
5. **Hawaiian** (Canada) - Controversial but popular pizza with ham and pineapple
