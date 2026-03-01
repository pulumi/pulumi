# Make + GoReleaser Build and Release System

This document describes the existing build and release pipeline for Pulumi, which
uses GNU Make for local development and SDK packaging, GoReleaser for
cross-platform Go binary distribution, and GitHub Actions for CI/CD orchestration.

---

## Overview

The build system has three layers:

1. **Make** — local development builds, SDK packaging, testing, and linting
2. **GoReleaser** — cross-platform Go binary compilation and archive creation
3. **GitHub Actions** — CI/CD orchestration tying everything together

```
Developer workstation          CI/CD
─────────────────────          ──────────────────────────────────
make build                     ci-build-binaries.yml (GoReleaser)
make test_fast                 ci-build-sdks.yml     (Make)
make install                   sign.yml              (cosign)
                               ci-prepare-release.yml
                               release.yml           (NPM, PyPI, S3)
```

---

## Version Management

### Source of Truth

The canonical version lives in `sdk/.version` (e.g., `3.225.0`). It is read by
`.github/scripts/get-version`.

### Version Computation (`scripts/pulumi-version.sh`)

```bash
# Default (no PULUMI_VERSION env var): appends -dev.0
VERSION=$(.github/scripts/get-version)
VERSION="${VERSION%-*}-dev.0"

# With PULUMI_VERSION set: uses that value directly
VERSION="${PULUMI_VERSION}"
```

Language-specific transforms:

| Language | Transform | Example |
|----------|-----------|---------|
| Go / Node.js | None | `3.225.0` |
| Python | `-alpha.*` → `a<timestamp>` | `3.225.0-alpha.1` → `3.225.0a1709251234` |

`scripts/versions.sh` exports derived variables for CI:

```bash
GENERIC_VERSION=${PULUMI_VERSION}
VERSION=${PULUMI_VERSION}
PYPI_VERSION=$(scripts/pulumi-version.sh python)
DOTNET_VERSION=$(scripts/pulumi-version.sh dotnet)
GORELEASER_CURRENT_TAG=v${PULUMI_VERSION}
```

### Version Stamping

| Artifact | Mechanism |
|----------|-----------|
| Go binaries | `ldflags -X ...version.Version=v${PULUMI_VERSION}` |
| Node.js package | `scripts/reversion.js` patches `package.json` and `version.js` |
| Python wheel | `sed` patches `pyproject.toml` version before `uv build` |

---

## Make Build System

### Hierarchy

```
Makefile (top-level)
├── build/common.mk          — shared infrastructure
├── sdk/nodejs/Makefile       — Node.js SDK
├── sdk/python/Makefile       — Python SDK
└── sdk/go/Makefile           — Go SDK
```

### Shared Infrastructure (`build/common.mk`)

Key variables:

```makefile
PULUMI_ROOT  ?= ~/.pulumi-dev
PULUMI_BIN   := $(PULUMI_ROOT)/bin
GO_BUILD_TAGS ?= grpcnotrace
GO_TEST_RACE  ?= true
```

Standard targets: `ensure`, `build`, `install`, `test_fast`, `test_all`, `lint`, `dist`, `brew`.

Sub-projects (controlled by `SUB_PROJECTS`) get compound targets like
`sdk_nodejs_build`, `sdk_python_test_fast`, etc.

### Top-Level Targets

```bash
make build              # Build CLI + all SDKs
make build_proto        # Rebuild protobuf definitions
make bin/pulumi         # Build main CLI binary only
make install            # Install to ~/.pulumi-dev/bin
make test_fast          # Quick unit tests
make test_all           # Full suite including integration tests
make lint               # All linters
make clean              # Remove build artifacts
```

The main CLI binary is built with:

```bash
go build -C pkg -o ../bin/pulumi \
  -ldflags "-X github.com/pulumi/pulumi/sdk/v3/go/common/version.Version=${VERSION}" \
  github.com/pulumi/pulumi/pkg/v3/cmd/pulumi
```

### Node.js SDK (`sdk/nodejs/Makefile`)

**`make build_package`** (the packaging target):

