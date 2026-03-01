# Bazel Build and Release System

This document describes the Bazel-based build and release pipeline for Pulumi. Bazel
provides hermetic, reproducible builds with built-in cross-compilation and caching,
replacing the GoReleaser + Make pipeline for binary compilation, SDK packaging, and
release archive creation.

---

## Overview

The Bazel build system is a single unified layer that handles all build tasks:

```
Developer workstation                 CI/CD
─────────────────────                 ──────────────────────────────────────
bazel build //build/release:all       ci-build-binaries-bazel.yml (all platforms)
bazel build //sdk/nodejs:npm_tarball  ci-build-sdks-bazel.yml     (Node.js + Python)
bazel build //sdk/python:wheel        sign.yml                    (cosign)
bazel test //sdk/nodejs:unit_test     ci-prepare-release.yml
bazel test //sdk/python/...           release.yml                 (NPM, PyPI, S3)
```

---

## Module Configuration (`MODULE.bazel`)

The root `MODULE.bazel` declares all external dependencies using Bazel's bzlmod system:

### Go Toolchain

```starlark
bazel_dep(name = "rules_go", version = "0.60.0")
bazel_dep(name = "gazelle", version = "0.47.0")

go_sdk = use_extension("@rules_go//go:extensions.bzl", "go_sdk")
go_sdk.download(version = "1.25.0")

go_deps = use_extension("@gazelle//:extensions.bzl", "go_deps")
go_deps.from_file(go_work = "//:go.work")
```

Go dependencies are imported from the existing `go.work` file, keeping the Bazel and
Make builds synchronized. A patch is applied to `google.golang.org/grpc` to include
`trace_notrace.go` (gated by the `grpcnotrace` build tag) which Gazelle omits by default.

The `pulumi/esc` module depends on local `sdk/v3` and `pkg/v3` modules. Gazelle overrides
redirect these imports to local Bazel targets instead of fetching external repos.

### Node.js / TypeScript Toolchain

```starlark
bazel_dep(name = "aspect_rules_js", version = "2.9.2")
bazel_dep(name = "aspect_rules_ts", version = "3.8.5")
bazel_dep(name = "rules_nodejs", version = "6.3.4")

node = use_extension("@rules_nodejs//nodejs:extensions.bzl", "node")
node.toolchain(node_version = "20.18.3")

npm = use_extension("@aspect_rules_js//npm:extensions.bzl", "npm")
npm.npm_translate_lock(
    name = "npm",
    pnpm_lock = "//sdk/nodejs:pnpm-lock.yaml",
)
```

Uses `aspect_rules_js` for npm dependency management (via pnpm lockfile) and
`aspect_rules_ts` for TypeScript compilation. The TypeScript version is derived from
`sdk/nodejs/package.json`.

### Python Toolchain

```starlark
bazel_dep(name = "rules_python", version = "1.8.5")

python = use_extension("@rules_python//python/extensions:python.bzl", "python")
python.toolchain(python_version = "3.12")

pip = use_extension("@rules_python//python/extensions:pip.bzl", "pip")
pip.parse(
    hub_name = "pypi",
    python_version = "3.12",
    requirements_lock = "//sdk/python:requirements.txt",
)
```

### Release Packaging

```starlark
bazel_dep(name = "rules_pkg", version = "1.0.1")

release_deps = use_extension("//build/release:external_deps.bzl", "release_deps")
use_repo(release_deps, "pulumi_watch_linux_amd64", ...)
```

External binaries (pulumi-watch, language providers) are declared via a custom module
extension that registers `http_archive` repos for each platform.

---

## Bazel Configuration (`.bazelrc`)

```
build --stamp
build --workspace_status_command=scripts/bazel-workspace-status.sh
test --test_output=errors
build --incompatible_enable_cc_toolchain_resolution
build --action_env=PATH
build --strategy=GoCompilePkg=worker,sandboxed
```

Key settings:

