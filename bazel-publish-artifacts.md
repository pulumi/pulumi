# Bazel Release Build Backend

This document summarizes the Bazel-based release artifact pipeline that replaces GoReleaser (for Go binaries) and Make (for SDK packages).

## Overview

All release artifacts — 6 platform archives, an npm tarball, and a Python wheel — are built with Bazel. The signing and publishing workflows are unchanged; only the build step is replaced.

## Architecture

### Cross-Compiled Go Binaries (`build/release/defs.bzl`)

The `pulumi_release_binary` macro generates `go_binary` targets for all 6 platforms (linux/darwin/windows × amd64/arm64) from a single `go_library` embed target. Each binary uses:

- `gc_linkopts = ["-w", "-s"]` — strip debug info (matching GoReleaser)
- `gotags = ["osusergo"]` — pure-Go user lookup (no CGO)
- `x_defs` — version injection via `{STABLE_PULUMI_VERSION}` stamp variable

Four binaries are cross-compiled: `pulumi`, `pulumi-language-go`, `pulumi-language-nodejs`, `pulumi-language-python`.

### External Dependencies (`build/release/external_deps.bzl`)

A module extension (`release_deps`) registers 23 `http_archive` repositories for bundled external binaries:

| Binary | Version | Platforms |
|--------|---------|-----------|
| `pulumi-watch` | v0.1.4 | 5 (no windows-arm64) |
| `pulumi-language-dotnet` | v3.101.2 | 6 |
| `pulumi-language-java` | v1.21.2 | 6 |
| `pulumi-language-yaml` | v1.29.1 | 6 |

Archives are fetched lazily — building for one platform doesn't download all 23.

### Archive Packaging (`build/release/packaging.bzl`)

The `pulumi_release_archives` macro creates per-platform archives using `pkg_tar` (unix) and `pkg_zip` (windows) from `rules_pkg`:

- Unix archives: files under `pulumi/` prefix, `.tar.gz` format
- Windows archives: files under `pulumi/bin/` prefix, `.zip` format
- `amd64` is renamed to `x64` in filenames (matching GoReleaser convention)
- Version in filename uses `--define PULUMI_VERSION=x.y.z`

Each archive contains:
- 4 cross-compiled Go binaries (renamed to strip Bazel target suffixes)
- Language-specific scripts (`pulumi-resource-pulumi-nodejs`, etc.)
- External language provider binaries
- `pulumi-watch` (except on windows-arm64)

Build all archives: `bazel build //build/release:all_archives --stamp --define PULUMI_VERSION=x.y.z`

### Node.js SDK Tarball (`build/release/npm.bzl`)

A custom Starlark rule (`npm_tarball`) assembles the npm package:

1. Copies compiled JS from `ts_project` (strips `bin/` prefix)
2. Copies proto-generated stubs and vendored dependencies
3. Copies `package.json`, `README.md`, `LICENSE`, `.npmignore`, dist scripts
4. Patches version placeholder with `--define PULUMI_VERSION=x.y.z`
5. Creates tarball with `package/` prefix using `tar czfh` (compatible with macOS and Linux)

Build: `bazel build //sdk/nodejs:npm_tarball --define PULUMI_VERSION=x.y.z`

### Python SDK Wheel (`sdk/python/BUILD.bazel`)

Two-step wheel build using `py_wheel` from `rules_python`:

1. `_wheel_base` — `py_wheel` target with all `.py` sources, metadata, and dependencies
2. `wheel` — genrule that copies the base wheel and adds `.pyi` type stubs and `py.typed` marker via `zip`

The `.pyi` files are not included by `py_wheel` (which only collects `.py` from `srcs`), so the genrule post-processes the wheel to add them.

Build: `bazel build //sdk/python:wheel --define PYPI_VERSION=x.y.z`

## CI Workflows

### `.github/workflows/ci-build-binaries-bazel.yml`

Single job on `ubuntu-latest` that builds all 6 platform archives and uploads per-platform artifacts:

- `artifacts-cli-linux-amd64`
- `artifacts-cli-linux-arm64`
- `artifacts-cli-darwin-amd64`
- `artifacts-cli-darwin-arm64`
- `artifacts-cli-windows-amd64`
- `artifacts-cli-windows-arm64`

### `.github/workflows/ci-build-sdks-bazel.yml`

Two jobs:
- `build_node_sdk` — builds npm tarball, uploads as `artifacts-nodejs-sdk`
- `build_python_sdk` — builds Python wheel (with PEP 440 alpha version conversion), uploads as `artifacts-python-sdk`

## Files

### New Files

| File | Purpose |
|------|---------|
| `build/release/defs.bzl` | Cross-compiled `go_binary` macro |
| `build/release/BUILD.bazel` | Release binary + archive targets |
| `build/release/external_deps.bzl` | Module extension for external binary downloads |
| `build/release/packaging.bzl` | Archive creation macros (`pkg_tar`/`pkg_zip`) |
| `build/release/npm.bzl` | Custom npm tarball Starlark rule |
| `build/release/stamp_replace.bzl` | Stamp-aware version replacement rule |
| `.github/workflows/ci-build-binaries-bazel.yml` | CI workflow for CLI archives |
| `.github/workflows/ci-build-sdks-bazel.yml` | CI workflow for SDK packages |

### Modified Files

| File | Change |
|------|--------|
| `MODULE.bazel` | Added `rules_pkg`, `release_deps` extension, 23 external repos |
| `BUILD.bazel` | Added `exports_files(["LICENSE"])` |
| `pkg/cmd/pulumi/BUILD.bazel` | Added `gotags`, `gc_linkopts`, expanded visibility |
| `sdk/go/pulumi-language-go/BUILD.bazel` | Added `osusergo` tag, `gc_linkopts`, visibility |
| `sdk/nodejs/cmd/pulumi-language-nodejs/BUILD.bazel` | Added `osusergo` tag, `gc_linkopts`, visibility |
| `sdk/python/cmd/pulumi-language-python/BUILD.bazel` | Added `osusergo` tag, `gc_linkopts`, visibility |
| `sdk/nodejs/BUILD.bazel` | Added `exports_files`, `npm_tarball` target |
| `sdk/python/BUILD.bazel` | Added `exports_files`, `py_wheel` + genrule targets |
| `scripts/bazel-workspace-status.sh` | Added `STABLE_PYPI_VERSION` |

## Version Injection

Two mechanisms are used:

1. **`x_defs` with stamp variables** — For Go binaries. Uses `{STABLE_PULUMI_VERSION}` from `bazel-workspace-status.sh`, activated by `--stamp`.

2. **`--define` variables** — For archive filenames, npm tarball version patching, and Python wheel version. Uses `--define PULUMI_VERSION=x.y.z` (or `PYPI_VERSION` for Python). Required because `rules_pkg` and custom rules need the version at analysis time, which stamp variables don't support.

## Transition Strategy

Run both GoReleaser and Bazel builds in parallel for 2-3 release cycles. Compare archive file listings between the two. Gate publishing on GoReleaser initially, switch to Bazel once validated.