1. `yarn run tsc` — compile TypeScript to `bin/`
2. Copy test data, proto files, vendor directory to `bin/`
3. Copy metadata: `.npmignore`, `README.md`, `LICENSE`, `dist/*` to `bin/`
4. `node ../../scripts/reversion.js bin/package.json ${VERSION}` — stamp version
5. `node ../../scripts/reversion.js bin/version.js ${VERSION}` — stamp version

**`make build_plugin`** — builds `pulumi-language-nodejs` Go binary with version ldflags
and copies `dist/pulumi-resource-pulumi-nodejs` to `../../bin/`.

In CI, `npm pack` is run in `bin/` to produce the `.tgz` tarball. The `.npmignore`
file excludes `tests/`, `tests_with_mocks/`, and `npm/testdata/`.

### Python SDK (`sdk/python/Makefile`)

**`make build_package`**:

```bash
uv run -m build --outdir ./build --installer uv
```

Uses hatchling (configured in `pyproject.toml`). Version is patched in CI by:

```bash
sed -i "s/^version = .*/version = \"${PYPI_VERSION}\"/g" pyproject.toml
```

The version is read from `lib/pulumi/_version.py` via the hatch version plugin.

Output: `build/pulumi-${PYPI_VERSION}-py3-none-any.whl`

**`make build_plugin`** — builds `pulumi-language-python` Go binary and copies
`pulumi-language-python-exec` and `dist/pulumi-resource-pulumi-python` alongside it.

### Go SDK (`sdk/go/Makefile`)

Builds `pulumi-language-go` binary with version ldflags. No packaging step needed
(Go SDK is consumed as a module, not a distributable artifact).

---

## GoReleaser Configuration (`.goreleaser.yml`)

GoReleaser v2 is used in snapshot mode for CI builds (no publishing, no validation).

### Go Binary Builds

Five builds are defined:

| ID | Binary | Source Dir | Tags |
|----|--------|-----------|------|
| `pulumi` | `pulumi` | `pkg` | `osusergo` |
| `pulumi-language-go` | `pulumi-language-go` | `sdk/go/pulumi-language-go` | `osusergo`, `grpcnotrace` |
| `pulumi-language-nodejs` | `pulumi-language-nodejs` | `sdk/nodejs/cmd/pulumi-language-nodejs` | `osusergo`, `grpcnotrace` |
| `pulumi-language-python` | `pulumi-language-python` | `sdk/python/cmd/pulumi-language-python` | `osusergo`, `grpcnotrace` |
| `pulumi-display-wasm` | `pulumi-display` | `pkg` (wasm) | `osusergo` |

All non-WASM builds target `linux`, `darwin`, `windows` × `amd64`, `arm64` (6 combinations).

Common flags:

```yaml
flags: [-trimpath]
ldflags: [-w, -s, -X ...version.Version=v{{.Env.PULUMI_VERSION}}]
env: [GO111MODULE=on]
tags: [osusergo]
```

The `tool` field points to `scripts/go-wrapper.sh`, which supports coverage and
race-detection modes via environment variables (`PULUMI_BUILD_MODE`,
`PULUMI_ENABLE_RACE_DETECTION`).

The WASM build includes a size check post-hook (≤ 77 MB).

### Archive Structure

```yaml
archives:
  - id: pulumi
    wrap_in_directory: pulumi{{ if eq .Os "windows" }}/bin{{ end }}
    format_overrides:
      - goos: windows
        formats: zip
    files:
      - src: bin/{{ .Os }}/*           # OS-specific scripts
      - src: bin/{{ .Os }}-{{ arch }}/* # Pre-built binaries
    name_template: pulumi-{{ .Tag }}-{{ .Os }}-{{ arch }}
```

Archive naming: `pulumi-v3.225.0-linux-x64.tar.gz`, `pulumi-v3.225.0-windows-x64.zip`.

Architecture mapping: Go `amd64` → archive `x64`; `arm64` unchanged.

### Archive Contents

**Unix (tar.gz, `pulumi/` prefix):**

| Source | Files |
|--------|-------|
| GoReleaser builds | `pulumi`, `pulumi-language-go`, `pulumi-language-nodejs`, `pulumi-language-python` |
| `bin/{os}/` (scripts) | `pulumi-resource-pulumi-nodejs`, `pulumi-resource-pulumi-python`, `pulumi-language-python-exec` |
| `bin/{os}-{arch}/` (external) | `pulumi-language-dotnet`, `pulumi-language-java`, `pulumi-language-yaml`, `pulumi-watch` |

