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
**Scale/Scope**: ~10 files changed in Phase 1, ~3 new files in Phase 2, ~5 new command files in Phase 3

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Status | Notes |
|-----------|--------|-------|
| I. Backward Compatibility | ✅ Pass | Phase 1 keeps all `IsRemote` error guards intact. No breaking changes to existing commands. Local config behavior unchanged. New commands are additive. |
| II. Multi-Language Correctness | ✅ Pass | CLI-only change. Config values are transparently available to all SDKs via the existing config resolution path. No SDK changes needed. |
| III. Test Discipline | ✅ Pass | Plan includes unit tests for ConfigEditor (local + ESC), behavior-preserving tests for refactored commands, integration tests against mock ESC backend. |
| IV. Minimal Complexity | ✅ Pass | `ConfigEditor` interface justified by two concrete implementations (local + ESC). Uses existing `config.Value` type with eager encryption — no new value types. Read paths not abstracted (already work). |
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
├── config_env_add.go     # MODIFY — add service-backed error with YAML snippet guidance
├── config_env_init.go    # MODIFY — add --migrate flag support
├── config_env_eject.go   # NEW — eject command
├── config_edit.go        # NEW — config edit command ($EDITOR workflow)
├── config_web.go         # NEW — config web command (browser open)
├── config_pin.go         # NEW — config pin command
├── config_restore.go     # NEW — config restore command
├── editor.go             # NEW — ConfigEditor interface, LocalConfigEditor, escConfigEditor, factory
├── editor_test.go        # NEW — unit tests for ConfigEditor implementations

pkg/cmd/pulumi/stack/
├── io.go                 # MODIFY — upgrade conflict detection from warning to hard error
├── io_test.go            # MODIFY — conflict detection tests
├── stack_init.go         # MODIFY — add --remote-config prompt, unhide flag
└── stack_init_test.go    # MODIFY — test interactive prompt and flag behavior

pkg/cmd/pulumi/newcmd/
└── new.go                # MODIFY — add --remote-config flag (alias --remote-stack-config), prompt

pkg/backend/
└── mock.go               # MODIFY — add mock ConfigEditor for tests (if needed)
```

**Structure Decision**: The `ConfigEditor` interface and both implementations
(`LocalConfigEditor`, `escConfigEditor`) live in `pkg/cmd/pulumi/config/`.
The interface is NOT on `backend.Stack` — that would create an import cycle
since `pkg/cmd/pulumi/config` already imports `pkg/backend` (see research.md
R8). Instead, a factory function `NewConfigEditor()` in the config package
switches on `stack.ConfigLocation().IsRemote`.

## Current Architecture

The comment at line 813 of `config.go` captures the aspiration:

> can we implement an interface so this "just works"? Probably wishful thinking.

```
config.Map  (sdk/go/common/resource/config/map.go)
  └── type Map map[Key]Value   ← concrete map, used everywhere

ProjectStack  (sdk/go/common/workspace/project.go:1216)
  ├── Config config.Map        ← the config data
  ├── Environment              ← ESC env definition
  ├── SecretsProvider, EncryptedKey, EncryptionSalt
  └── Save(path)

Stack interface  (pkg/backend/stack.go)
  ├── ConfigLocation() StackConfigLocation
  ├── LoadRemoteConfig(ctx, project) (*ProjectStack, error)
  └── SaveRemoteConfig(ctx, ps) error
```

**Load path** (`pkg/cmd/pulumi/stack/io.go`):
- Local: `workspace.LoadProjectStack()` → reads YAML file
- Remote: `stack.LoadRemoteConfig()` → returns ProjectStack with **empty** Config

**Save path**:
- Local: `workspace.SaveProjectStack()` → writes YAML file
- Remote: `stack.SaveRemoteConfig()` → **rejects non-nil Config**, only saves env imports

**Config commands** (`pkg/cmd/pulumi/config/config.go`):
Every mutating command (set, rm, rm-all, set-all, refresh, copy) checks
`ConfigLocation().IsRemote` early and returns a hard error.

**Key Design Insight**: Read commands (`config get`, `config ls`) already work
for both backends via `openStackEnv()` + `workspace.ApplyProjectConfig()`.
Only write commands hard-fail. Therefore the abstraction is **write-focused
only** — read paths stay as-is.

## Implementation Guide

### Interface and Factory

See `contracts/config-editor.md` for the full behavioral contract. Reference
implementation:

```go
// In pkg/cmd/pulumi/config/editor.go

type ConfigEditor interface {
    Set(ctx context.Context, key config.Key, value config.Value, path bool) error
    Remove(ctx context.Context, key config.Key, path bool) error
    Save(ctx context.Context) error
}

