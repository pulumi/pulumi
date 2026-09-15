# Copyright 2026, Pulumi Corporation.
#
# Prebaked toolchain image for the ubuntu test jobs in ci-run-test.yml.
# Built by ci-toolchain-image.yml, tagged by content hash (see
# .github/scripts/toolchain-image-tag).
#
# Each layer mirrors a step of .github/actions/install-toolchain and
# leaves its tools on a fixed PATH, so jobs never run mise (runtime shims
# lazily resolve through the mounted repo's config, the #22181 flake).
# Language versions come from the test matrix via build args, all other
# pins from .mise.toml via tool-pin.
#
# Build from the repo root: docker build -f .github/toolchain/Dockerfile .

# Matches GitHub's ubuntu-latest runners, so container and host runs
# share the same OS.
FROM ubuntu:24.04

# First layer: a rolled tag busts the cache so floating pins re-resolve.
ARG TOOLCHAIN_TAG=dev
RUN echo "${TOOLCHAIN_TAG}" > /etc/toolchain-tag

# Language versions from the test matrix (get-job-matrix.py), cooldown
# window from the dependency-cooldown action, exported per layer so the
# image carries no cooldown settings.
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

# The heredoc RUN scripts run under this shell.
SHELL ["/bin/bash", "-o", "pipefail", "-c"]

