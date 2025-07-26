import { McpServer } from '@modelcontextprotocol/sdk/server/mcp.js';
import { StreamableHTTPServerTransport } from '@modelcontextprotocol/sdk/server/streamableHttp.js';
import { SSEServerTransport } from '@modelcontextprotocol/sdk/server/sse.js';
import express, { Request, Response } from 'express';
import { randomUUID } from 'node:crypto';

const app = express();
app.use(express.json());

// CORS middleware
app.use((req, res, next) => {
  res.header('Access-Control-Allow-Origin', '*');
  res.header('Access-Control-Allow-Methods', 'GET, POST, PUT, DELETE, OPTIONS');
  res.header(
    'Access-Control-Allow-Headers',
    'Content-Type, Authorization, mcp-session-id',
  );
  if (req.method === 'OPTIONS') {
    res.sendStatus(200);
    return;
  }
  next();
});

// Store transports for session management
const transports = {
  streamable: {} as Record<string, StreamableHTTPServerTransport>,
  sse: {} as Record<string, SSEServerTransport>,
};

// Mock data for top 5 pizzas in the world
const TOP_PIZZAS = [
  {
    rank: 1,
    name: 'Margherita',
    origin: 'Naples, Italy',
    description:
      'A classic pizza with tomato sauce, fresh mozzarella, and basil',
    yearCreated: 1889,
    keyIngredients: [
      'San Marzano tomatoes',
      'Mozzarella di Bufala',
      'Fresh basil',
      'Olive oil',
    ],
  },
  {
    rank: 2,
    name: 'Neapolitan',
    origin: 'Naples, Italy',
    description:
      'The original pizza with a thin, soft crust and minimal toppings',
    yearCreated: 1750,
    keyIngredients: ['Tomato sauce', 'Olive oil', 'Garlic', 'Oregano'],
  },
  {
    rank: 3,
    name: 'Pepperoni',
    origin: 'United States',
    description: 'An American classic with pepperoni sausage and cheese',
    yearCreated: 1950,
    keyIngredients: [
      'Pepperoni',
      'Mozzarella cheese',
      'Tomato sauce',
      'Italian herbs',
    ],
  },
  {
    rank: 4,
    name: 'Four Cheese (Quattro Formaggi)',
    origin: 'Italy',
    description: 'A rich pizza featuring four different types of cheese',
    yearCreated: 1960,
    keyIngredients: [
      'Mozzarella',
      'Gorgonzola',
      'Parmigiano-Reggiano',
      'Ricotta',
    ],
  },
  {
    rank: 5,
    name: 'Hawaiian',
    origin: 'Canada',
    description: 'A controversial but popular pizza with ham and pineapple',
    yearCreated: 1962,
    keyIngredients: ['Ham', 'Pineapple', 'Mozzarella cheese', 'Tomato sauce'],
  },
];

// Create and configure the MCP server
function createMcpServer(): McpServer {
  const server = new McpServer({
    name: 'Pizza Demo MCP Server',
    version: '1.0.0',
  });

  // Single tool: get top pizzas
  server.tool(
    'get_top_pizzas',
    'Get information about the top 5 pizzas in the world',
    async () => {
      console.log('🍕 get_top_pizzas tool called!');

      const result = {
        content: [
          {
            type: 'text' as const,
            text: `Top 5 Pizzas in the World:\n\n${TOP_PIZZAS.map(
              (pizza) =>
                `${pizza.rank}. ${pizza.name} (${pizza.origin})\n` +
                `   Description: ${pizza.description}\n` +
                `   Year Created: ${pizza.yearCreated}\n` +
                `   Key Ingredients: ${pizza.keyIngredients.join(', ')}\n`,
            ).join('\n')}`,
          },
        ],
      };

      console.log('🍕 Returning pizza data');
      return result;
    },
  );

  return server;
}

