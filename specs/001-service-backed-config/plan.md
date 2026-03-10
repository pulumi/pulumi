# Implementation Plan: Service-Backed Configuration

**Branch**: `001-service-backed-config` | **Date**: 2026-03-10 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `/specs/001-service-backed-config/spec.md`

## Summary

Replace the concrete `config.Map` + `IsRemote` error-guard pattern in CLI
write commands with a `ConfigEditor` interface so that `pulumi config
set/rm/set-all/rm-all` works transparently against both local files and ESC
environments. Add new commands for migration, eject, pinning, versioning,
edit, and web access. The approach is phased: refactor local paths behind
the abstraction first (no behavior change), then wire the ESC implementation,
then add new commands.

## Technical Context

**Language/Version**: Go 1.25
**Primary Dependencies**: `github.com/pulumi/esc`, `github.com/pulumi/esc/cmd/esc/cli/client`, `github.com/pulumi/esc/eval`
**Storage**: ESC environments via Pulumi Cloud API; local `Pulumi.<stack>.yaml` files
**Testing**: Go standard testing + testify (assert/require), gotestsum in CI, mock backend via function-pointer pattern
**Target Platform**: Cross-platform CLI (macOS, Linux, Windows)
**Project Type**: CLI (part of `pulumi/pulumi` monorepo, SDK version 3.226.0)
**Performance Goals**: N/A — CLI commands; latency dominated by network RTT to Pulumi Cloud
**Constraints**: Backward compatible — existing local config behavior must not change
**Scale/Scope**: ~7 files changed in Phase 1, ~3 new files in Phase 2, ~5 new command files in Phase 3

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Status | Notes |
|-----------|--------|-------|
| I. Backward Compatibility | ✅ Pass | Phase 1 keeps all `IsRemote` error guards intact. No breaking changes to existing commands. Local config behavior unchanged. New commands are additive. |
| II. Multi-Language Correctness | ✅ Pass | CLI-only change. Config values are transparently available to all SDKs via the existing config resolution path. No SDK changes needed. |
| III. Test Discipline | ✅ Pass | Plan includes unit tests for ConfigEditor (local + ESC), behavior-preserving tests for refactored commands, integration tests against mock ESC backend. |
| IV. Minimal Complexity | ✅ Pass | `ConfigEditor` interface justified by two concrete implementations (local + ESC). `NormalizedValue` justified by encryption divergence between backends. Read paths not abstracted (already work). |
| V. Observable & Debuggable | ✅ Pass | Error messages include ESC environment name, revision context, and actionable guidance. Permission errors distinguish stack access from ESC access. |

**Code Standards**: Apache 2.0 headers, `gofumpt` formatting, `golangci-lint` passing. Changelog entries required.

## Project Structure

### Documentation (this feature)

```text
specs/001-service-backed-config/
├── plan.md              # This file
├── research.md          # Phase 0 output
├── data-model.md        # Phase 1 output
├── quickstart.md        # Phase 1 output
├── contracts/           # Phase 1 output
└── tasks.md             # Phase 2 output (/speckit.tasks)
```

### Source Code (repository root)

```text
pkg/cmd/pulumi/config/
├── config.go             # MODIFY — refactor set/rm/rm-all/set-all to use ConfigEditor
├── config_env.go         # MODIFY — add bare `config env` handler, error guards for service-backed
├── config_env_init.go    # MODIFY — add --migrate flag support
├── config_env_eject.go   # NEW — eject command
├── config_edit.go        # NEW — config edit command ($EDITOR workflow)
├── config_web.go         # NEW — config web command (browser open)
├── config_pin.go         # NEW — config pin command
├── config_restore.go     # NEW — config restore command
├── editor.go             # NEW — ConfigEditor interface, NormalizedValue, LocalConfigEditor
├── editor_test.go        # NEW — unit tests for ConfigEditor implementations
├── io.go                 # MODIFY — conflict detection logic
└── io_test.go            # MODIFY — conflict detection tests

pkg/backend/
├── stack.go              # MODIFY — add ConfigEditor method to Stack interface
├── mock.go               # MODIFY — add mock ConfigEditor
└── httpstate/
    ├── stack.go           # MODIFY — implement ConfigEditor for cloud stacks
    └── config_editor.go   # NEW — escConfigEditor implementation

pkg/backend/diy/
└── stack.go              # MODIFY — implement ConfigEditor (returns LocalConfigEditor)
```

**Structure Decision**: This feature extends the existing `pkg/cmd/pulumi/config/` package
and `pkg/backend/` interfaces. No new top-level packages. The `ConfigEditor` interface
bridges the CLI command layer and the backend layer, following the existing pattern where
`backend.Stack` owns persistence concerns.

## Complexity Tracking

No constitution violations to justify.