RUN apt-get update && apt-get install -y --no-install-recommends \
        bash build-essential ca-certificates curl git \
        jq libicu74 make sudo tar tzdata unzip xz-utils zip \
    && rm -rf /var/lib/apt/lists/*

# Fixed paths: the Actions runner overrides $HOME in job containers.
ENV MISE_DATA_DIR=/opt/mise \
    MISE_CACHE_DIR=/opt/mise/cache \
    MISE_GLOBAL_CONFIG_FILE=/src/.mise.toml
RUN curl -fsSLo /tmp/mise.tgz \
        "https://github.com/jdx/mise/releases/download/${MISE_VERSION}/mise-${MISE_VERSION}-linux-x64.tar.gz" \
    && echo "${MISE_SHA256}  /tmp/mise.tgz" | sha256sum -c - \
    && tar -xzf /tmp/mise.tgz -C /usr/local --strip-components=1 mise/bin/mise \
    && rm /tmp/mise.tgz

# .mise.toml is used verbatim, with the four language pins overridden by
# the build args. Its [tools] install in the next layer, its [vars] pins
# in the npm and go layers below. mise.dev.toml is not copied in.
COPY .mise.toml /src/.mise.toml
COPY .github/scripts/tool-pin /src/.github/scripts/tool-pin
RUN mise trust "$MISE_GLOBAL_CONFIG_FILE" \
    && mise config set -t string -f "$MISE_GLOBAL_CONFIG_FILE" tools.go "${GO_VERSION%.x}" \
    && mise config set -t string -f "$MISE_GLOBAL_CONFIG_FILE" tools.node "${NODE_VERSION%.x}" \
    && mise config set -t string -f "$MISE_GLOBAL_CONFIG_FILE" tools.python "${PYTHON_VERSION%.x}" \
    && mise config set -t string -f "$MISE_GLOBAL_CONFIG_FILE" tools.dotnet "${DOTNET_VERSION}"

# Counterpart of install-toolchain's setup-* steps. The optional secret
# token avoids GitHub API rate limits. Caches are cleared in-layer,
# later layers cannot shrink earlier ones.
RUN --mount=type=secret,id=github-token <<'EOF'
    set -euo pipefail
    if [ -f /run/secrets/github-token ]; then export GITHUB_TOKEN="$(cat /run/secrets/github-token)"; fi
    set -x
    # One bounded retry for transient network errors, mise install is
    # idempotent.
    mise install || { sleep 15; mise install; }
    mise cache clear
    # ENV PATH, JAVA_HOME and DOTNET_ROOT need version-independent paths,
    # but mise install dirs are named after versions resolved only now.
    # Whole-directory symlinks bridge that: the tools locate GOROOT, the
    # stdlib, node_modules etc. relative to their real path, so linking
    # or copying the binaries alone would break them.
    for tool in go node python java gradle dotnet; do
        ln -sfnv "$(mise where "$tool")" "/opt/$tool"
    done
    # Self-contained single binaries, symlinked to avoid storing them
    # twice.
    for tool in uv uvx bun bunx goteststats; do
        ln -sfv "$(mise which "$tool")" "/usr/local/bin/$tool"
    done
    # Fail the build, not a test job, if a layout assumed by the ENV PATH
    # below changes.
    for bin in /opt/go/bin/go /opt/go/bin/gofmt \
               /opt/node/bin/node /opt/node/bin/npm /opt/node/bin/npx \
               /opt/python/bin/python /opt/python/bin/python3 \
               /opt/java/bin/java /opt/java/bin/javac \
               /opt/gradle/bin/gradle /opt/dotnet/dotnet; do
        test -x "$bin"
    done
EOF
# The dotnet launcher sits at the root of its tree, not in bin/.
ENV PATH=/opt/go/bin:/opt/node/bin:/opt/python/bin:/opt/java/bin:/opt/gradle/bin:/opt/dotnet:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin \
    JAVA_HOME=/opt/java \
    DOTNET_ROOT=/opt/dotnet \
    DOTNET_CLI_TELEMETRY_OPTOUT=true

# Mirrors the host's npm step, globals land in /opt/node/bin. npm first,
# the cooldown needs npm >= 11.17.0, then yarn and pnpm install with it
# enforced. The upgrade itself is cooldown-checked only if the bundled
# npm is new enough, as on hosts.
RUN <<'EOF'
    set -euxo pipefail
    pin=/src/.github/scripts/tool-pin
    export npm_config_min_release_age="${COOLDOWN_DAYS}"
    npm install -g "npm@$($pin npm)"
    npm install -g "yarn@$($pin yarn)" "pnpm@$($pin pnpm)"
EOF

# Mirrors the host's delve and gotestsum installs, GOBIN drops them into
# /usr/local/bin. Test jobs restore their own Go caches.
RUN <<'EOF'
    set -euxo pipefail
    pin=/src/.github/scripts/tool-pin
    export GOBIN=/usr/local/bin
    go install "github.com/go-delve/delve/cmd/dlv@v$($pin dlv)"
    go install "gotest.tools/gotestsum@v$($pin gotestsum)"
    rm -rf /root/go /root/.cache/go-build
EOF

# Mirrors the host's Python deps and Poetry steps. pip upgrades first,
# only pip >= 26.1 honors the cooldown. mise's python has no PEP 668
# marker, so pip manages it directly. Scripts land in /opt/python/bin.
RUN <<'EOF'
    set -euxo pipefail
    export PIP_UPLOADED_PRIOR_TO="P${COOLDOWN_DAYS}D"
    python -m ensurepip --upgrade
    python -m pip install --no-cache-dir --upgrade pip
    python -m pip install --no-cache-dir requests wheel urllib3 chardet build
    python -m pip install --no-cache-dir poetry
EOF

# Let git operate on repo mounts owned by another uid (validate-local).
RUN git config --system --add safe.directory '*'

# Root would bypass the file modes some tests rely on. uid 1001 matches
# the hosted runner user that owns the mounted workspace and home, and
# passwordless sudo mirrors the hosts. The go dirs make cache volumes
# mounted there start out user-owned.
RUN useradd --uid 1001 --create-home runner \
    && echo "runner ALL=(ALL) NOPASSWD: ALL" > /etc/sudoers.d/runner \
    && mkdir -p /home/runner/go /home/runner/.cache/go-build \
    && chown -R runner:runner /home/runner/go /home/runner/.cache
USER runner