**Windows (zip, `pulumi/bin/` prefix):**

Same files with `.exe` / `.cmd` extensions as appropriate. `.cmd` variants used for
`pulumi-resource-pulumi-nodejs` and `pulumi-resource-pulumi-python`.

### Pre-Release Preparation (`scripts/prep-for-goreleaser.sh`)

Populates the `bin/` directory structure consumed by GoReleaser:

1. Cleans `./bin/`
2. Copies Node.js and Python SDK entry-point scripts into `bin/{os}/`
3. Downloads `pulumi-watch` v0.1.4 from `pulumi/watchutil-rs` GitHub releases
4. Downloads language providers (dotnet v3.101.2, java v1.21.2, yaml v1.29.1) from
   their respective `pulumi/pulumi-{lang}` GitHub releases

The `local` argument restricts downloads to the host platform only.

File timestamps are normalized to the commit timestamp for reproducibility.

---

## CI/CD Workflows

### Pipeline Architecture

```
on-pr.yml / on-push.yml
  └─→ ci.yml (lint, unit tests)
  └─→ ci-build-binaries.yml (GoReleaser, per-platform matrix)
  └─→ ci-build-sdks.yml (Make: npm pack, uv build)
        └─→ sign.yml (SHA256/BLAKE3/SHA512 checksums, cosign signatures)
              └─→ ci-prepare-release.yml (draft GitHub release)
                    └─→ release.yml (publish to NPM, PyPI, S3)
```

### Binary Build (`ci-build-binaries.yml`)

Reusable workflow, called once per OS/arch combination (6 calls via matrix).

Inputs: `ref`, `os`, `arch`, `version`, `version-set`, `artifact-suffix`,
`enable-coverage`, `enable-race-detection`.

Key steps:

1. Setup Go with caching
2. Run `scripts/versions.sh` to export version env vars
3. Install GoReleaser Pro
4. Run `scripts/prep-for-goreleaser.sh local` (platform-specific)
5. Filter `.goreleaser.yml` for target OS/arch via `goreleaser-filter`
6. Run GoReleaser in snapshot mode: `goreleaser release -f - -p 5 --skip=validate --clean --snapshot`
7. Upload `goreleaser/*.tar.gz` / `*.zip` as `artifacts-cli-{os}-{arch}{suffix}`

### SDK Build (`ci-build-sdks.yml`)

Two jobs:

**Python:**
1. Setup uv + Python
2. Patch `pyproject.toml` version via `sed`
3. `make build_package`
4. Upload `sdk/python/build/*.whl` as `artifacts-python-sdk`

**Node.js:**
1. Setup Node.js + yarn
2. `make ensure && make build_package`
3. `cd sdk/nodejs/bin && npm pack`
4. Upload `sdk/nodejs/bin/*.tgz` as `artifacts-nodejs-sdk`

### Signing (`sign.yml`)

1. Downloads all build artifacts
2. Renames SDK artifacts: `*.whl` → `sdk-python-*.whl`, `*.tgz` → `sdk-nodejs-*.tgz`
3. Verifies the linux-x64 binary is not a coverage build
4. Generates checksums: SHA256 (`pulumi-{version}-checksums.txt`), BLAKE3 (`B3SUMS`), SHA512 (`SHA512SUMS`)
5. Signs all artifacts and checksums with cosign (Sigstore bundles: `*.sig`)
6. Uploads `artifacts-signatures`

### Release Preparation (`ci-prepare-release.yml`)

Creates a draft pre-release on GitHub with all signed artifacts.

### Release Distribution (`release.yml`)

Triggered when a GitHub release is published:

1. **SDKs**: Downloads `sdk-{lang}-*` from the release, runs `make publish`
   - Node.js → NPM (via `scripts/publish_npm.sh`, tag logic based on branch)
   - Python → PyPI (via `twine upload`)
2. **S3**: Uploads `pulumi-*` archives to `s3://get.pulumi.com/releases/sdk/`
3. **Dispatch**: Triggers downstream updates (Homebrew, Chocolatey, Docker, templates, docs)

### Dev Release (`ci-dev-release.yml`)

Triggered on push to master:

