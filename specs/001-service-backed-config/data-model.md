# Data Model: Service-Backed Configuration

**Feature**: 001-service-backed-config
**Date**: 2026-03-10

## Entities

### ConfigEditor (NEW — interface)

Write-focused abstraction for mutating stack-owned config. Uses existing
`config.Key` and `config.Value` types — no new value types needed.
Encryption is handled by each implementation **eagerly in `Set()`** —
the buffered state always contains encrypted/wrapped values.

| Method | Signature | Description |
|--------|-----------|-------------|
| Set | `(ctx, Key, Value, path bool) error` | Encrypt (if secret) and store a config value |
| Remove | `(ctx, Key, path bool) error` | Delete a config key |
| Save | `(ctx) error` | Flush buffered mutations |

**Location**: `pkg/cmd/pulumi/config/editor.go`

**Obtained via factory**: `NewConfigEditor(ctx, stack, ps, encrypter)`
in the same package. Switches on `stack.ConfigLocation().IsRemote`. See
research.md R8 for why this is a factory, not a `backend.Stack` method.

**Design note**: The `path bool` parameter is included because path
resolution is inseparable from `config.Map` in the local case. See
research.md R4 for rationale. The `config.Value` carries plaintext +
`secure=true` for secrets; `Set()` encrypts immediately (local via
stack secrets manager, ESC via `fn::secret`) so the buffered state
always holds valid ciphertext. See research.md R3 for rationale.

### LocalConfigEditor (NEW — struct, implements ConfigEditor)

Wraps existing `ProjectStack` + file-based save for local config.
Delegates directly to `config.Map.Set/Remove/Get` which handles all
path resolution and nested object navigation internally.

| Field | Type | Description |
|-------|------|-------------|
| stack | `backend.Stack` | Stack reference for save path |
| ps | `*workspace.ProjectStack` | In-memory config state |
| encrypter | `config.Encrypter` | For encrypting secret values before save |

**Location**: `pkg/cmd/pulumi/config/editor.go`

### escConfigEditor (NEW — struct, implements ConfigEditor)

Mutates an ESC environment definition in memory, flushes via API on Save.
Translates `config.Key` + path to ESC YAML paths under `pulumiConfig`.

| Field | Type | Description |
|-------|------|-------------|
| stack | `*cloudStack` | Cloud stack reference |
| escEnv | `string` | ESC environment name (`<project>/<stack>`) |
| envDef | `[]byte` | Mutable YAML definition |
| revision | `string` | Etag for optimistic concurrency |

**Location**: `pkg/backend/httpstate/config_editor.go`

### StackConfigLocation (EXISTING — extended context)

Already exists at `pkg/backend/stack.go:36`.

| Field | Type | Description |
|-------|------|-------------|
| IsRemote | `bool` | Whether config is stored in ESC |
| EscEnv | `*string` | ESC environment name if remote |

No schema changes needed. The `IsRemote` flag is already set by the
cloud backend when a stack has a linked ESC environment.

### Stack (EXISTING — no changes to interface)

`backend.Stack` at `pkg/backend/stack.go:43`. No new methods added.
The `ConfigEditor` is obtained via a factory function in the config
package, not via a `Stack` method (see research.md R8 — adding a
method would create an import cycle).

### ProjectStack (EXISTING — no changes)

`workspace.ProjectStack` at `sdk/go/common/workspace/project.go:1216`.
No schema changes. The `Config` field (`config.Map`) and `Environment`
field (`*Environment`) are used as-is.

## Relationships

```
backend.Stack
  ├── ConfigLocation() → StackConfigLocation{IsRemote, EscEnv}
  └── LoadRemoteConfig() → ProjectStack (existing, used by read path)

NewConfigEditor(ctx, stack, ps, encrypter) → ConfigEditor
  ├── if local  → LocalConfigEditor (wraps ProjectStack.Config + file save)
  └── if remote → escConfigEditor (wraps ESC YAML definition + API save)

Command Handler (config set/rm/set-all/rm-all)
  ├── Resolves --path flag
  ├── Creates config.Value with plaintext + secure=true (for secrets)
  ├── Calls editor.Set() (encrypts eagerly) / editor.Remove()
  └── Calls editor.Save()
```

## State Transitions

### Config Source Mode

A stack is always in exactly one mode:

```
                    stack init --remote-config
    [No Stack] ──────────────────────────────────→ [Service-Backed]
        │                                               │    ↑
        │ stack init (local)                    eject   │    │ config env init --migrate
        ↓                                               ↓    │
    [Local Config] ←────────────────────────────────────┘    │
        │                                                     │
        └─────────────────────────────────────────────────────┘
```

### Pin State

A service-backed stack has exactly one pin state:

```
    [Unpinned (latest)] ──pin <rev>──→ [Pinned to Revision]
           ↑                                    │
           │                                    │
           └──────── pin latest ────────────────┘
           │
    [Unpinned (latest)] ──pin <tag>──→ [Pinned to Tag]
           ↑                                    │
           └──────── pin latest ────────────────┘
```

When pinned (to revision or tag), mutation commands (`set`, `rm`, `edit`)
are rejected. `restore` operates on the base environment regardless of pin.
