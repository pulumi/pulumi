# Copyright 2026, Pulumi Corporation.
#
# Prebaked toolchain image for the ubuntu test jobs in ci-run-test.yml.
# Built by ci-toolchain-image.yml, tagged by content hash (see
# .github/scripts/toolchain-image-tag).
#
# mise is used only while building. It installs the tools, every tool
# binary is linked into /opt/toolchain/bin, and mise is then deleted.
# At runtime the tools are plain binaries on PATH, so nothing installs
# lazily and workspace configs cannot change tool resolution (the
# flakiness that reverted #22181). verify-tools is the pre-push gate
# that checks the versions and that mise is gone.
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
# below. mise.dev.toml is not copied in.
COPY .mise.toml /src/.mise.toml
COPY .github/scripts/tool-pin /src/.github/scripts/tool-pin

# The whole toolchain installs in one layer so mise never outlives it.
# The npm, Go and Python steps mirror install-toolchain's host steps.
# The optional secret token avoids GitHub API rate limits.
RUN --mount=type=secret,id=github-token <<'EOF'
    set -euo pipefail
    # mise settings for this layer only. No other layer runs mise.
    export MISE_DATA_DIR=/opt/mise \
           MISE_GLOBAL_CONFIG_FILE=/src/.mise.toml \
           MISE_TRUSTED_CONFIG_PATHS=/src \
           MISE_CACHE_DIR=/tmp/mise-cache
    if [ -f /run/secrets/github-token ]; then export GITHUB_TOKEN="$(cat /run/secrets/github-token)"; fi
    set -x

    # The matrix passes versions like "1.27.x" but mise wants "1.27".
    mise config set -t string -f "$MISE_GLOBAL_CONFIG_FILE" tools.go "${GO_VERSION%.x}"
    mise config set -t string -f "$MISE_GLOBAL_CONFIG_FILE" tools.node "${NODE_VERSION%.x}"
    mise config set -t string -f "$MISE_GLOBAL_CONFIG_FILE" tools.python "${PYTHON_VERSION%.x}"
    mise config set -t string -f "$MISE_GLOBAL_CONFIG_FILE" tools.dotnet "${DOTNET_VERSION}"
    # Retry once, downloads can flake and mise install is idempotent.
    mise install || { sleep 15; mise install; }
    # Put the installed tools on PATH for the rest of this script.
    PATH="$(mise bin-paths | tr '\n' ':')${PATH}"
    export PATH

    PIN=/src/.github/scripts/tool-pin

    # npm upgrades first because the dependency cooldown needs npm >=
    # 11.17.0. yarn and pnpm then install with the cooldown enforced.
    export npm_config_min_release_age="${COOLDOWN_DAYS}"
    npm install -g "npm@$($PIN npm)"
    npm install -g "yarn@$($PIN yarn)" "pnpm@$($PIN pnpm)"

    # Go test tools. Test jobs restore their own Go build caches.
    GOBIN=/usr/local/bin go install "github.com/go-delve/delve/cmd/dlv@v$($PIN dlv)"
    GOBIN=/usr/local/bin go install "gotest.tools/gotestsum@v$($PIN gotestsum)"

    # Python deps and Poetry. pip upgrades first because the cooldown
    # needs pip >= 26.1. mise's python has no PEP 668 marker, so pip
    # can manage it directly.
    export PIP_UPLOADED_PRIOR_TO="P${COOLDOWN_DAYS}D"
    python -m ensurepip --upgrade
    python -m pip install --no-cache-dir --upgrade pip
    python -m pip install --no-cache-dir requests wheel urllib3 chardet build
    python -m pip install --no-cache-dir poetry

    # Link every tool binary, including the npm globals and pip scripts
    # installed above, into the one directory on the runtime PATH. Only
    # executable files: dotnet's bin path is its whole SDK tree. ln
    # fails the build if two tools ship the same binary name.
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
    # JAVA_HOME and DOTNET_ROOT need stable paths and the versioned
    # install paths change every rebuild. Symlinks rather than moves
    # keep mise's tree intact, and copies would double the image size.
    # readlink because dotnet's install dir is itself a symlink.
    ln -s "$(readlink -f "$(mise where java)")" /opt/java
    ln -s "$(readlink -f "$(mise where dotnet)")" /opt/dotnet

    # mise's work is done. Deleting it and its shims makes runtime
    # resolution impossible to sway. verify-tools asserts it stays
    # gone. Also drop the build caches.
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

# Tests must not run as root or they would bypass the file modes some
# tests rely on. uid 1001 matches the hosted runner user that owns the
# mounted workspace, and passwordless sudo mirrors the hosts. The go
# dirs make cache volumes mounted there start out user-owned.
RUN useradd --uid 1001 --create-home runner \
    && echo "runner ALL=(ALL) NOPASSWD: ALL" > /etc/sudoers.d/runner \
    && mkdir -p /home/runner/go /home/runner/.cache/go-build \
    && chown -R runner:runner /home/runner/go /home/runner/.cache
USER runner