// Modern Streamable HTTP endpoint (supports both session-based and stateless)
app.all('/mcp', async (req: Request, res: Response) => {
  try {
    const sessionId = req.headers['mcp-session-id'] as string | undefined;
    let transport: StreamableHTTPServerTransport;

    if (sessionId && transports.streamable[sessionId]) {
      // Reuse existing transport
      transport = transports.streamable[sessionId];
    } else {
      // Create new transport
      const server = createMcpServer();

      transport = new StreamableHTTPServerTransport({
        sessionIdGenerator: () => randomUUID(),
        onsessioninitialized: (newSessionId) => {
          transports.streamable[newSessionId] = transport;
          console.log(
            `New Streamable HTTP session initialized: ${newSessionId}`,
          );
        },
      });

      // Clean up on close
      transport.onclose = () => {
        if (transport.sessionId) {
          delete transports.streamable[transport.sessionId];
          console.log(`Streamable HTTP session closed: ${transport.sessionId}`);
        }
      };

      await server.connect(transport);
    }

    await transport.handleRequest(req, res, req.body);
  } catch (error) {
    console.error('Error handling Streamable HTTP request:', error);
    if (!res.headersSent) {
      res.status(500).json({
        jsonrpc: '2.0',
        error: {
          code: -32603,
          message: 'Internal server error',
        },
        id: null,
      });
    }
  }
});

// Legacy SSE endpoint for backward compatibility
app.get('/sse', async (req: Request, res: Response) => {
  try {
    console.log('New SSE connection request');

    const server = createMcpServer();
    const transport = new SSEServerTransport('/messages', res);
    const sessionId = randomUUID();

    transports.sse[sessionId] = transport;
    console.log(`New SSE session initialized: ${sessionId}`);

    res.on('close', () => {
      delete transports.sse[sessionId];
      console.log(`SSE session closed: ${sessionId}`);
    });

    await server.connect(transport);
  } catch (error) {
    console.error('Error handling SSE connection:', error);
    if (!res.headersSent) {
      res.status(500).send('Internal server error');
    }
  }
});

// Legacy message endpoint for SSE clients
app.post('/messages', async (req: Request, res: Response) => {
  try {
    const sessionId = req.query.sessionId as string;
    const transport = transports.sse[sessionId];

    console.log(`📨 SSE message received for session: ${sessionId}`);
    console.log(`📋 Request body:`, JSON.stringify(req.body, null, 2));

    if (transport) {
      console.log(
        `✅ Transport found for session ${sessionId}, handling message...`,
      );
      await transport.handlePostMessage(req, res, req.body);
      console.log(`✅ Message handled for session ${sessionId}`);
    } else {
      console.log(`❌ No transport found for sessionId: ${sessionId}`);
      res.status(400).json({
        jsonrpc: '2.0',
        error: {
          code: -32000,
          message: 'No transport found for sessionId',
        },
        id: null,
      });
    }
  } catch (error) {
    console.error('❌ Error handling SSE message:', error);
    if (!res.headersSent) {
      res.status(500).json({
        jsonrpc: '2.0',
        error: {
          code: -32603,
          message: 'Internal server error',
        },
        id: null,
      });
    }
  }
});


// Info endpoint
app.get('/', (req: Request, res: Response) => {
  res.json({
    name: 'Pizza Demo MCP Server',
    version: '1.0.0',
    description: 'Simple demo server showcasing top 5 pizzas in the world',
    endpoints: {
      mcp: '/mcp (Streamable HTTP - GET/POST/DELETE)',
      sse: '/sse (Legacy SSE - GET)',
      messages: '/messages (Legacy SSE Messages - POST)',
      info: '/',
    },
    capabilities: {
      tools: [
        {
          name: 'get-top-pizzas',
          description: 'Get the top 5 pizzas in the world with details',
        },
      ],
    },
  });
});

const PORT = process.env.PORT ? parseInt(process.env.PORT) : 8084;

app.listen(PORT, () => {
  console.log(`🍕 Pizza Demo MCP Server started on port ${PORT}`);
  console.log(`📍 Endpoints:`);
  console.log(`   • Streamable HTTP: http://localhost:${PORT}/mcp`);
  console.log(`   • Legacy SSE: http://localhost:${PORT}/sse`);
  console.log(`   • Info: http://localhost:${PORT}/`);
  console.log(`🔧 Built with official @modelcontextprotocol/sdk`);
});

// Graceful shutdown
process.on('SIGINT', () => {
  console.log('\n🛑 Shutting down server...');

  // Close all active transports
  Object.values(transports.streamable).forEach((transport) =>
    transport.close(),
  );
  Object.values(transports.sse).forEach((transport) => transport.close());

  process.exit(0);
});

export default app;