| Setting | Purpose |
|---------|---------|
| `--stamp` | Enable version stamping in binaries and archives |
| `--workspace_status_command` | Run `bazel-workspace-status.sh` to provide `STABLE_PULUMI_VERSION` |
| `--strategy=GoCompilePkg=worker,sandboxed` | Use persistent workers for Go compilation (faster incremental builds) |
| `--action_env=PATH` | Allow CC toolchain detection from PATH |

---

## Version Management

### Workspace Status Script (`scripts/bazel-workspace-status.sh`)

```bash
VERSION=$("${SCRIPTDIR}/pulumi-version.sh")
PYPI_VERSION=$("${SCRIPTDIR}/pulumi-version.sh" python)
echo "STABLE_PULUMI_VERSION ${VERSION}"
echo "STABLE_PYPI_VERSION ${PYPI_VERSION}"
```

Produces two stable status keys consumed by downstream rules. The `STABLE_` prefix
ensures these values trigger rebuilds when they change.

### Version Stamping

| Artifact | Mechanism |
|----------|-----------|
| Go binaries | `x_defs = {"...version.Version": "{STABLE_PULUMI_VERSION}"}` on `go_binary` |
| Node.js tarball | `npm_tarball` rule replaces `version_placeholder` string with `--define PULUMI_VERSION=X` |
| Python wheel | `py_wheel` uses `version = "$(PYPI_VERSION)"` with `--define PYPI_VERSION=X` |
| Release archives | `pkg_tar`/`pkg_zip` use `{PULUMI_VERSION}` in `package_file_name` via `stamp = 1` |

### Version Override

In CI, the version is set via `--define`:

```bash
bazel build //build/release:all_archives \
  --stamp \
  --define PULUMI_VERSION="${PULUMI_VERSION}"
```

Without `--define`, the workspace status script computes a dev version from
`scripts/pulumi-version.sh` (same logic as the Make system).

---

## Go Binary Builds

### Cross-Compilation Macro (`build/release/defs.bzl`)

```starlark
PLATFORMS = [
    ("linux", "amd64"), ("linux", "arm64"),
    ("darwin", "amd64"), ("darwin", "arm64"),
    ("windows", "amd64"), ("windows", "arm64"),
]

def pulumi_release_binary(name, embed, gotags = [], x_defs = {}):
    for goos, goarch in PLATFORMS:
        go_binary(
            name = "%s_%s_%s" % (name, goos, goarch),
            embed = embed,
            goos = goos,
            goarch = goarch,
            gotags = gotags,
            gc_linkopts = ["-w", "-s"],
            x_defs = x_defs,
        )
```

Uses `goos` and `goarch` parameters on `go_binary` for cross-compilation (no need for
separate CI runners per platform). Linker options `-w -s` strip debug info, matching
GoReleaser's `-w -s` ldflags.

### Release Binary Targets (`build/release/BUILD.bazel`)

```starlark
_VERSION_X_DEFS = {
    "github.com/pulumi/pulumi/sdk/v3/go/common/version.Version": "{STABLE_PULUMI_VERSION}",
}

pulumi_release_binary(
    name = "pulumi",
    embed = ["//pkg/cmd/pulumi:pulumi_lib"],
    gotags = ["osusergo"],
    x_defs = _VERSION_X_DEFS,
)

pulumi_release_binary(
    name = "pulumi-language-go",
    embed = ["//sdk/go/pulumi-language-go:pulumi-language-go_lib"],
    gotags = ["osusergo", "grpcnotrace"],
    x_defs = _VERSION_X_DEFS,
)

pulumi_release_binary(
    name = "pulumi-language-nodejs",
    embed = ["//sdk/nodejs/cmd/pulumi-language-nodejs:pulumi-language-nodejs_lib"],
    gotags = ["osusergo", "grpcnotrace"],
    x_defs = _VERSION_X_DEFS,
)

pulumi_release_binary(
    name = "pulumi-language-python",
    embed = ["//sdk/python/cmd/pulumi-language-python:pulumi-language-python_lib"],
    gotags = ["osusergo", "grpcnotrace"],
    x_defs = _VERSION_X_DEFS,
)
```

