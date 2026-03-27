# Pulumi Bazel Build System Status

**Last Updated:** March 2026
**Status:** Functional with selective test support

## Quick Summary

The Pulumi monorepo has a working Bazel 9.0.0 build system with 264 BUILD.bazel files across the repository. Go packages build successfully via gazelle-generated BUILD files. Tests are selectively executed based on environment and capability, with many integration and language-specific tests skipped in the Bazel environment. The build was set up over 3 commits on top of upstream `master`.

## Versions & Dependencies

### Bazel Configuration

- **Bazel Version:** 9.0.0 (from `.bazelversion`)
- **Compilation Mode:** `opt` (optimized by default)
- **Convenience Symlinks:** Enabled for ease of development

### Core Build Rules

| Dependency | Version | Purpose |
|------------|---------|---------|
| `rules_go` | 0.60.0 | Go language support |
| `gazelle` | 0.48.0 | Go dependency management and BUILD file generation |
| `rules_proto` | 7.1.0 | Protocol buffer support |
| `protobuf` | 29.3 | Protocol buffer runtime |
| `rules_python` | 1.7.0 | Python language support |

### Language Toolchains

- **Go:** 1.25.8 (downloaded and managed by `rules_go`)
- **Python:** 3.10 (default, via `rules_python`)
- **Node.js:** DISABLED — all available Node.js rules (`aspect_rules_js`, `rules_nodejs`) have `incompatible_use_toolchain_transition` which was removed in Bazel 7/8

## Module Layout

Pulumi is structured as a **multi-module Go monorepo**. Bazel treats the entire repo as a single workspace, but Go sources span multiple `go.mod` modules (`pkg/go.mod`, `sdk/go.mod`, `tests/go.mod`).

### pkg/ Module
- **Prefix:** `github.com/pulumi/pulumi/pkg/v3`
- CLI commands, deployment engine, codegen, backends, resource management
- Primary dependency source for gazelle (`go_deps.from_file(go_mod = "//pkg:go.mod")`)

### sdk/ Module
- **Prefix:** `github.com/pulumi/pulumi/sdk/v3`
- Go SDK, language host binaries, proto definitions
- Exports `sdk_files` filegroup for codegen tests (all non-test `.go` files + `.version`, `go.mod`, `go.sum`)
- Has additional dependencies declared manually in `MODULE.bazel` (`go-version`, `nxadm/tail`, `spf13/cast`, `tomb.v1`, `autogold`, `valast`, `lockfile`)

### tests/ Module
- **Prefix:** `github.com/pulumi/pulumi/tests/v3`
- Integration, acceptance, and smoke tests
- Most tests skip in Bazel; some run with reduced functionality

### Dependency Management

- Primary source: `pkg/go.mod` (via gazelle `go_deps.from_file`)
- Manual additions in `MODULE.bazel` for `sdk/go.mod`-only deps (7 extra modules)
- `github.com/pulumi/pulumi/sdk/v3` has `build_file_generation = "off"` — uses local BUILD files rather than generating from external module
- `github.com/cloudflare/circl` uses `purego` build tag to avoid assembly issues
- `github.com/pulumi/esc` has a `gazelle_override` to regenerate BUILD files with proper local SDK import resolution (12 `gazelle:resolve` directives mapping `pulumi/pulumi/sdk/v3/...` and `pkg/v3/...` imports to local targets)
- 91 external Go dependencies declared in `use_repo(go_deps, ...)` block

## Build Configuration (.bazelrc)

### Build Settings
- `--compilation_mode=opt` — optimized builds by default
- `--@rules_go//go/config:tags=purego` — avoids assembly issues in cloudflare/circl
- `--keep_going` — continue after errors to see all issues
- `--experimental_convenience_symlinks=normal` — create `bazel-bin`, `bazel-out` etc. symlinks

### Test Environment
- `BAZEL_TEST=1` — signals tests they're running under Bazel
- `BUILD_WORKSPACE_DIRECTORY` — passed through for finding actual source files
- `PATH` — passed through so tests can find external tools (go, git, curl, etc.)
- `GOWORK=off` — disabled to prevent testdata modules with their own `go.mod` + replace directives from conflicting
- `HOME`, `TEST_TMPDIR`, `PULUMI_HOME` — temp directory access for test isolation
- Git safe.directory config (`GIT_CONFIG_COUNT=1`, `GIT_CONFIG_KEY_0=safe.directory`, `GIT_CONFIG_VALUE_0=*`) — allows git access to testdata repos with different ownership
- `--strategy=TestRunner=local` — **no sandboxing**, allows system tool access (significant hermeticity tradeoff)
- `--test_output=errors` — only show output from failing tests