1. Computes alpha version: `${version}-alpha.${short_sha}`
2. Builds all binaries and SDKs
3. Signs artifacts
4. Conditionally publishes SDKs if `sdk/` files changed (detected via `git diff`)
5. Uploads signed binaries to S3

---

## Artifact Naming Conventions

### Binary Archives

```
pulumi-v{VERSION}-{os}-{arch}.{ext}
```

| Platform | Example |
|----------|---------|
| Linux x64 | `pulumi-v3.225.0-linux-x64.tar.gz` |
| Linux ARM64 | `pulumi-v3.225.0-linux-arm64.tar.gz` |
| macOS x64 | `pulumi-v3.225.0-darwin-x64.tar.gz` |
| macOS ARM64 | `pulumi-v3.225.0-darwin-arm64.tar.gz` |
| Windows x64 | `pulumi-v3.225.0-windows-x64.zip` |
| Windows ARM64 | `pulumi-v3.225.0-windows-arm64.zip` |

### SDK Packages

| SDK | Filename |
|-----|----------|
| Node.js | `pulumi-{VERSION}.tgz` (npm tarball) |
| Python | `pulumi-{PYPI_VERSION}-py3-none-any.whl` |

### GitHub Actions Artifacts

| Name | Contents |
|------|----------|
| `artifacts-cli-{os}-{arch}` | Platform archive |
| `artifacts-nodejs-sdk` | npm tarball |
| `artifacts-python-sdk` | Python wheel |
| `artifacts-signatures` | Checksums + cosign signatures |

---

## Key File Paths

```
sdk/.version                                # Version source of truth
.goreleaser.yml                             # GoReleaser configuration
build/common.mk                            # Shared Make infrastructure
scripts/pulumi-version.sh                  # Version computation
scripts/versions.sh                        # CI version exports
scripts/reversion.js                       # Node.js version stamping
scripts/go-wrapper.sh                      # GoReleaser Go build wrapper
scripts/prep-for-goreleaser.sh             # Pre-release bin/ population
scripts/get-language-providers.sh           # Download external language hosts
scripts/get-pulumi-watch.sh                # Download pulumi-watch
scripts/publish_npm.sh                     # NPM publication logic
sdk/nodejs/Makefile                        # Node.js SDK build
sdk/python/Makefile                        # Python SDK build
sdk/python/pyproject.toml                  # Python build config (hatchling)
sdk/python/lib/pulumi/_version.py          # Python version file
.github/workflows/ci-build-binaries.yml    # GoReleaser CI
.github/workflows/ci-build-sdks.yml        # SDK CI
.github/workflows/sign.yml                 # Artifact signing
.github/workflows/ci-prepare-release.yml   # Release preparation
.github/workflows/release.yml              # Release distribution
.github/workflows/ci-dev-release.yml       # Dev release
```

---

## Environment Variables

### Build-Time

| Variable | Purpose |
|----------|---------|
| `PULUMI_VERSION` | Override version (e.g., `3.225.0`) |
| `GORELEASER_CURRENT_TAG` | GoReleaser tag (e.g., `v3.225.0`) |
| `PULUMI_BUILD_MODE` | `coverage` or `normal` |
| `PULUMI_ENABLE_RACE_DETECTION` | `true` for race builds |
| `GO_BUILD_TAGS` | Go build tags (default: `grpcnotrace`) |

### Release-Time

| Variable | Purpose |
|----------|---------|
| `NODE_AUTH_TOKEN` / `NPM_TOKEN` | NPM authentication |
| `PYPI_USERNAME` / `PYPI_PASSWORD` | PyPI authentication |
| `AWS_ACCESS_KEY_ID` / `AWS_SECRET_ACCESS_KEY` | S3 upload credentials |
| `PULUMI_BOT_TOKEN` | GitHub token for PRs and dispatches |

---

## Dependency Management

| Language | Tool | Lock File | Ensure Command |
|----------|------|-----------|----------------|
| Go | Go modules | `go.sum` (per module) | `go mod download` |
| Node.js | Yarn | `yarn.lock` | `yarn install --frozen-lockfile` |
| Python | uv | `uv.lock` | `uv venv && uv sync --dev` |
| Proto | Docker | `proto/.checksum.txt` | `proto/generate.sh` |