Each `pulumi_release_binary` call generates 6 targets (one per platform), totaling
24 cross-compiled Go binaries. The `embed` attribute references the `go_library` target
for each binary's main package.

---

## Release Archive Packaging (`build/release/packaging.bzl`)

### Archive Assembly

Each platform archive contains three categories of files:

1. **Go binaries** — cross-compiled from this repo, renamed from Bazel target names
   (e.g., `pulumi_linux_amd64`) to release names (e.g., `pulumi`)
2. **Scripts** — shell/cmd entry-point scripts for Node.js and Python SDK
3. **External binaries** — pre-built binaries downloaded as `http_archive` deps

```starlark
for binary in _GO_BINARIES:
    label = ":%s_%s_%s" % (binary, goos, goarch)
    go_binary_renames[label] = "%s/%s%s" % (prefix, binary, ext)

pkg_files(
    name = "_files_go_%s_%s" % (goos, goarch),
    srcs = go_binary_srcs,
    renames = go_binary_renames,
    attributes = pkg_attributes(mode = "0755"),
)
```

### Archive Format

| Platform | Format | Internal Prefix | Bazel Output | Release Filename |
|----------|--------|-----------------|--------------|------------------|
| Linux / macOS | `.tar.gz` | `pulumi/` | `archive_linux_amd64.tar.gz` | `pulumi-v3.225.0-linux-x64.tar.gz` |
| Windows | `.zip` | `pulumi/bin/` | `archive_windows_amd64.zip` | `pulumi-v3.225.0-windows-x64.zip` |

Architecture mapping: Go `amd64` → archive `x64`; `arm64` unchanged.

The archive targets use Bazel's default output filenames (e.g., `archive_linux_amd64.tar.gz`)
rather than versioned `package_file_name` values. This is intentional: `rules_pkg` resolves
`package_file_name` variables at analysis time via `ctx.var`, which requires `--define
PULUMI_VERSION=...` to be present. Without it, `bazel test //...` (which analyzes all
targets) would fail. The CI workflow renames the files to the versioned convention
(`pulumi-v{version}-{os}-{arch}.{ext}`) at upload time.

### Special Cases

- `pulumi-watch` is absent for `windows-arm64` (no upstream binary exists)
- Windows uses `.exe` for binaries and `.cmd` for wrapper scripts

### All-Archives Target

```starlark
pulumi_release_archives(name = "archives")
```

Creates individual `archive_{goos}_{goarch}` targets plus an `all_archives` filegroup
that builds all 6 platform archives in a single `bazel build` invocation.

---

## External Dependency Downloads (`build/release/external_deps.bzl`)

A Bazel module extension downloads external binaries at repository-fetch time:

### pulumi-watch (from `pulumi/watchutil-rs`)

| Platform | Triple | Archive Format |
|----------|--------|---------------|
| linux-amd64 | `x86_64-unknown-linux-gnu` | `.tar.gz` |
| linux-arm64 | `aarch64-unknown-linux-gnu` | `.tar.gz` |
| darwin-amd64 | `x86_64-apple-darwin` | `.tar.gz` |
| darwin-arm64 | `aarch64-apple-darwin` | `.tar.gz` |
| windows-amd64 | `x86_64-pc-windows-msvc` | `.zip` |

Version: `v0.1.4`. No `windows-arm64` binary.

### Language Providers

| Provider | Version | Platforms |
|----------|---------|-----------|
| `pulumi-language-dotnet` | `v3.101.2` | All 6 (linux, darwin, windows × amd64, arm64) |
| `pulumi-language-java` | `v1.21.2` | All 6 |
| `pulumi-language-yaml` | `v1.29.1` | All 6 |

Each is registered as an `http_archive` with `exports_files` in the auto-generated
BUILD file:

```starlark
http_archive(
    name = name,
    url = url,
    build_file_content = 'exports_files(["%s"])' % binary,
)
```

Total: 23 `http_archive` repos (5 watch + 18 language providers).

---

## Node.js SDK (`sdk/nodejs/BUILD.bazel`)

