# Multi-stage build for better caching and smaller final image
FROM mcr.microsoft.com/devcontainers/go:2-1.25-bookworm AS base

ENV TASK_VERSION=v3.45.5 \
    CTLPTL_VERSION=0.8.43 \
    K3D_VERSION=v5.8.3 \
    KUBECTL_VERSION=v1.34.3 \
    HELM_VERSION=v3.19.1 \
    GORELEASER_VERSION=v2.13.0 \
    GOLANG_CI_LINT_VERSION=v2.7.2 \
    GITHUB_CLI_VERSION=2.83.2 \
    NODE_VERSION=lts \
    SEMANTIC_RELEASE_VERSION=v25.0.2 \
    INFERENCE_GATEWAY_CLI=v0.95.0

# Determine architecture once and store as build arg
RUN architecture=$(uname -m) && \
    if [ "$architecture" = "x86_64" ]; then \
        echo "amd64" > /tmp/arch; \
    else \
        echo "arm64" > /tmp/arch; \
    fi

# Install system dependencies (stable layer)
RUN apt-get update && \
    apt-get install -y iputils-ping dnsutils && \
    apt-get clean && \
    rm -rf /var/lib/apt/lists/*

# Install Node.js (stable layer)
RUN curl -fsSL https://deb.nodesource.com/setup_${NODE_VERSION}.x | bash - && \
    apt-get install -y nodejs && \
    apt-get clean && \
    rm -rf /var/lib/apt/lists/*

# Install task
RUN architecture_pretty=$(cat /tmp/arch) && \
    echo "Installing dependencies for ${architecture_pretty}" && \
    curl -s https://taskfile.dev/install.sh | sh -s -- -b /usr/local/bin ${TASK_VERSION}

# Install ctlptl
RUN architecture_pretty=$(cat /tmp/arch) && \
    curl -fsSL https://github.com/tilt-dev/ctlptl/releases/download/v${CTLPTL_VERSION}/ctlptl.${CTLPTL_VERSION}.linux.${architecture_pretty}.tar.gz | tar -xzv -C /usr/local/bin ctlptl

# Install k3d
RUN curl -fsSL https://raw.githubusercontent.com/k3d-io/k3d/main/install.sh | TAG=${K3D_VERSION} bash

# Install kubectl
RUN architecture_pretty=$(cat /tmp/arch) && \
    curl -fsSL https://dl.k8s.io/release/${KUBECTL_VERSION}/bin/linux/${architecture_pretty}/kubectl -o /usr/local/bin/kubectl && \
    chmod +x /usr/local/bin/kubectl

# Install helm
RUN curl -fsSL https://raw.githubusercontent.com/helm/helm/main/scripts/get-helm-3 | HELM_INSTALL_DIR=/usr/local/bin HELM_VERSION=${HELM_VERSION} bash

# Install goreleaser
RUN architecture_pretty=$(cat /tmp/arch) && \
    curl -fsSL https://github.com/goreleaser/goreleaser/releases/download/${GORELEASER_VERSION}/goreleaser_Linux_${architecture_pretty}.tar.gz | tar -xzv -C /usr/local/bin goreleaser

# Install golangci-lint
RUN curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/HEAD/install.sh | sh -s -- -b $(go env GOPATH)/bin ${GOLANG_CI_LINT_VERSION}

# Install gh cli
RUN architecture_pretty=$(cat /tmp/arch) && \
    curl -fsSL https://github.com/cli/cli/releases/download/v${GITHUB_CLI_VERSION}/gh_${GITHUB_CLI_VERSION}_linux_${architecture_pretty}.tar.gz -o /tmp/gh.tar.gz && \
    tar -xzf /tmp/gh.tar.gz -C /tmp && \
    find /tmp -name "gh" -type f -executable -exec mv {} /usr/local/bin/gh \; && \
    rm -rf /tmp/gh*

# Install Inference Gateway CLI
RUN curl -fsSL https://raw.githubusercontent.com/inference-gateway/cli/main/install.sh | bash -s -- --version ${INFERENCE_GATEWAY_CLI}

# Install Node.js packages
RUN npm install -g @stoplight/spectral-cli

# Install semantic-release
RUN npm install -g \
    semantic-release@${SEMANTIC_RELEASE_VERSION} \
    @semantic-release/changelog \
    @semantic-release/git \
    @semantic-release/github \
    @semantic-release/exec \
    conventional-changelog-cli \
    conventional-changelog-conventionalcommits

RUN npm install -g prettier@3.6.2

# Development stage with all tools
FROM base AS development

USER vscode

RUN go install go.uber.org/mock/mockgen@v0.6.0 && \
    go install github.com/inference-gateway/tools/cmd/generator@v0.1.2

# Use Powerlevel10k theme
RUN git clone --depth=1 https://github.com/romkatv/powerlevel10k.git /home/vscode/.powerlevel10k

# Configure Powerlevel10k
RUN echo 'source /home/vscode/.powerlevel10k/powerlevel10k.zsh-theme' >> /home/vscode/.zshrc && \
    echo 'POWERLEVEL9K_DISABLE_CONFIGURATION_WIZARD=true' >> /home/vscode/.zshrc && \
    echo 'POWERLEVEL9K_LEFT_PROMPT_ELEMENTS=(dir vcs)' >> /home/vscode/.zshrc && \
    echo 'POWERLEVEL9K_RIGHT_PROMPT_ELEMENTS=(command_execution_time status)' >> /home/vscode/.zshrc && \
    echo 'POWERLEVEL9K_COMMAND_EXECUTION_TIME_THRESHOLD=0' >> /home/vscode/.zshrc && \
    echo 'POWERLEVEL9K_COMMAND_EXECUTION_TIME_PRECISION=2' >> /home/vscode/.zshrc && \
    echo 'POWERLEVEL9K_COMMAND_EXECUTION_TIME_FORMAT="duration"' >> /home/vscode/.zshrc


# Shell completion
RUN echo "source <(gh completion -s zsh)" >> /home/vscode/.zshrc && \
    echo "source <(kubectl completion zsh)" >> /home/vscode/.zshrc && \
    echo "source <(helm completion zsh)" >> /home/vscode/.zshrc && \
    echo "source <(k3d completion zsh)" >> /home/vscode/.zshrc && \
    echo "source <(ctlptl completion zsh)" >> /home/vscode/.zshrc && \
    echo "source <(task --completion zsh)" >> /home/vscode/.zshrc && \
    echo "source <(goreleaser completion zsh)" >> /home/vscode/.zshrc && \
    echo "source <(infer completion zsh)" >> /home/vscode/.zshrc