### Remote Caching
Commented out in `.bazelrc` — config exists for BuildBuddy (`grpcs://remote.buildbuddy.io`) but is not active.

## Test Execution Strategy

### Environment Detection

Tests detect Bazel via a standard pattern:
```go
if os.Getenv("BAZEL_TEST") != "" || os.Getenv("TEST_SRCDIR") != "" {
    t.Skip("reason...")
}
```

Two skip mechanisms are used:
1. **`TestMain` + `os.Exit(0)`** — skips entire test packages (used for packages where no tests can work)
2. **`t.Skip()`** — skips individual tests (used when some tests in a package work)

### Packages Entirely Skipped (via TestMain)

| Package | Reason |
|---------|--------|
| `sdk/go/auto` | Requires `PULUMI_ACCESS_TOKEN` / backend access |
| `sdk/go/pulumi-language-go` | Requires language host binary and complete Go toolchain |
| `sdk/nodejs/cmd/pulumi-language-nodejs` | Requires Node.js runtime |
| `sdk/nodejs/npm` | Requires npm/pnpm toolchains |
| `sdk/python/cmd/pulumi-language-python` | Requires Python runtime |
| `sdk/python/toolchain` | Requires Python runtime |
| `pkg/codegen/python` | Requires Python venv setup |

### Test Categories Skipped (via t.Skip)

1. **Language Plugin Tests** — require pre-built language host binaries
2. **Go SDK Execution Tests** (`pkg/codegen/go/gen_test.go`) — SDK not available as complete source tree in sandbox
3. **Node.js Codegen Execution** (`pkg/codegen/nodejs/gen_test.go`) — Node.js toolchain not available
4. **CLI Convert Tests** (`pkg/cmd/pulumi/convert/convert_test.go`) — require language plugins
5. **Terminal Info Tests** (`pkg/backend/display/internal/terminal/info_test.go`) — terminfo database may not be available in sandbox
6. **New Command Acceptance Tests** (`pkg/cmd/pulumi/newcmd/new_acceptance_test.go`) — require templates and backend
7. **Integration Tests** (`tests/`) — most require built binaries and backends

### Directories Excluded from Gazelle Entirely

These directories don't get BUILD files generated:
- `testdata`, `**/testdata`
- `tests/integration`, `tests/benchmarks`, `tests/roundtrip`
- `developer-docs`, `docs`, `coverage`, `changelog`
- `sdk/python`, `sdk/nodejs` (use manual BUILD files)
- `cmd/pulumi-test-language`
- `sdk/go/auto/test/errors/compilation_error` — intentionally broken code
- `sdk/python/lib/test/automation/errors/compilation_error` — intentionally broken code

### Codegen Test Batching

Large codegen program tests are split into batch1-6 + batchyaml for parallelization:
- Each batch is a separate `go_test` target in its own subdirectory
- Example: `pkg/codegen/go/gen_program_test/batch1/`
- All batches share a generated `gen_program_test.go` source file
- Each batch references shared test data via `//pkg/codegen/testing/test:testdata`, `//sdk:sdk_files`, and `//tests/testdata:codegen` filegroups
- Languages covered: Go, Node.js, Python (7 batches each = 21 batch targets)

## Test Data Resolution

### Workspace/Testdata Path: `pkg/codegen/testing/utils/testdata.go`

This is a new file that provides `TestdataPath()` — the canonical way to find codegen test data under Bazel. Resolution order:

1. `BUILD_WORKSPACE_DIRECTORY` env var (Bazel `bazel run` commands)
2. `TEST_SRCDIR/_main` (Bazel test runfiles)
3. Walk up from CWD looking for `MODULE.bazel`/`WORKSPACE`/`WORKSPACE.bazel`
4. `runtime.Caller(0)` — derive workspace from file location (non-Bazel fallback)
5. Current directory as last resort

Returns `{workspace}/tests/testdata/codegen`.

