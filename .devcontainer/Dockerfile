FROM mcr.microsoft.com/devcontainers/go:1-1.23-bookworm

ENV ZSH_CUSTOM=/home/vscode/.oh-my-zsh/custom \
    TASK_VERSION=v3.39.2 \
    CTLPTL_VERSION=0.8.34 \
    K3D_VERSION=v5.7.4 \
    KUBECTL_VERSION=v1.31.0 \
    HELM_VERSION=v3.17.1 \
    GORELEASER_VERSION=v2.5.1 \
    GOLANG_CI_LINT_VERSION=v1.63.4

RUN apt-get update && \
    # Determine architecture
    architecture=$(uname -m); \
    architecture_pretty="arm64"; \
    if [ "$architecture" = "x86_64" ]; then \
    architecture_pretty="amd64"; \
    fi && \
    echo "Installing depenenices for ${architecture}" && \
    # Install networking utilities
    apt-get install -y iputils-ping dnsutils && \
    # Install powerlevel10k theme
    git clone --depth=1 https://github.com/romkatv/powerlevel10k.git ${ZSH_CUSTOM}/themes/powerlevel10k && \
    # Install Task
    curl -s https://taskfile.dev/install.sh | sh -s -- -b /usr/local/bin ${TASK_VERSION} && \
    # Install ctlptl
    curl -fsSL https://github.com/tilt-dev/ctlptl/releases/download/v${CTLPTL_VERSION}/ctlptl.${CTLPTL_VERSION}.linux.${architecture_pretty}.tar.gz | tar -xzv -C /usr/local/bin ctlptl && \
    # Install k3d
    curl -fsSL https://raw.githubusercontent.com/k3d-io/k3d/main/install.sh | TAG=${K3D_VERSION} bash && \
    # Install kubectl
    curl -fsSL https://dl.k8s.io/release/${KUBECTL_VERSION}/bin/linux/${architecture_pretty}/kubectl -o /usr/local/bin/kubectl && \
    chmod +x /usr/local/bin/kubectl && \
    # Install helm
    curl -fsSL https://raw.githubusercontent.com/helm/helm/main/scripts/get-helm-3 | HELM_INSTALL_DIR=/usr/local/bin HELM_VERSION=${HELM_VERSION} bash && \
    # Install GoReleaser
    curl -fsSL https://github.com/goreleaser/goreleaser/releases/download/${GORELEASER_VERSION}/goreleaser_Linux_${architecture_pretty}.tar.gz | tar -xzv -C /usr/local/bin goreleaser && \
    # Install golangci-lint
    curl -fsSL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b /usr/local/bin ${GOLANG_CI_LINT_VERSION} && \
    # Clean up
    apt-get clean && \
    rm -rf /var/lib/apt/lists/*

USER vscode

RUN go install go.uber.org/mock/mockgen@v0.5.0
