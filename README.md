<h1 align="center">Inference Gateway</h1>

<p align="center">
  <!-- CI Status Badge -->
  <a href="https://github.com/edenreich/inference-gateway/actions/workflows/ci.yml?query=branch%3Amain">
    <img src="https://github.com/edenreich/inference-gateway/actions/workflows/ci.yml/badge.svg?branch=main" alt="CI Status"/>
  </a>
  <!-- Version Badge -->
  <a href="https://github.com/edenreich/inference-gateway/releases">
    <img src="https://img.shields.io/github/v/release/edenreich/inference-gateway?color=blue&style=flat-square" alt="Version"/>
  </a>
  <!-- License Badge -->
  <a href="https://github.com/edenreich/inference-gateway/blob/main/LICENSE">
    <img src="https://img.shields.io/github/license/edenreich/inference-gateway?color=blue&style=flat-square" alt="License"/>
  </a>
</p>

The Inference Gateway is a proxy server designed to facilitate access to various language model APIs. It allows users to interact with different language models through a unified interface, simplifying the configuration and the process of sending requests and receiving responses from multiple LLMs, enabling an easy use of Mixture of Experts.

- [Key Features](#key-features)
- [Overview](#overview)
- [Supported API's](#supported-apis)
- [Configuration](#configuration)
- [Examples](#examples)
- [SDKs](#sdks)
- [License](#license)
- [Contributing](#contributing)
- [Motivation](#motivation)

## Key Features

- 📜 **Open Source**: Available under the MIT License.
- 🚀 **Unified API Access**: Proxy requests to multiple language model APIs, including OpenAI, Ollama, Groq, Cohere etc.
- ⚙️ **Environment Configuration**: Easily configure API keys and URLs through environment variables.
- 🐳 **Docker Support**: Use Docker and Docker Compose for easy setup and deployment.
- ☸️ **Kubernetes Support**: Ready for deployment in Kubernetes environments.
- 📊 **OpenTelemetry Tracing**: Enable tracing for the server to monitor and analyze performance.
- 🛡️ **Production Ready**: Built with production in mind, with configurable timeouts and TLS support.
- 🌿 **Lightweight**: Includes only essential libraries and runtime, resulting in smaller size binary of ~10.8MB.
- 📉 **Minimal Resource Consumption**: Designed to consume minimal resources and have a lower footprint.
- 📚 **Documentation**: Well documented with examples and guides.
- 🧪 **Tested**: Extensively tested with unit tests and integration tests.
- 🛠️ **Maintained**: Actively maintained and developed.
- 📈 **Scalable**: Easily scalable and can be used in a distributed environment - with <a href="https://kubernetes.io/docs/tasks/run-application/horizontal-pod-autoscale/" target="_blank">HPA</a> in Kubernetes.
- 🔒 **Compliance** and Data Privacy: This project does not collect data or analytics, ensuring compliance and data privacy.
- 🏠 **Self-Hosted**: Can be self-hosted for complete control over the deployment environment.

## Overview

```mermaid
graph TD
    A[Client] -->|Auth?| B[Inference Gateway]
    A -->|GET /health| I[Health Check]
    B -->|GET /llms| P[Proxy Gateway]
    B -->|POST /llms/provider/generate| P[Proxy Gateway]
    P[Proxy Gateway] --> C[Ollama API]
    P[Proxy Gateway] --> D[Groq API]
    P[Proxy Gateway] --> E[OpenAI API]
    P[Proxy Gateway] --> F[Google API]
    P[Proxy Gateway] --> G[Cloudflare API]
    P[Proxy Gateway] --> H[Cohere API]
```

Client is sending:

```bash
curl -X POST http://localhost:8080/llms/openai/generate
  -d '{"prompt": "Hello, world!", "model": "gpt-3.5-turbo"}'
```

Client receives:

```json
{
  "provider": "openai",
  "response": "Hello, world! How are you doing today?"
}
```

## Supported API's

- [OpenAI](https://platform.openai.com/)
- [Ollama](https://ollama.com/)
- [Groq Cloud](https://console.groq.com/)
- [Google](https://aistudio.google.com/)
- [Cloudflare](https://www.cloudflare.com/)
- [Cohere](https://docs.cohere.com/docs/the-cohere-platform)

## Configuration

The Inference Gateway can be configured using environment variables. The following [environment variables](./Configurations.md) are supported.

## Examples

- Using [Docker Compose](examples/docker-compose/)
- Using [Kubernetes](examples/kubernetes/)
- Using standard [REST endpoints](examples/rest-endpoints/)

## SDKs

More SDKs could be generated using the OpenAPI specification. The following SDKs are currently available:

- [Go SDK](https://github.com/edenreich/inference-gateway-go-sdk)

## License

This project is licensed under the MIT License.

## Contributing

Found a bug, missing provider, or have a feature in mind?  
You're more than welcome to submit pull requests or open issues for any fixes, improvements, or new ideas!

## Motivation

My motivation is to build AI Agents without being tied to a single vendor. By avoiding vendor lock-in and supporting self-hosted LLMs from a single interface, organizations gain both portability and data privacy. You can choose to consume LLMs from a cloud provider or run them entirely offline with Ollama.
