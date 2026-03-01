# Plan: Verify Bazel Release Artifacts Match GoReleaser/Make

## Context

The `pgavlin/bazel` branch adds a Bazel-based release build system alongside the existing GoReleaser (Go binaries) + Make (SDK packages) pipeline. Before transitioning to Bazel, we need automated verification that Bazel-produced artifacts are equivalent to those from the existing pipeline. This plan creates a verification script and CI workflow that builds artifacts with both systems and compares them.

---

## Step 1: Add `artifact-suffix` input to Bazel CI workflows

Both Bazel workflows need an optional suffix so artifacts from Bazel and GoReleaser builds can coexist in the same CI run. The existing `ci-build-binaries.yml` already supports `artifact-suffix` (line 26). The Bazel workflows do not.

**Modify** `.github/workflows/ci-build-binaries-bazel.yml`:
- Add input `artifact-suffix` (type: string, required: false, default: `''`)
- Append `${{ inputs.artifact-suffix || '' }}` to every `upload-artifact` `name:` field (lines 62, 67, 72, 77, 84, 89 — the 6 per-platform upload steps)

**Modify** `.github/workflows/ci-build-sdks-bazel.yml`:
- Add input `artifact-suffix` (type: string, required: false, default: `''`)
- Append `${{ inputs.artifact-suffix || '' }}` to `name: artifacts-nodejs-sdk` (line 57) and `name: artifacts-python-sdk` (line 101)

---

## Step 2: Create verification script

**New file** `scripts/verify-bazel-artifacts.sh`

```
Usage: scripts/verify-bazel-artifacts.sh <goreleaser-dir> <bazel-dir> [--version VERSION]
Exit 0 = all checks passed, Exit 1 = failure
```

Both directories should contain: `*.tar.gz` / `*.zip` (platform archives), `*.tgz` (npm tarball), `*.whl` (Python wheel).

### 2a. Helpers

- `pass_check(msg)`, `fail_check(msg)`, `warn_check(msg)` — colored output, increment counters
- `extract_archive(file, dest)` — handles `.tar.gz`, `.zip`, `.tgz`, `.whl` (zip)
- `check_size_tolerance(file_a, file_b, pct)` — size within pct% tolerance
- `file_listing(dir)` — `find . -type f | sort` relative to dir

### 2b. `compare_platform_archives()`

For each platform found in both directories (match by `pulumi-*-{os}-{arch}.tar.gz/.zip` glob):

1. **File listing diff** — extract both, list files relative to `pulumi/` root, sort, diff. Any missing/extra file is a failure.

2. **Classify each file** and apply appropriate comparison:

   | Files | Strategy |
   |-------|----------|
   | `pulumi`, `pulumi-language-{go,nodejs,python}` (.exe) | **Structural**: `file` type matches (ELF/Mach-O/PE32+), size within 20%, version string present via `strings \| grep` |
   | `pulumi-resource-pulumi-nodejs`, `pulumi-resource-pulumi-python`, `pulumi-language-python-exec` (.cmd) | **Byte-identical**: `cmp` |
   | `pulumi-language-{dotnet,java,yaml}`, `pulumi-watch` (.exe) | **Byte-identical**: `cmp` (same upstream downloads) |

3. **Verify `pulumi-watch` absent** for windows-arm64 in both.

### 2c. `compare_npm_tarball()`

Find `*.tgz` in each directory. Extract both (they have `package/` prefix).

1. **File listing diff** — list files under `package/`, sort, diff. Known: Make's `npm pack` respects `.npmignore` (excludes `tests/`, `tests_with_mocks/`), Bazel never includes them. Listings should match.