### TypeScript Compilation

```starlark
ts_project(
    name = "sdk_ts",
    srcs = _ALL_TS_SRCS + _TYPE_SRCS + ["package.json"],
    declaration = True,
    out_dir = "bin",
    resolve_json_module = True,
    source_map = True,
    transpiler = "tsc",
    tsconfig = "tsconfig.bazel.json",
    deps = [":node_modules", ":proto_lib"],
)
```

Uses a Bazel-specific `tsconfig.bazel.json` that omits path-dependent settings
(like `rootDir` which differs between Make's `tsc` and Bazel's sandboxed execution).
The `out_dir = "bin"` mirrors the Make system's output location.

Proto-generated `.js` and `.d.ts` files are provided as a `js_library` dependency, and
separately copied into `bin/proto/` for runtime import resolution.

### npm Tarball (`build/release/npm.bzl`)

A custom Starlark rule is needed because the standard `npm pack` command works by
running inside a fully-assembled `node_modules`-aware directory, reading `.npmignore`,
and creating a tarball. Bazel's sandboxed execution model doesn't have that — the
compiled JS files come from `ts_project` in one location, proto files are checked into
source in another, and vendor files are in a third. There is no off-the-shelf Bazel rule
that combines "gather files from multiple targets, rename/remap paths, patch a version
string, and produce an npm-format tgz." The `npm_tarball` rule handles all of this:

```starlark

```starlark
npm_tarball(
    name = "npm_tarball",
    compiled_js = ":sdk_ts",
    extra_srcs = [
        ".npmignore", "README.md", "//:LICENSE",
        "dist/pulumi-resource-pulumi-nodejs",
        "dist/pulumi-resource-pulumi-nodejs.cmd",
    ],
    package_json = ["package.json"],
    proto_srcs = _PROTO_SRCS,
    vendor = glob(["vendor/**"]),
    version_placeholder = "3.225.0",
)
```

The rule:

1. Copies compiled JS from `ts_project` output (skipping the tsc-emitted `package.json`)
2. Copies proto source files into `proto/` subdirectory
3. Copies vendored dependencies into `vendor/` subdirectory
4. Copies metadata files (`package.json`, `.npmignore`, `README.md`, `LICENSE`, dist scripts)
5. Patches version: replaces `version_placeholder` with `--define PULUMI_VERSION=X` via
   `perl -pi -e` in `package.json` and `version.js`
6. Creates gzip'd tar with `package/` prefix (npm tarball convention)

### Tests

```starlark
mocha_bin.mocha_test(
    name = "unit_test",
    args = ["--timeout", "120000", "--require", "source-map-support/register"] + [...],
    data = [":sdk", ":node_modules", ":proto_in_bin"],
)
```

---

## Python SDK (`sdk/python/BUILD.bazel`)

### Library

```starlark
py_library(
    name = "pulumi_sdk",
    srcs = glob(["lib/pulumi/**/*.py"]),
    data = glob(["lib/pulumi/**/*.pyi", "lib/pulumi/py.typed"]),
    imports = ["lib"],
    deps = ["@pypi//grpcio", "@pypi//protobuf", ...],
)
```

### Wheel Building

The `py_wheel` rule from `rules_python` is designed for building and testing, not for
producing publishable distribution artifacts. It has two limitations:

1. It only packages `.py` files from `py_library` deps, but Pulumi's Python SDK ships
   `.pyi` type stubs and a `py.typed` marker that `py_wheel` ignores.
2. It validates the `version` string as PEP 440 at analysis time. Using
   `version = "$(PYPI_VERSION)"` with `stamp = 1` requires `--define PYPI_VERSION=...`
   to be present, which breaks `bazel test //...` (the wildcard analyzes all targets).

A two-step process works around both limitations. The base wheel is built with a fixed
placeholder version, then a custom `stamped_wheel` rule (`build/release/wheel.bzl`)
patches in the real version and injects `.pyi` files. This matches the `npm_tarball`
pattern: version is read from `--define` at build time with a fallback default.

