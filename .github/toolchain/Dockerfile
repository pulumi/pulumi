# Copyright 2026, Pulumi Corporation.
#
# Prebaked toolchain image for the ubuntu test jobs in ci-run-test.yml.
# Built by ci-toolchain-image.yml, tagged by content hash (see
# .github/scripts/toolchain-image-tag). mise installs the tools from
# .mise.toml (language versions overridden by build args) and they are
# symlinked into /usr/local/bin, so nothing at job runtime depends on mise.
#
# Build from the repo root: docker build -f .github/toolchain/Dockerfile .

FROM ubuntu:24.04

# Recorded before any install layer: a rolled tag must bust the layer
# cache so floating pins re-resolve.
ARG TOOLCHAIN_TAG=dev
RUN echo "${TOOLCHAIN_TAG}" > /etc/toolchain-tag

# All tool versions arrive as required build args from their authorities
# (get-job-matrix.py, .mise.toml). Only mise, used solely here, is pinned.
# COOLDOWN_DAYS is the window for the image's own npm/pip installs; the
# build sites pass the default of .github/actions/dependency-cooldown, so
# the runtime authority stays the only one. It is exported per layer below,
# so the published image carries no cooldown settings of its own.
ARG GO_VERSION=
ARG NODE_VERSION=
ARG PYTHON_VERSION=
ARG DOTNET_VERSION=
ARG NPM_VERSION=
ARG COOLDOWN_DAYS=
RUN for v in GO_VERSION NODE_VERSION PYTHON_VERSION DOTNET_VERSION NPM_VERSION COOLDOWN_DAYS; do \
        eval "val=\${$v}"; \
        if [ -z "$val" ]; then \
            echo "FATAL: build-arg $v not set; build via ci-toolchain-image.yml or .github/toolchain/validate-local" >&2; \
            exit 1; \
        fi; \
    done
# renovate: datasource=github-releases depName=jdx/mise
ARG MISE_VERSION=v2026.4.11

ENV DEBIAN_FRONTEND=noninteractive

# Fail the build on fetch errors in the curl | sh pipes below.
SHELL ["/bin/bash", "-o", "pipefail", "-c"]

RUN apt-get update && apt-get install -y --no-install-recommends \
        bash build-essential ca-certificates curl git \
        jq libicu74 make sudo tar tzdata unzip xz-utils zip \
    && rm -rf /var/lib/apt/lists/*

# Fixed paths: the Actions runner overrides $HOME in job containers.
ENV MISE_DATA_DIR=/opt/mise \
    MISE_CACHE_DIR=/opt/mise/cache \
    MISE_GLOBAL_CONFIG_FILE=/opt/mise/config.toml
RUN curl -fsSL https://mise.run \
    | env MISE_VERSION="${MISE_VERSION}" MISE_INSTALL_PATH=/usr/local/bin/mise sh

# Derive a minimal config from .mise.toml: only the tools the test jobs
# need, with the language pins overridden by the build args.
COPY .mise.toml /tmp/mise-repo.toml
RUN set -eux; \
    mkdir -p "${MISE_DATA_DIR}"; \
    section() { awk -v s="[$1]" '$0==s{f=1;next} /^\[/{f=0} f' /tmp/mise-repo.toml; }; \
    { \
        echo '[tools]'; \
        echo "go = \"${GO_VERSION%.x}\""; \
        echo "node = \"${NODE_VERSION%.x}\""; \
        echo "python = \"${PYTHON_VERSION%.x}\""; \
        echo "dotnet = \"${DOTNET_VERSION}\""; \
        section tools | grep -E '^(java|gradle|uv|bun) = '; \
        section tools | grep -E '^"(npm:yarn|npm:pnpm|go:github\.com/go-delve/delve/cmd/dlv|go:gotest\.tools/gotestsum|github:pulumi/goteststats)" = '; \
        echo '[plugins]'; \
        section plugins | grep '^gradle = '; \
    } > "${MISE_GLOBAL_CONFIG_FILE}"; \
    cat "${MISE_GLOBAL_CONFIG_FILE}"

# The optional token (a BuildKit secret, never stored in a layer) avoids
# GitHub API rate limits. Caches are cleared within each layer, since later
# layers cannot shrink earlier ones.
RUN --mount=type=secret,id=github-token \
    set -eux; \
    if [ -f /run/secrets/github-token ]; then { set +x; } 2>/dev/null; export GITHUB_TOKEN="$(cat /run/secrets/github-token)"; set -x; fi; \
    mise install go node python dotnet java gradle uv bun; \
    mise cache clear

# Upgrade npm first: test jobs apply the dependency cooldown through this
# npm, which needs min-release-age-exclude (npm >= 11.17.0). Then install
# the remaining config tools (yarn, pnpm, dlv, gotestsum, goteststats).
# The bundled npm enforces the cooldown on the upgrade itself only if it is
# new enough, the same bootstrap gap host runners have.
RUN --mount=type=secret,id=github-token \
    set -eux; \
    if [ -f /run/secrets/github-token ]; then { set +x; } 2>/dev/null; export GITHUB_TOKEN="$(cat /run/secrets/github-token)"; set -x; fi; \
    export npm_config_min_release_age="${COOLDOWN_DAYS}"; \
    mise exec -- npm install -g "npm@${NPM_VERSION}"; \
    mise install; \
    mise cache clear; \
    rm -rf /root/go /root/.cache/go-build

# mise's python has no PEP 668 marker, so pip manages it directly, as on
# host runners. pip upgrades on its own first, so a cooldown-capable pip
# (>= 26.1, older versions ignore the window) installs the floating packages.
RUN set -eux; \
    export PIP_UPLOADED_PRIOR_TO="P${COOLDOWN_DAYS}D"; \
    mise exec -- python -m ensurepip --upgrade; \
    mise exec -- python -m pip install --no-cache-dir --upgrade pip; \
    mise exec -- python -m pip install --no-cache-dir requests wheel urllib3 chardet build; \
    mise exec -- python -m pip install --no-cache-dir poetry

# Symlink the real binaries into /usr/local/bin; reshim first so executables
# added since install (poetry) are indexed.
RUN set -eux; \
    mise reshim; \
    for tool in go gofmt node npm npx python python3 pip pip3 dotnet \
                java javac gradle uv uvx bun bunx yarn pnpm \
                dlv gotestsum goteststats poetry; do \
        real="$(mise which "$tool")"; \
        ln -sfv "$real" "/usr/local/bin/$tool"; \
    done; \
    java_home="$(mise where java)"; \
    dotnet_root="$(mise where dotnet)"; \
    ln -sfnv "$java_home" /opt/java; \
    ln -sfnv "$dotnet_root" /opt/dotnet
ENV JAVA_HOME=/opt/java \
    DOTNET_ROOT=/opt/dotnet \
    DOTNET_CLI_TELEMETRY_OPTOUT=true

# The workspace mount is owned by a different uid; let git operate on it.
RUN git config --system --add safe.directory '*'