Previously, testdata paths were computed with relative paths like `filepath.Join("..", "testing", "test", "testdata")` — this breaks under Bazel's runfiles layout.

### Pulumi Binary Resolution: `sdk/go/common/testing/environment.go`

Tests find the `pulumi` CLI binary via:
1. `PULUMI_INTEGRATION_BINARY_PATH` env var
2. `RUNFILES_DIR/_main/pkg/cmd/pulumi/pulumi_/pulumi` (Bazel runfiles)
3. `TEST_SRCDIR/_main/pkg/cmd/pulumi/pulumi_/pulumi` (Bazel alt path)
4. System PATH lookup (`exec.LookPath`)

### Per-package testdataPath helpers

Several packages define local `testdataPath()` helpers for their own testdata, each with Bazel-aware resolution:
- `sdk/go/common/workspace/plugins_test.go` — resolves through symlinks for go-git compatibility
- `sdk/go/common/resource/asset_test.go`
- `sdk/go/common/resource/plugin/plugin_test.go`
- `sdk/go/common/util/gitutil/git_test.go`

### PULUMI_HOME Resolution: `sdk/go/common/workspace/paths.go`

Modified to check `TEST_TMPDIR` before the user's home directory, so Bazel tests use isolated temporary directories instead of writing to `~/.pulumi`.

## Protobuf Handling

**Strategy:** Pre-generated files checked into source control (NOT built by Bazel).

- Proto files live in `proto/pulumi/`, `proto/pulumi/codegen/`, `proto/pulumi/testing/`
- Generated Go files live in `sdk/proto/go/`, `sdk/proto/go/codegen/`, `sdk/proto/go/testing/`
- Generation is done via `proto/generate.sh` (manual, uses mise tools)
- `sdk/proto/go/BUILD.bazel` explicitly lists all `.pb.go` and `_grpc.pb.go` files
- `proto/pulumi/BUILD.bazel` has `proto_library` + `go_proto_library` rules but these are for reference/validation, not the primary build path
- `proto/google/protobuf/BUILD.bazel` exists for well-known types

## Language SDKs

### Go SDK — Fully integrated
- Built by gazelle-generated rules across ~100+ BUILD files
- Tests run with environment-based skipping for unsupported scenarios
- CLI binary (`pkg/cmd/pulumi`) built with `go_binary` and version stamped via `x_defs`
- Language host binaries (`pulumi-language-go`, `pulumi-language-nodejs`, `pulumi-language-python`) also built

### Python SDK — Manual BUILD files, limited test support
- `sdk/python/lib/pulumi/BUILD.bazel` — `py_library` with globbed `.py` files
- Dependencies: protobuf, grpcio, dill, semver, pyyaml, debugpy (via `@pip//` labels)
- `sdk/python/requirements_lock.txt` — flat requirements file for pip extension
- Tests defined in `sdk/python/lib/test/BUILD.bazel` with `tags = ["manual"]` (not run by default)
- Python toolchain Go code (`sdk/python/toolchain/`) builds but tests skip in Bazel
- Several testdata-only BUILD files for automation test fixtures

### Node.js SDK — Pre-compiled artifacts only
- Node.js rules disabled (incompatible with Bazel 9, comment in MODULE.bazel)
- Only Go code under `sdk/nodejs/` is built: `sdk/nodejs/cmd/pulumi-language-nodejs/` and `sdk/nodejs/npm/`
- TypeScript/JavaScript treated as pre-compiled
- Tests for npm package entirely skip in Bazel

## Complete List of Modified Go Source Files

### New files (not in upstream)
| File | Purpose |
|------|---------|
| `pkg/codegen/testing/utils/testdata.go` | Workspace/testdata path resolution helper |
| `pkg/codegen/python/main_test.go` | TestMain to skip all Python codegen tests |
| `sdk/go/auto/main_test.go` | TestMain to skip all auto API tests |
| `sdk/nodejs/npm/main_test.go` | TestMain to skip all npm tests |
| `sdk/python/toolchain/main_test.go` | TestMain to skip all Python toolchain tests |