2. **Per-file comparison**:

   | Files | Strategy |
   |-------|----------|
   | `package.json` | Parse, check `version` matches `--version`, diff other fields |
   | `version.js` | Check version string stamped correctly |
   | `proto/**`, `vendor/**` | **Byte-identical**: checked-in source files |
   | `.npmignore`, `README.md`, `LICENSE`, `dist/*` | **Byte-identical** |
   | `*.js`, `*.d.ts` (compiled TS) | **Diff**: should match if same tsc version. Warn (don't fail) on `.js.map` source path differences |

### 2d. `compare_python_wheel()`

Find `*.whl` in each directory. Extract both (wheels are zips).

1. **File listing diff** — normalize `.dist-info` directory name (strip version), sort, diff.

2. **Per-file comparison**:

   | Files | Strategy |
   |-------|----------|
   | `pulumi/**/*.py` | **Byte-identical**: same source tree |
   | `pulumi/**/*.pyi` | **Byte-identical**: same source tree |
   | `pulumi/py.typed` | **Present in both** |
   | `METADATA` | Compare key fields: `Name`, `Version`, `Requires-Dist`, `Requires-Python`, `Classifier`. **Warn** on `Metadata-Version` or `Description` differences |
   | `WHEEL` | Check `Tag: py3-none-any` matches. **Ignore** `Generator` line |
   | `RECORD` | **Skip** (hash listing order differs between tools) |

### 2e. Summary

Print pass/fail/warn counts. Exit 1 if any failures.

---

## Step 3: Create CI verification workflow

**New file** `.github/workflows/ci-verify-bazel.yml`

```yaml
name: Verify Bazel Artifacts
on:
  workflow_call:
    inputs:
      ref: { required: true, type: string }
      version: { required: true, type: string }
      version-set: { required: false, type: string, default: '...' }
  workflow_dispatch:
    inputs:
      ref: { default: pgavlin/bazel, type: string }
      version: { default: "3.225.0-alpha.0", type: string }
```

### Jobs

**`goreleaser-binaries`** — Call `ci-build-binaries.yml` for `linux/amd64` only (sufficient for verification, saves CI time). Use `artifact-suffix: '-verify-gr'`.

**`goreleaser-sdks`** — Call `ci-build-sdks.yml`. Artifacts are `artifacts-nodejs-sdk`, `artifacts-python-sdk` (no suffix support in this workflow — we don't modify it, we just use the default names).

**`bazel-binaries`** — Call `ci-build-binaries-bazel.yml`. Use `artifact-suffix: '-verify-bz'`.

**`bazel-sdks`** — Call `ci-build-sdks-bazel.yml`. Use `artifact-suffix: '-verify-bz'`.

**`verify`** — depends on all 4 jobs above:
1. Checkout code
2. Download artifacts into `verify/goreleaser/` and `verify/bazel/`:
   - `artifacts-cli-linux-amd64-verify-gr` → `verify/goreleaser/`
   - `artifacts-nodejs-sdk` → `verify/goreleaser/`
   - `artifacts-python-sdk` → `verify/goreleaser/`
   - `artifacts-cli-linux-amd64-verify-bz` → `verify/bazel/`
   - `artifacts-nodejs-sdk-verify-bz` → `verify/bazel/`
   - `artifacts-python-sdk-verify-bz` → `verify/bazel/`
3. Run `scripts/verify-bazel-artifacts.sh verify/goreleaser verify/bazel --version "$VERSION"`

---

## Files Summary

| File | Action | Purpose |
|------|--------|---------|
| `scripts/verify-bazel-artifacts.sh` | New | Comparison script for all artifact types |
| `.github/workflows/ci-verify-bazel.yml` | New | CI workflow that builds both systems and compares |
| `.github/workflows/ci-build-binaries-bazel.yml` | Modify | Add `artifact-suffix` input |
| `.github/workflows/ci-build-sdks-bazel.yml` | Modify | Add `artifact-suffix` input |

---

## Known Expected Differences (warnings, not failures)

1. **Go binary bytes** — different compilers/toolchains produce different output; structural comparison only
2. **`.js.map` source paths** — Bazel sandbox paths may differ from Make paths
3. **Wheel `METADATA` `Metadata-Version`** — hatchling uses 2.3, rules_python may use different default
4. **Wheel `WHEEL` `Generator`** — hatchling vs rules_python
5. **Wheel `RECORD`** — different hash ordering between tools

---

## Verification

1. Run the script locally against manually-built artifacts:
   ```bash
   # Build GoReleaser linux-amd64 archive
   ./scripts/prep-for-goreleaser.sh local
   cat .goreleaser.yml | go run github.com/t0yv0/goreleaser-filter@v0.3.0 -goos linux -goarch amd64 | goreleaser release -f - --skip=validate --clean --snapshot
   mkdir -p verify/goreleaser && cp goreleaser/*.tar.gz verify/goreleaser/

   # Build Make SDK artifacts
   cd sdk/nodejs && make build_package && cd bin && npm pack && cp *.tgz ../../../verify/goreleaser/ && cd ../../..
   cd sdk/python && make build_package && cp build/*.whl ../../verify/goreleaser/ && cd ../..

   # Build Bazel artifacts
   bazel build //build/release:archive_linux_amd64 --stamp --define PULUMI_VERSION=$(./scripts/pulumi-version.sh)
   bazel build //sdk/nodejs:npm_tarball --define PULUMI_VERSION=$(./scripts/pulumi-version.sh)
   bazel build //sdk/python:wheel --define PYPI_VERSION=$(./scripts/pulumi-version.sh python)
   mkdir -p verify/bazel
   cp bazel-bin/build/release/pulumi-*.tar.gz verify/bazel/
   cp bazel-bin/sdk/nodejs/npm_tarball.tgz verify/bazel/
   cp bazel-bin/sdk/python/pulumi.whl verify/bazel/

   # Compare
   scripts/verify-bazel-artifacts.sh verify/goreleaser verify/bazel --version $(./scripts/pulumi-version.sh)
   ```

2. Trigger `ci-verify-bazel.yml` via `workflow_dispatch` on the `pgavlin/bazel` branch and confirm the verify job passes.