**Step 1** — Base wheel via `py_wheel` (placeholder version):

```starlark
py_wheel(
    name = "_wheel_base",
    distribution = "pulumi",
    python_tag = "py3",
    version = "0.0.0",
    requires = [...],
    python_requires = ">=3.10",
    classifiers = [...],
    strip_path_prefixes = ["sdk/python/lib/"],
    deps = [":pulumi_sdk"],
)
```

**Step 2** — Stamp version and inject `.pyi` type stubs:

```starlark
stamped_wheel(
    name = "wheel",
    wheel = ":_wheel_base",
    type_stubs = glob(["lib/pulumi/**/*.pyi", "lib/pulumi/py.typed"]),
    version_placeholder = "0.0.0",
)
```

The `stamped_wheel` rule extracts the base wheel, renames the `.dist-info` directory,
patches `Version:` in METADATA and paths in RECORD, adds `.pyi`/`py.typed` files, and
repacks. The version comes from `--define PYPI_VERSION=X`; without it, the placeholder
`0.0.0` is kept. For alpha versions, the CI workflow converts to PEP 440 format
(e.g., `3.225.0-alpha.1` → `3.225.0a1709251234`).

### Tests

```starlark
[py_test(
    name = src.replace("lib/test/", "").replace(".py", ""),
    srcs = ["run_pytest.py"] + glob(["lib/test/**/*.py"]),
    args = ["-xvs", "sdk/python/" + src],
    main = "run_pytest.py",
    deps = [":pulumi_sdk", "@pypi//pytest", ...],
) for src in glob(["lib/test/test_*.py"], ...)]
```

---

## CI/CD Workflows

### Binary Build (`ci-build-binaries-bazel.yml`)

Single job that builds **all 6 platform archives** on one `ubuntu-latest` runner
(cross-compilation eliminates the need for per-platform runners):

```yaml
- name: Build all release archives
  run: |
    bazel build //build/release:all_archives \
      --stamp \
      --define PULUMI_VERSION="${PULUMI_VERSION}"
```

Outputs are uploaded as per-platform artifacts matching the naming convention
expected by downstream workflows:

| Artifact Name | Content |
|---------------|---------|
| `artifacts-cli-linux-amd64{suffix}` | `pulumi-*-linux-x64.tar.gz` |
| `artifacts-cli-linux-arm64{suffix}` | `pulumi-*-linux-arm64.tar.gz` |
| `artifacts-cli-darwin-amd64{suffix}` | `pulumi-*-darwin-x64.tar.gz` |
| `artifacts-cli-darwin-arm64{suffix}` | `pulumi-*-darwin-arm64.tar.gz` |
| `artifacts-cli-windows-amd64{suffix}` | `pulumi-*-windows-x64.zip` |
| `artifacts-cli-windows-arm64{suffix}` | `pulumi-*-windows-arm64.zip` |

Bazel caching is persisted via `actions/cache` on `~/.cache/bazel`.

### SDK Build (`ci-build-sdks-bazel.yml`)

Two jobs:

**Node.js:**
```yaml
- name: Build Node.js SDK tarball
  run: |
    bazel build //sdk/nodejs:npm_tarball \
      --define PULUMI_VERSION="${PULUMI_VERSION}"
```

**Python:**
```yaml
- name: Compute PyPI version
  run: |
    PYPI_VERSION=$(echo -n "${PULUMI_VERSION}" | sed "s/-alpha.*/a$(date +%s)/")

- name: Build Python SDK wheel
  run: |
    bazel build //sdk/python:wheel \
      --define PYPI_VERSION="${PYPI_VERSION}"
```

### Artifact Verification (`ci-verify-bazel.yml`)

Builds artifacts with both systems and compares them:

```
goreleaser-binaries (linux-amd64 only, suffix: -verify-gr)
goreleaser-sdks     (nodejs + python, no suffix)
bazel-binaries      (all platforms, suffix: -verify-bz)
bazel-sdks          (nodejs + python, suffix: -verify-bz)
    └─→ verify (downloads all, runs scripts/verify-bazel-artifacts.sh)
```