func NewConfigEditor(ctx context.Context, stack backend.Stack,
    ps *workspace.ProjectStack, encrypter config.Encrypter,
) (ConfigEditor, error) {
    if stack.ConfigLocation().IsRemote {
        return newESCConfigEditor(ctx, stack)
    }
    return &LocalConfigEditor{stack: stack, ps: ps, encrypter: encrypter}, nil
}
```

### LocalConfigEditor

```go
type LocalConfigEditor struct {
    stack     backend.Stack
    ps        *workspace.ProjectStack
    encrypter config.Encrypter
}

func (e *LocalConfigEditor) Set(ctx context.Context, k config.Key, v config.Value, path bool) error {
    if v.Secure() {
        plaintext, _ := v.Value(config.NopDecrypter)
        encrypted, err := e.encrypter.EncryptValue(ctx, plaintext)
        if err != nil {
            return err
        }
        v = config.NewSecureValue(encrypted)
    }
    return e.ps.Config.Set(k, v, path)
}
```

### Command handler usage

```go
editor, err := NewConfigEditor(ctx, s, ps, encrypter)
if err != nil {
    return err
}
// For secrets: config.NewSecureValue(plaintext) — editor encrypts in Set()
err = editor.Set(ctx, key, v, c.Path)
if err != nil {
    return err
}
return editor.Save(ctx)
```

### Migration Phases

**Phase 1: Interface + local implementation (no behavior change)**

Keep all `IsRemote` error guards intact. Only refactor local code paths.

1. Create `editor.go` with `ConfigEditor` interface, `LocalConfigEditor`,
   and `NewConfigEditor()` factory.
2. Refactor write commands to use `ConfigEditor` for local stacks only:
   - `configSetCmd` → `editor.Set()` + `editor.Save()`
   - `configRmCmd` → `editor.Remove()` + `editor.Save()`
   - `configRmAllCmd` → loop of `editor.Remove()` + one `editor.Save()`
   - `configSetAllCmd` → loop of `editor.Set()` + one `editor.Save()`
3. Add behavior-preserving tests (secret, path, set-all cases).
4. Do NOT refactor `get`, `list`, `copy`, or `refresh`.

**Phase 2: ESC implementation**

1. Implement `escConfigEditor` in `editor.go`.
2. Update factory to return it when `IsRemote`.
3. Handle secrets (`fn::secret`), namespace translation (`pulumiConfig.`),
   path resolution, and optimistic concurrency (etag).
4. Remove `IsRemote` error checks from set/rm/rm-all/set-all handlers.
5. Upgrade conflict detection in `pkg/cmd/pulumi/stack/io.go` from warning
   to hard error.
6. Unhide `--remote-config` in `stack init` and `new`, add interactive prompt.
7. Add service-backed error guards with YAML snippets for `config env add/rm/ls`.

**Phase 3: New commands + extended operations (separate effort)**

New commands: `config edit`, `config web`, `config pin`, `config restore`,
`config env init --migrate`, `config env eject`. Also `config cp` and
`config refresh` for service-backed stacks (dedicated semantics, not via
ConfigEditor).

## Risks and Open Questions

1. **ESC API surface**: Does the ESC SDK expose environment YAML manipulation
   at the right granularity? The `esc` package imported in config.go suggests
   yes, but the exact API for modifying environment definitions needs
   verification during Phase 2.

2. **Atomicity**: `Save()` buffers all mutations and flushes atomically.
   Local: write temp file + rename. ESC: single API call per `Save()`.

3. **`config.Map` as a dependency**: The engine, deployment system, and read
   commands continue using `config.Map` directly. `ConfigEditor` is only for
   CLI write commands — no impact on the rest of the system.

4. **ESC namespace conflicts**: Setting a key inherited from an imported
   environment shadows it in the leaf environment. This is consistent with
   ESC layering semantics.

## Evaluation History

### Round 1: Claude Opus 4.6 + Codex/GPT-5.4

- Narrowed from read+write `ConfigStore` to write-only `ConfigEditor`.
- Reverted to `config.Value` — the struct is not tied to ciphertext.
- Restored `path bool` in the interface (inseparable from `config.Map`).
- Excluded `copy` and `refresh` from the initial abstraction.

### Round 2: Multi-agent critique (Go systems, security, CLI/UX, compat)

- Eager encryption in `Set()`, not lazy on `Save()` — prevents plaintext
  secret leaks via accidental serialization.
- Factory function, not `backend.Stack` method — avoids import cycle.
- Expanded scope to include `stack init`, `pulumi new`, conflict detection.
- Actionable YAML snippets in `config env add/rm` errors.
- Fixed quickstart mismatches and secret exposure in error messages.

## Complexity Tracking

No constitution violations to justify.
