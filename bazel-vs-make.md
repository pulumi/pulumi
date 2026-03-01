# Bazel vs Make + GoReleaser: Build System Comparison

This document compares the two build and release systems for Pulumi: the existing
Make + GoReleaser pipeline and the new Bazel-based alternative.

---

## Architecture Comparison

| Aspect | Make + GoReleaser | Bazel |
|--------|-------------------|-------|
| **Go binaries** | GoReleaser (YAML config, per-platform CI runners) | `go_binary` with `goos`/`goarch` (single runner cross-compilation) |
| **Node.js SDK** | Yarn + tsc + `npm pack` | `ts_project` + custom `npm_tarball` rule |
| **Python SDK** | hatchling + `uv build` | `py_wheel` + genrule for `.pyi` injection |
| **Release archives** | GoReleaser native archiving | `pkg_tar` / `pkg_zip` from rules_pkg |
| **External binaries** | Shell scripts download at build time | `http_archive` repos fetched at analysis time |
| **Version stamping** | ldflags, `scripts/reversion.js`, `sed` | `x_defs` + workspace status + `--define` |
| **Dependency management** | go.sum, yarn.lock, uv.lock (3 tools) | MODULE.bazel + go.work, pnpm-lock.yaml, requirements.txt (1 tool) |
| **Configuration** | Makefile hierarchy + `.goreleaser.yml` | MODULE.bazel + `.bazelrc` + Starlark macros |

---

## Cross-Compilation

### Make + GoReleaser

GoReleaser supports cross-compilation natively, but the CI pipeline calls it once per
OS/arch combination using `goreleaser-filter` to subset the configuration. This requires
**6 CI runner invocations** (one per platform), typically dispatched as a GitHub Actions
matrix.

The `scripts/prep-for-goreleaser.sh local` script downloads external binaries only for
the host platform on each runner.

### Bazel

Bazel's `go_binary` rule accepts `goos` and `goarch` parameters, enabling all 6 platforms
to be built on a **single `ubuntu-latest` runner**. External binaries are declared as
`http_archive` dependencies and fetched once during repository analysis.

```starlark
go_binary(
    name = "pulumi_darwin_arm64",
    embed = ["//pkg/cmd/pulumi:pulumi_lib"],
    goos = "darwin",
    goarch = "arm64",
)
```

**Tradeoff**: The single-runner approach reduces CI complexity and runner costs, but
concentrates all work on one machine. GoReleaser's per-platform approach parallelizes
across runners.

---

## Version Stamping

### Make + GoReleaser

Three separate mechanisms:

1. **Go binaries**: `-ldflags "-X ...version.Version=v${PULUMI_VERSION}"` passed to
   `go build` or GoReleaser
2. **Node.js**: `scripts/reversion.js` patches `package.json` and `version.js` after
   TypeScript compilation
3. **Python**: `sed` patches `pyproject.toml` before `uv build`

### Bazel

Unified mechanism via workspace status:

1. **Go binaries**: `x_defs` on `go_binary` reference `{STABLE_PULUMI_VERSION}` from
   workspace status
2. **Node.js**: `npm_tarball` rule reads `PULUMI_VERSION` from `--define` and replaces
   the placeholder string in `package.json` and `version.js`
3. **Python**: `py_wheel` reads `PYPI_VERSION` from `--define` and sets the wheel version

All three flow from the same `scripts/bazel-workspace-status.sh` script, which calls
`scripts/pulumi-version.sh` — the same version computation logic used by Make.

**Tradeoff**: Bazel's approach is more uniform but requires `--define` flags in CI
commands. Make's approach uses shell scripts that feel more natural in a shell-oriented
workflow.

---

## SDK Packaging

### Node.js

| Step | Make | Bazel |
|------|------|-------|
| TypeScript compilation | `yarn run tsc` | `ts_project` (aspect_rules_ts) |
| TypeScript config | `tsconfig.json` | `tsconfig.bazel.json` (no rootDir) |
| Dependency resolution | `yarn install --frozen-lockfile` (`yarn.lock`) | `npm_translate_lock` (`pnpm-lock.yaml`) |
| Proto files | Copied to `bin/` by Makefile | Copied by `npm_tarball` rule |
| Version patching | `scripts/reversion.js` (Node.js script) | `perl -pi -e` in `npm_tarball` rule |
| Tarball creation | `npm pack` in `bin/` directory | Custom `npm_tarball` rule (tar with `package/` prefix) |

