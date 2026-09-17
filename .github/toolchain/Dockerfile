# Copyright 2026, Pulumi Corporation.
#
# Prebaked toolchain image for the ubuntu test jobs in ci-run-test.yml.
# Built by ci-toolchain-image.yml, tagged by content hash (see
# .github/scripts/toolchain-image-tag).
#
# mise installs the tools at build time and is then deleted, leaving plain
# binaries on PATH: no lazy installs, no runtime resolution to sway (#22181).
#
# Language versions come from the test matrix via build args. All other
# pins come from .mise.toml.
#
# Build from the repo root: docker build -f .github/toolchain/Dockerfile .

# Matches GitHub's ubuntu-latest runners, so container and host runs
# share the same OS.
FROM ubuntu:24.04

# The tag rolls weekly. Storing it in the first layer busts the layer
# cache so floating pins (Go patch, apt packages) re-resolve.
ARG TOOLCHAIN_TAG=dev
RUN echo "${TOOLCHAIN_TAG}" > /etc/toolchain-tag

# Language versions from the test matrix (get-job-matrix.py) and the
# cooldown window from the dependency-cooldown action.
ARG GO_VERSION=
ARG NODE_VERSION=
ARG PYTHON_VERSION=
ARG DOTNET_VERSION=
ARG COOLDOWN_DAYS=
RUN test -n "$GO_VERSION" && test -n "$NODE_VERSION" \
    && test -n "$PYTHON_VERSION" && test -n "$DOTNET_VERSION" \
    && test -n "$COOLDOWN_DAYS" \
    || { echo "FATAL: version build args not set. Build via ci-toolchain-image.yml or .github/toolchain/validate-local." >&2; exit 1; }

# On a bump, update the hash from the release's SHASUMS256.txt.
ARG MISE_VERSION=v2026.4.11
ARG MISE_SHA256=514bb582966f576f0f4225f929fe8f307f7bd5c3ccca5565bbab616c6487190d

ARG DEBIAN_FRONTEND=noninteractive

# The heredoc RUN script below runs under this shell.
SHELL ["/bin/bash", "-o", "pipefail", "-c"]

RUN apt-get update && apt-get install -y --no-install-recommends \
        bash build-essential ca-certificates curl git \
        jq libicu74 make sudo tar tzdata unzip xz-utils zip \
    && rm -rf /var/lib/apt/lists/*

RUN curl -fsSLo /tmp/mise.tgz \
        "https://github.com/jdx/mise/releases/download/${MISE_VERSION}/mise-${MISE_VERSION}-linux-x64.tar.gz" \
    && echo "${MISE_SHA256}  /tmp/mise.tgz" | sha256sum -c - \
    && tar -xzf /tmp/mise.tgz -C /usr/local --strip-components=1 mise/bin/mise \
    && rm /tmp/mise.tgz

# .mise.toml is used as is, except the four language pins are overridden
# below.
COPY .mise.toml /src/.mise.toml

# The whole toolchain installs in one layer so mise never outlives it.
# The optional secret token avoids GitHub API rate limits.
RUN --mount=type=secret,id=github-token <<'EOF'
    set -euo pipefail
    export MISE_DATA_DIR=/opt/mise \
           MISE_GLOBAL_CONFIG_FILE=/src/.mise.toml \
           MISE_TRUSTED_CONFIG_PATHS=/src \
           MISE_CACHE_DIR=/tmp/mise-cache
    # Shell npm installs out to npm itself and apply the dependency
    # cooldown, as the dependency-cooldown action does for the host jobs.
    export MISE_NPM_SHELL_OUT=true \
           npm_config_min_release_age="${COOLDOWN_DAYS}" \
           PIP_UPLOADED_PRIOR_TO="P${COOLDOWN_DAYS}D"
    if [ -f /run/secrets/github-token ]; then export GITHUB_TOKEN="$(cat /run/secrets/github-token)"; fi
    set -x

    # The matrix passes versions like "1.27.x" but mise wants "1.27".
    mise config set -t string -f "$MISE_GLOBAL_CONFIG_FILE" tools.go "${GO_VERSION%.x}"
    mise config set -t string -f "$MISE_GLOBAL_CONFIG_FILE" tools.node "${NODE_VERSION%.x}"
    mise config set -t string -f "$MISE_GLOBAL_CONFIG_FILE" tools.python "${PYTHON_VERSION%.x}"
    mise config set -t string -f "$MISE_GLOBAL_CONFIG_FILE" tools.dotnet "${DOTNET_VERSION}"

    # One command installs the whole toolchain. mise installs node before
    # the npm-backed tools, which shell out to node's bundled npm: it must
    # be cooldown-capable (>= 11.17.0), which verify-tools asserts.
    mise install
    export PATH="$(mise bin-paths | tr '\n' ':')${PATH}"

    # Python deps and Poetry. pip upgrades first because the cooldown
    # needs pip >= 26.1.
    python -m ensurepip --upgrade
    python -m pip install --no-cache-dir --upgrade pip
    python -m pip install --no-cache-dir requests wheel urllib3 chardet build poetry

    # Link every executable into the one directory on the runtime PATH.
    # ln fails the build if two tools ship the same binary name.
    mkdir -p /opt/toolchain/bin
    for dir in $(mise bin-paths); do
        for file in "$dir"/*; do
            if [ -f "$file" ] && [ -x "$file" ]; then
                # Symlinks rather than copies as go, npm, python and java
                # locate their libraries relative to the executable's path.
                ln -s "$file" /opt/toolchain/bin/
            fi
        done
    done
    # mise wraps node's npm launcher in a reshim hook that resolves its
    # paths from the unresolved $0, so it breaks behind the symlink (and
    # needs mise). Link npm's real entry point instead, like node's own
    # npx symlink does.
    ln -sf "$(mise where node)/lib/node_modules/npm/bin/npm-cli.js" /opt/toolchain/bin/npm

    # Stable paths for JAVA_HOME and DOTNET_ROOT. readlink because
    # dotnet's install dir is itself a symlink.
    ln -s "$(readlink -f "$(mise where java)")" /opt/java
    ln -s "$(readlink -f "$(mise where dotnet)")" /opt/dotnet

    # mise's work is done. verify-tools asserts it stays gone.
    rm -rf /usr/local/bin/mise /opt/mise/shims "$MISE_CACHE_DIR" \
        /root/go /root/.cache/go-build
EOF

# Plain binaries only, no shims and no runtime version resolution.
ENV PATH=/opt/toolchain/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin \
    JAVA_HOME=/opt/java \
    DOTNET_ROOT=/opt/dotnet \
    DOTNET_CLI_TELEMETRY_OPTOUT=true

# Let git operate on repo mounts owned by another uid (validate-local).
RUN git config --system --add safe.directory '*'

# Tests run as non-root so file permission checks hold. uid 1001 matches
# the hosted runner user, and the go dirs keep cache mounts user-owned.
RUN useradd --uid 1001 --create-home runner \
    && echo "runner ALL=(ALL) NOPASSWD: ALL" > /etc/sudoers.d/runner \
    && mkdir -p /home/runner/go /home/runner/.cache/go-build \
    && chown -R runner:runner /home/runner/go /home/runner/.cache
USER runner
