FROM ubuntu:mantic

ENV DEBIAN_FRONTEND=noninteractive
RUN apt-get update \
    && apt-get -y install --no-install-recommends \
        ca-certificates=20230311ubuntu1 \
        apt-utils=2.7.3 \
    && apt-get clean \
    && rm -rf /var/lib/apt/lists/*

RUN apt-get update \
    && apt-get -y install --no-install-recommends \
        git=1:2.40.1-1ubuntu1 \
        golang=2:1.21~2 \
        zsh=5.9-5ubuntu1 \
        tree=2.1.1-1 \
        neovim=0.7.2-7 \
        wget=1.21.3-1ubuntu1 \
    && apt-get clean \
    && rm -rf /var/lib/apt/lists/*

RUN export GOBIN=/usr/local/bin \
    && go install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.55.2 \
    && go install golang.org/x/tools/gopls@v0.14.2 \
    && go install github.com/goreleaser/goreleaser@v1.23.0 \
    && go install github.com/go-delve/delve/cmd/dlv@v1.22.0

RUN wget -q -O /usr/local/bin/hadolint \
        https://github.com/hadolint/hadolint/releases/download/v2.8.0/hadolint-Linux-x86_64 \
        && chmod +x /usr/local/bin/hadolint

ENV DEBIAN_FRONTEND=dialog

SHELL ["/bin/zsh", "-c"]
