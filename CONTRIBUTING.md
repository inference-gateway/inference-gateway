# Contributing to Inference Gateway

Thank you for considering contributing to Inference Gateway! We welcome contributions in many forms, including bug reports, feature requests, code, and documentation.

## Table of Contents

- [Contributing to Inference Gateway](#contributing-to-inference-gateway)
  - [Table of Contents](#table-of-contents)
  - [How to Contribute](#how-to-contribute)
    - [Reporting Bugs](#reporting-bugs)
    - [Requesting Features](#requesting-features)
    - [Code Contributions](#code-contributions)
    - [Development Setup](#development-setup)
    - [Code Style](#code-style)
  - [Adding New Providers](#adding-new-providers)
    - [Quick Start](#quick-start)
    - [Step-by-Step Guide](#step-by-step-guide)
      - [1. Configure the Provider](#1-configure-the-provider)
      - [2. Generate Provider Files](#2-generate-provider-files)
      - [3. Set Environment Variables](#3-set-environment-variables)
      - [4. Test Your Provider](#4-test-your-provider)
    - [Protected Files](#protected-files)
    - [Generated Files](#generated-files)
    - [Authentication Types](#authentication-types)
    - [Benefits](#benefits)

## How to Contribute

### Reporting Bugs

If you find a bug, please create an issue on GitHub with as much detail as possible. Include steps to reproduce the bug, the expected behavior, and any relevant logs or screenshots.

### Requesting Features

We welcome feature requests! Please create an issue on GitHub with a clear description of the feature and its benefits. If possible, include examples of how the feature would be used.

### Code Contributions

1. Fork the repository.
2. Create a new branch for your feature or bug fix.
3. **Set up development environment** by running `task pre-commit:install` to install pre-commit hooks for automatic code quality checks.
4. Write your code and tests.
5. Run the tests to ensure everything works.
6. Commit your changes and push your branch to your fork.
7. Create a pull request on GitHub.

### Development Setup

Before starting development, it's essential to set up your development environment properly:

```bash
# Install pre-commit hooks for automatic code quality checks
task pre-commit:install
```

This will install a pre-commit hook that automatically runs:

- Code generation (`task generate`)
- Linting (`task lint` and `task openapi-lint`)
- Building (`task build`)
- Testing (`task test`)

The pre-commit hook ensures code quality and prevents commits that would break the build or introduce inconsistencies.

### Code Style

Please follow the coding style used in the project. We use `gofmt` to format our Go code.

Also semantic-release is being used for automated releases, so please ensure your commits are following [conventional commits](https://www.conventionalcommits.org/en/v1.0.0/#specification).

You don't need to install most of the tools if you use VSCode, because there is a dev container.

## Adding New Providers

The Inference Gateway uses an automated code generation system to make onboarding new providers simple and consistent. The system generates all necessary provider files automatically from the OpenAPI specification.

### Quick Start

To add a new provider, follow these simple steps:

1. **Add provider configuration** to `openapi.yaml` under the `Provider` schema's `x-provider-configs` section
2. **Run code generation** with `task generate`
3. **Configure environment variables** for the new provider

### Step-by-Step Guide

#### 1. Configure the Provider

Add your new provider to the `openapi.yaml` file under the `Provider` schema. For example, to add a new provider called "newai":

> **Important**: Provider names must be valid Go identifiers. Use only lowercase letters, and camel-case if really needed (recommended: one word for example 'newai').

```yaml
Provider:
  type: string
  enum:
    - ollama
    - groq
    - openai
    - cloudflare
    - cohere
    - anthropic
    - deepseek
    - newai # Add your provider here
  x-provider-configs:
    # ... existing providers ...
    newai:
      id: "newai"
      url: "https://api.newai.com/v1"
      auth_type: "bearer" # or "xheader", "query", "none"
      endpoints:
        models:
          name: "list_models"
          method: "GET"
          endpoint: "/models"
        chat:
          name: "chat_completions"
          method: "POST"
          endpoint: "/chat/completions"
```

#### 2. Generate Provider Files

Run the code generation command to automatically create all necessary files:

```bash
task generate
```

This command will:

- Generate a new provider file (`providers/newai.go`) with OpenAI-compatible structure
- Update the provider registry (`providers/registry.go`) to include your provider
- Update configuration files to support the new provider
- Generate constants and types for the new provider

#### 3. Set Environment Variables

Configure your new provider by setting the appropriate environment variables:

```bash
export NEWAI_API_KEY="your-api-key-here"
export NEWAI_API_URL="https://api.newai.com/v1"  # Optional: override default URL
```

#### 4. Test Your Provider

Test the new provider by making a request:

```bash
curl -X POST http://localhost:8080/v1/chat/completions \
  -d '{
    "model": "newai/your-model-name",
    "messages": [{"role": "user", "content": "Hello!"}]
  }'
```

### Protected Files

The code generation system respects existing custom implementations through the `.openapi-ignore` file. Files listed there will not be overwritten during generation:

```
# .openapi-ignore
providers/anthropic.go
providers/cohere.go
providers/cloudflare.go
providers/ollama.go
providers/openai.go
providers/deepseek.go
providers/groq.go
```

If you need custom implementation details for your provider, add it to this ignore file after generation.

### Generated Files

The code generation process creates:

- **Provider implementation** (`providers/{provider}.go`): Contains the `ListModelsResponse` struct and `Transform()` method
- **Provider registry updates** (`providers/registry.go`): Adds your provider to the central registry
- **Configuration updates** (`config/config.go`): Includes environment variable support
- **Common types** (`providers/common_types.go`): Provider constants and endpoints

### Authentication Types

The system supports different authentication methods:

- **`bearer`**: Uses `Authorization: Bearer {token}` header
- **`xheader`**: Uses custom header (like Anthropic's `x-api-key`)
- **`query`**: Adds API key as query parameter
- **`none`**: No authentication required (like local Ollama)

### Benefits

- **Consistency**: All providers follow the same structure and patterns
- **Maintainability**: Changes to the OpenAPI spec automatically update all providers
- **Type Safety**: Generated Go types ensure compile-time correctness
- **Documentation**: Provider configurations are self-documenting in the OpenAPI spec
- **Testing**: Generated code follows established testing patterns