### Modified source files (non-test)
| File | Change |
|------|--------|
| `sdk/go/common/workspace/paths.go` | `GetPulumiHomeDir()` checks `TEST_TMPDIR` for Bazel test isolation |
| `sdk/go/auto/cmd.go` | `NewPulumiCommand()` checks `RUNFILES_DIR`/`TEST_SRCDIR` for Bazel binary |
| `sdk/go/common/testing/environment.go` | `resolvePulumiPath()` checks Bazel runfiles paths |
| `pkg/codegen/testing/test/program_driver.go` | `testdataPath` uses `utils.TestdataPath()` instead of relative path |
| `pkg/codegen/testing/test/sdk_driver.go` | `defaultDir` uses `utils.TestdataPath()` instead of relative path |
| `pkg/codegen/testing/test/go.go` | `checkGo()` skips Go toolchain checks in Bazel |
| `pkg/codegen/testing/test/nodejs.go` | `checkNodeJS()` and `TypeCheckNodeJSPackage()` skip in Bazel |
| `pkg/codegen/testing/test/python.go` | `checkPython()` skips in Bazel |
| `tests/testutil/test_main_util.go` | `SetupPulumiBinary()` and `InstallPythonProvider()` handle Bazel |

### Modified test files
| File | Change |
|------|--------|
| `pkg/backend/display/internal/terminal/info_test.go` | Skip when terminfo unavailable |
| `pkg/cmd/pulumi/convert/convert_test.go` | `skipInBazel()` helper for plugin tests |
| `pkg/cmd/pulumi/newcmd/new_acceptance_test.go` | Skip acceptance tests in Bazel |
| `pkg/cmd/pulumi/newcmd/new_ai_test.go` | Skip in Bazel |
| `pkg/cmd/pulumi/newcmd/new_test.go` | Skip in Bazel |
| `pkg/codegen/go/gen_program_test.go` | Testdata path changes |
| `pkg/codegen/go/gen_test.go` | Skip Go SDK execution tests in Bazel |
| `pkg/codegen/nodejs/gen_program_test.go` | Testdata path changes |
| `pkg/codegen/nodejs/gen_test.go` | Skip Node.js tests in Bazel |
| `pkg/codegen/pcl/binder_schema_test.go` | Uses `utils.TestdataPath()` |
| `pkg/codegen/pcl/binder_test.go` | Uses `utils.TestdataPath()` |
| `pkg/codegen/python/utilities_test.go` | Uses `utils.TestdataPath()` |
| `pkg/codegen/schema/docs_test.go` | Uses `utils.TestdataPath()` |
| `pkg/codegen/schema/loader_schema_test.go` | Testdata path changes |
| `pkg/codegen/schema/schema_test.go` | Uses `utils.TestdataPath()` |
| `pkg/codegen/utilities_test.go` | Uses `utils.TestdataPath()` |
| `pkg/importer/hcl2_test.go` | Uses `utils.TestdataPath()` |
| `pkg/testing/integration/program_test.go` | Skip in Bazel |
| `pkg/workspace/plugin_test.go` | Skip git operations in Bazel |
| `sdk/go/auto/local_workspace_test.go` | Bazel-aware path resolution |
| `sdk/go/auto/stack_test.go` | Bazel adjustments |
| `sdk/go/common/resource/asset_test.go` | `testdataPath()` helper for Bazel |
| `sdk/go/common/resource/plugin/plugin_test.go` | `testdataPath()` helper for Bazel |
| `sdk/go/common/tail/tail_test.go` | Bazel-aware testdata resolution |
| `sdk/go/common/util/gitutil/git_test.go` | `testdataPath()` helper for Bazel |
| `sdk/go/common/workspace/plugins_test.go` | `testdataPath()` with symlink resolution |
| `sdk/go/internal/gen-pux-applyn/main_test.go` | Bazel-aware path resolution |
| `sdk/go/pulumi-language-go/language_test.go` | Bazel-aware path resolution |
| `sdk/go/pulumi-language-go/main_test.go` | TestMain skip all in Bazel |
| `sdk/nodejs/cmd/pulumi-language-nodejs/main_test.go` | TestMain skip all in Bazel |
| `sdk/python/cmd/pulumi-language-python/main_test.go` | TestMain skip all in Bazel |
| `tests/examples/examples_acceptance_test.go` | Skip in Bazel |
| `tests/examples/examples_test.go` | Skip in Bazel |
| `tests/history/history_test.go` | Skip certain tests in Bazel |
| `tests/login/login_test.go` | Skip in Bazel |
| `tests/login/oidc_login_test.go` | Skip in Bazel |
| `tests/performance/performance_test.go` | Skip in Bazel |
| `tests/preview_only/preview_only_test.go` | Skip in Bazel |
| `tests/smoke/smoke_test.go` | `isBazelTest()` helper, selective skipping |
| `tests/stack/stack_test.go` | Skip in Bazel |

