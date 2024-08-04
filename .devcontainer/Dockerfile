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