The verification script compares:

| Artifact Type | Comparison Strategy |
|---------------|-------------------|
| Go binaries | Structural: file type, size within 20%, version string present |
| Scripts (`.sh`, `.cmd`) | Byte-identical (`cmp`) |
| External binaries | Byte-identical (`cmp`) — same upstream downloads |
| npm tarball files | Byte-identical (skipping `.js.map` source paths) |
| Python `.py`/`.pyi` | Byte-identical from same source tree |
| Wheel `METADATA` | Field-by-field: Name, Version, Requires-Dist, Classifiers |
| Wheel `WHEEL` | Tag match required; Generator difference is a warning |
| Wheel `RECORD` | Skipped (hash ordering differs between tools) |

---

## Artifact Naming Conventions

Identical to the Make/GoReleaser system — Bazel produces archives with the same filenames
so downstream workflows (signing, release, S3 upload) work unchanged.

### Binary Archives

```
pulumi-v{VERSION}-{os}-{arch}.{ext}
```

### SDK Packages

| SDK | Build Command | Output |
|-----|---------------|--------|
| Node.js | `bazel build //sdk/nodejs:npm_tarball` | `bazel-bin/sdk/nodejs/npm_tarball.tgz` |
| Python | `bazel build //sdk/python:wheel` | `bazel-bin/sdk/python/pulumi.whl` |

---

## Key File Paths

```
MODULE.bazel                              # Root module: all external dependencies
.bazelrc                                  # Build settings (stamp, workers)
scripts/bazel-workspace-status.sh         # Version stamp values

build/release/BUILD.bazel                 # Release binary + archive + install targets
build/release/defs.bzl                    # pulumi_release_binary() macro
build/release/packaging.bzl               # pulumi_release_archives() macro
build/release/install.bzl                 # pulumi_install() macro (local dev install)
build/release/npm.bzl                     # npm_tarball rule
build/release/wheel.bzl                   # stamped_wheel rule
build/release/external_deps.bzl           # Module extension for external binaries
build/patches/grpc-grpcnotrace.patch      # Patch for gRPC grpcnotrace build tag

sdk/nodejs/BUILD.bazel                    # Node.js SDK: ts_project, npm_tarball, tests
sdk/nodejs/tsconfig.bazel.json            # TypeScript config for Bazel builds
sdk/nodejs/pnpm-lock.yaml                 # npm dependency lock (consumed by rules_js)

sdk/python/BUILD.bazel                    # Python SDK: py_library, py_wheel, tests
sdk/python/requirements.txt               # pip dependency lock (consumed by rules_python)
sdk/python/run_pytest.py                  # Test runner wrapper

.github/workflows/ci-build-binaries-bazel.yml  # Binary build CI
.github/workflows/ci-build-sdks-bazel.yml       # SDK build CI
.github/workflows/ci-verify-bazel.yml           # Artifact verification CI
scripts/verify-bazel-artifacts.sh               # Artifact comparison script
```

---

## Build Commands Reference

```bash
# Build all release archives (all 6 platforms)
bazel build //build/release:all_archives \
  --define PULUMI_VERSION=3.225.0

# Build a single platform archive
bazel build //build/release:archive_linux_amd64 \
  --define PULUMI_VERSION=3.225.0

# Build a single Go binary for one platform
bazel build //build/release:pulumi_darwin_arm64

# Build Node.js SDK tarball
bazel build //sdk/nodejs:npm_tarball \
  --define PULUMI_VERSION=3.225.0

# Build Python SDK wheel
bazel build //sdk/python:wheel \
  --define PYPI_VERSION=3.225.0

# Run Node.js SDK tests
bazel test //sdk/nodejs:unit_test

# Run Python SDK tests
bazel test //sdk/python/...

# Build everything
bazel build //build/release:all_archives //sdk/nodejs:npm_tarball //sdk/python:wheel \
  --define PULUMI_VERSION=3.225.0 \
  --define PYPI_VERSION=3.225.0
```