## Patches

### `patches/circl_x25519.patch`
Modifies `cloudflare/circl`'s `dh/x25519/BUILD.bazel` to exclude AMD64 assembly files (`curve_amd64.go`, `curve_amd64.h`, `curve_amd64.s`) and use generic Go implementations (`curve_generic.go`, `curve_noasm.go`). This is needed because the assembly include paths don't work correctly under Bazel's build environment.

## Architecture Decisions and Rationale

### Why no sandboxing (`--strategy=TestRunner=local`)
Many tests invoke external tools like `go`, `git`, `curl`, and the `pulumi` binary itself. Bazel's default sandboxing would prevent access to these tools. The tradeoff is reduced hermeticity — tests can see system state and aren't fully reproducible.

### Why `GOWORK=off`
The repo uses `go.work` for development, but several testdata directories have their own `go.mod` files with `replace` directives. `GOWORK=off` prevents Go from trying to use the workspace `go.work` file, which would conflict with these replace directives.

### Why the SDK dependency is `build_file_generation = "off"`
`github.com/pulumi/pulumi/sdk/v3` is both an external Go module dependency (used by `pkg/`) and a local source tree (in `sdk/`). Setting `build_file_generation = "off"` tells gazelle not to generate BUILD files for the external module, instead using the local BUILD files in `sdk/`.

### Why codegen tests are split into batches
The codegen program tests run hundreds of individual test cases. Bazel can parallelize separate test targets but runs tests within a single `go_test` sequentially. Splitting into 7 batches (batch1-6 + batchyaml) per language allows better parallelization.

### Why Python SDK uses manual BUILD files
Python's import structure and test setup (virtualenvs, mocking) don't map cleanly to gazelle's auto-generation. Manual BUILD files give explicit control over imports, deps, and test isolation.

## Known Limitations

1. **No language plugins in test sandbox** — most integration tests can't run
2. **No Python virtual environment support** — Python codegen execution tests skip
3. **No backend connectivity** — automation API and integration tests skip
4. **Proto generation not integrated** — still uses external `proto/generate.sh`
5. **Node.js rules incompatible with Bazel 9** — no JS/TS build support
6. **Tests run without sandboxing** — reduced hermeticity, results depend on host system
7. **Testdata path resolution is fragile** — multiple bespoke `testdataPath()` helpers across packages rather than one standard approach
8. **Git testdata repos need symlink resolution** — go-git doesn't work with Bazel's symlinked runfiles

## Optimization Opportunities

1. **Consolidate testdata path helpers** — Replace per-package `testdataPath()` functions with a single shared helper (extend `pkg/codegen/testing/utils/testdata.go`)
2. **Use `local = True` on tests selectively** — instead of globally disabling sandbox, mark only tests that need system tools
3. **Build language plugins as Bazel targets** — building `pulumi-language-go`, `pulumi-language-python`, `pulumi-language-nodejs` as `go_binary` targets (already done!) could enable more tests via `data` dependencies
4. **Enable remote caching** — BuildBuddy config is already commented out in `.bazelrc`; enabling would significantly speed up CI
5. **Reduce test skipping** — Some tests that skip due to "bazel environment" could likely work if the right binaries/data were provided via `data` attributes

## Recommendations for Next Steps

1. **Stabilize after rebase** — After rebasing to latest `master`, re-run `gazelle` and fix any new build/test errors
2. **Enable remote caching** — Quick win for CI speed with BuildBuddy or similar
3. **Wire up language host binaries in tests** — Use `data = ["//pkg/cmd/pulumi"]` etc. in test targets to make integration tests work
4. **Upgrade Node.js rules** — Monitor `aspect_rules_js` / `rules_js` for Bazel 9 compatibility
5. **Integrate proto generation** — Use `go_proto_library` rules to generate protos as part of the build rather than checking them in
6. **CI integration** — Add a Bazel CI job alongside the existing Make-based CI
7. **Split into clean commits** — Current work is in 3 large commits; splitting into logical units (config, BUILD files, source modifications) would make review easier
