FROM mcr.microsoft.com/devcontainers/base:1-bullseye

RUN apt-get update -y \
    && DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends \
        build-essential \
        ca-certificates \
        curl \
        file \
        inetutils-ping \
        tmux \
    && rm -rf /var/lib/apt/lists/*

# Install pulumictl
# RUN version=0.0.42 \
#     && curl -fsSLO https://github.com/pulumi/pulumictl/releases/download/v$version/pulumictl-v$version-linux-amd64.tar.gz \
#     && tar -xzf pulumictl-v$version-linux-amd64.tar.gz --directory /usr/local/bin --no-same-owner pulumictl \
#     && rm -f pulumictl-v$version-linux-amd64.tar.gz \
#     && pulumictl version

# RUN mkdir -p /go/bin \
#     && chown -R $USER_NAME: /go \
#     && mkdir -p $HOME/.pulumi/bin \
#     && chown -R $USER_NAME: $HOME/.pulumi
