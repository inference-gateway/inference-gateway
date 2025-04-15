# Tools Example with Inference Gateway

This example demonstrates how to use the Inference Gateway with tools functionality, allowing models to call functions and process the results.

## What's Included

- **Inference Gateway**: The main service that proxies requests to various LLM providers
- **Agent**: A shell-based agent that demonstrates tools-use by making curl requests to the inference-gateway

## How It Works

The agent in this example:
1. Makes an initial request to the inference-gateway with a query that likely requires tools
2. Processes any tool calls requested by the model
3. Simulates tool execution (weather data and web search)
4. Returns the tool execution results to the model for completion

## Setup Instructions

1. Configure your API keys in the `.env` file:
   ```
   MODEL=openai/gpt-3.5-turbo  # Or another model that supports function calling
   OPENAI_API_KEY=your_openai_api_key_here
   ```

2. Start the services:
   ```bash
   docker compose up
   ```

3. Watch the agent logs to see the tool calls in action:
   ```bash
   docker compose logs -f agent
   ```

## Customizing the Agent

The agent implementation is written in the agent.sh file using curl commands for clarity.

## Available Tools

The agent currently implements two example tools:

1. `get_weather`: Simulates retrieving weather data for a specified location
2. `search_web`: Simulates searching the web for information

These are simple examples that return mock data. In a real implementation, you would connect these to actual APIs.

## Extending This Example

You can expand this example by:
- Adding more sophisticated tools
- Connecting to real APIs for weather data, search results, etc.
- Implementing a more interactive agent that can take user input
- Adding a web UI for interacting with the agent