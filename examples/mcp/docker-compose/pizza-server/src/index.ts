// Pizza demo MCP server on the official TypeScript SDK v2. createMcpHandler
// serves MCP 2026-07-28 per request - the revision the gateway speaks upstream
// - and 2025-era clients from the same endpoint, with no session to keep.
import { createMcpExpressApp } from '@modelcontextprotocol/express';
import { toNodeHandler } from '@modelcontextprotocol/node';
import { createMcpHandler, McpServer } from '@modelcontextprotocol/server';

const PORT = process.env.PORT ? parseInt(process.env.PORT, 10) : 8084;

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

const formatPizzas = () =>
  `Top 5 Pizzas in the World:\n\n${TOP_PIZZAS.map(
    (pizza) =>
      `${pizza.rank}. ${pizza.name} (${pizza.origin})\n` +
      `   Description: ${pizza.description}\n` +
      `   Year Created: ${pizza.yearCreated}\n` +
      `   Key Ingredients: ${pizza.keyIngredients.join(', ')}\n`,
  ).join('\n')}`;

// The factory runs once per request, so every call gets a fresh server.
const handler = createMcpHandler(() => {
  const server = new McpServer({ name: 'pizza', version: '1.0.0' });
  server.registerTool(
    'get_top_pizzas',
    { description: 'Get information about the top 5 pizzas in the world' },
    async () => ({ content: [{ type: 'text', text: formatPizzas() }] }),
  );
  return server;
});

// Bound to all interfaces inside the container, so name the hosts the
// gateway reaches it by - the Compose service, and the Service the operator
// creates for the Kubernetes example's MCP resource, which service discovery
// addresses by its cluster FQDN; the DNS rebinding check rejects any other Host.
const app = createMcpExpressApp({
  host: '0.0.0.0',
  allowedHosts: [
    'mcp-pizza-server',
    'pizza-service.inference-gateway.svc.cluster.local',
    'localhost',
  ],
});
const node = toNodeHandler(handler);
app.all('/mcp', (req, res) => void node(req, res, req.body));
app.get('/health', (_req, res) => {
  res.sendStatus(200);
});

app.listen(PORT, () => {
  console.log(`🍕 Pizza MCP server listening on http://localhost:${PORT}/mcp`);
});