**Tradeoff**: Make uses standard npm tooling (`npm pack`) which applies `.npmignore`
rules. Bazel's custom rule explicitly selects files to include, avoiding reliance on
npmignore but requiring manual file list maintenance.

### Python

| Step | Make | Bazel |
|------|------|-------|
| Build tool | hatchling via `uv build` | `py_wheel` (rules_python) |
| `.pyi` stubs | Included automatically by hatchling | Injected via `genrule` + `zip` |
| Version source | `lib/pulumi/_version.py` via hatch plugin | `--define PYPI_VERSION=X` |
| Metadata | `pyproject.toml` (single source) | Duplicated in `py_wheel` attrs |

**Tradeoff**: hatchling reads all metadata from `pyproject.toml`, keeping it DRY. The
Bazel `py_wheel` rule requires metadata (requires, classifiers, etc.) to be duplicated
in the BUILD file. Changes to dependencies must be updated in both places.

---

## CI Pipeline

### Make + GoReleaser

```
ci-build-binaries.yml (6 runners × 1 platform each)
ci-build-sdks.yml     (2 jobs: nodejs, python)
    └─→ sign.yml → ci-prepare-release.yml → release.yml
```

- 8 total CI jobs for building
- Each binary build job: setup Go → GoReleaser → upload one archive
- Requires GoReleaser Pro license

### Bazel

```
ci-build-binaries-bazel.yml (1 runner × all platforms)
ci-build-sdks-bazel.yml     (2 jobs: nodejs, python)
    └─→ (same downstream: sign.yml → release.yml)
```

- 3 total CI jobs for building
- Binary build job: setup Bazel → build all archives → upload 6 artifacts
- No GoReleaser license needed

**Tradeoff**: Bazel reduces CI job count from 8 to 3, but the single binary build job
takes longer than any individual GoReleaser job. Total wall-clock time depends on Bazel
cache hit rates and runner performance.

---

## Caching

### Make + GoReleaser

- Go module cache (`~/go/pkg/mod`)
- Go build cache (`~/.cache/go-build`)
- Yarn cache (`~/.cache/yarn`)
- Each cached separately; no cross-language coordination

### Bazel

- Single unified cache (`~/.cache/bazel`)
- Action-level granularity: only re-executes changed actions
- Supports remote cache (not yet configured) for cross-CI sharing
- Persistent Go compilation workers reduce incremental build times

**Tradeoff**: Bazel's unified cache is more intelligent (action-level vs file-level)
but has a larger cold-cache cost. First builds are slower; subsequent builds with warm
cache are faster.

---

## Reproducibility

### Make + GoReleaser

- `scripts/prep-for-goreleaser.sh` normalizes file timestamps to commit time
- GoReleaser's `snapshot` mode does not produce deterministic archives
- `go build` output is not bitwise-reproducible across different machines
- npm and Python build tools may produce different metadata

### Bazel

- Sandbox execution isolates builds from host state
- Hermetic toolchains (Go, Node.js, Python) pinned to exact versions
- `pkg_tar`/`pkg_zip` produce deterministic archives given same inputs
- `--stamp` uses workspace status (not wall-clock time) for version values

**Tradeoff**: Bazel is significantly more reproducible by design. The Make pipeline
requires manual timestamp normalization and is sensitive to host toolchain versions.

---

## External Binary Management

### Make + GoReleaser

Shell scripts (`scripts/get-language-providers.sh`, `scripts/get-pulumi-watch.sh`)
download binaries at build time using `curl`. The `local` flag restricts downloads to
the host platform. File timestamps are manually normalized.

### Bazel

`http_archive` repos declared in `build/release/external_deps.bzl` are fetched once
during repository analysis and cached by Bazel. Each repo has an auto-generated BUILD
file with `exports_files`.

**Tradeoff**: Bazel's approach is declarative and cached; the Make approach is imperative
and re-downloads on clean builds. However, Bazel fetches all 23 repos (all platforms)
even when building for one platform, while Make's `local` flag downloads only what's
needed.

---

## Developer Experience

### Make + GoReleaser

```bash
make build          # Build everything
make test_fast      # Quick tests
make install        # Install to ~/.pulumi-dev/bin
make lint           # All linters
```

Familiar to most developers. Direct `go build`, `yarn`, `uv` commands also work.
IDE integration works out of the box since source layout matches Go module structure.

### Bazel

```bash
bazel build //build/release:all_archives    # Build everything
bazel test //sdk/nodejs:unit_test           # Node.js tests
bazel test //sdk/python/...                 # Python tests
bazel build //build/release:pulumi_darwin_arm64  # Single binary
```

Requires Bazel installation. IDE integration requires additional setup (Bazel plugins
for Go, TypeScript). The sandboxed execution model means standard `go build` may behave
differently from Bazel's `go_binary`.

**Tradeoff**: Make has lower barrier to entry and better IDE integration. Bazel offers
more precise dependency tracking and faster incremental builds but requires learning
Bazel conventions and Starlark.

---

## Maintenance Burden

### Make + GoReleaser

- `.goreleaser.yml`: YAML with Go templates, YAML anchors for DRY
- Makefiles: multiple files with `include` directives and shell commands
- Shell scripts: `prep-for-goreleaser.sh`, `reversion.js`, `versions.sh`
- GoReleaser Pro license management

### Bazel

- `MODULE.bazel`: central dependency declaration
- Starlark rules (`.bzl`): type-checked, testable, composable macros
- `BUILD.bazel` files: one per package, declarative
- Patch management for third-party BUILD files (`build/patches/`)

**Tradeoff**: Bazel replaces multiple configuration languages (Make, YAML, shell, Node.js)
with one (Starlark), but adds the complexity of managing Bazel module dependencies,
BUILD file generation (Gazelle), and third-party patches.

---

## Known Differences in Artifacts

When both systems build the same version, the following differences are expected:

| Artifact | Difference | Impact |
|----------|-----------|--------|
| Go binaries | Different bytes (different compilers/flags) | None — functionally equivalent |
| Go binary size | Within ~20% | None |
| `.js.map` source paths | Bazel sandbox paths vs Make paths | Developer debugging only |
| Wheel `METADATA` `Generator` | `hatchling` vs `rules_python` | None |
| Wheel `METADATA` `Metadata-Version` | May differ (2.3 vs 2.1) | None |
| Wheel `RECORD` | Different hash ordering | None |

All other files (`.py`, `.pyi`, `.js`, `.d.ts`, proto files, vendor files, scripts,
external binaries) are expected to be **byte-identical** between the two systems.

---

## Summary

| Dimension | Make + GoReleaser | Bazel | Winner |
|-----------|-------------------|-------|--------|
| Setup simplicity | Standard tools (make, go, yarn) | Requires Bazel install + config | Make |
| Cross-compilation | Per-platform CI runners | Single runner, built-in | Bazel |
| CI cost | 8 jobs (6 binary + 2 SDK) | 3 jobs (1 binary + 2 SDK) | Bazel |
| Cold build speed | Fast (native toolchains) | Slower (hermetic toolchain setup) | Make |
| Warm build speed | Limited caching | Action-level caching + workers | Bazel |
| Reproducibility | Manual timestamp normalization | Hermetic by design | Bazel |
| IDE integration | Works out of the box | Requires Bazel plugins | Make |
| SDK packaging | Standard tools (npm pack, hatchling) | Custom Starlark rules | Make |
| Dependency management | 3 separate tools | 1 unified system | Bazel |
| Configuration DRY-ness | Metadata duplicated across files | Metadata duplicated in BUILD + pyproject.toml | Tie |
| External binary management | Imperative shell scripts | Declarative http_archive | Bazel |
| Downstream workflow compatibility | Native | Produces identical artifact names | Tie |
