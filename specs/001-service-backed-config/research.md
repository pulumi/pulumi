# Research: Service-Backed Configuration

**Feature**: 001-service-backed-config
**Date**: 2026-03-10

## R1: ESC SDK API Surface for Environment Mutation

**Decision**: Use `esc/cmd/esc/cli/client.Client` for environment CRUD via
the existing `httpstate` backend's `escClient` field.

**Rationale**: The backend already initializes an ESC client at
`pkg/backend/httpstate/backend.go:207` and exposes it through the
`EnvironmentsBackend` interface. The client supports:
- `CreateEnvironmentWithProject(ctx, org, project, name)` — create
- `UpdateEnvironmentWithProject(ctx, org, project, name, yaml, tag)` — update with etag
- `CheckYAMLEnvironment(ctx, org, yaml)` — validate without persisting
- `OpenYAMLEnvironment(ctx, org, yaml, duration)` — resolve for reading

For the `escConfigEditor`, we need to:
1. Load the environment definition (YAML bytes)
2. Parse and modify the `pulumiConfig` section in memory
3. Serialize back to YAML and call `UpdateEnvironmentWithProject`

The `eval.EncryptSecrets` / `eval.DecryptSecrets` functions handle secret
wrapping/unwrapping for the ESC format (`fn::secret`).

**Alternatives considered**:
- Direct REST API calls: Rejected — the ESC client already abstracts this.
- Modifying `esc.Environment` (resolved form): Rejected — the resolved
  environment is read-only; mutations must go through the YAML definition.

## R2: Optimistic Concurrency via Etags

**Decision**: Use the environment revision number as an etag for
optimistic concurrency on write operations.

**Rationale**: `UpdateEnvironmentWithProject` accepts a `tag` parameter.
When non-empty, the update succeeds only if the current revision matches.
This provides optimistic concurrency without locking. The conflict window
for CLI `config set/rm` is very short (read → modify → write), making
retry-on-conflict sufficient.

On conflict, the CLI prints a clear error: "The environment was modified
by another user. Please retry your command."

**Alternatives considered**:
- Last-write-wins (no etag): Simpler but risks silent data loss.
- Distributed locking: Over-engineered for CLI-driven workflows.

## R3: Value Type for ConfigEditor

**Decision**: OPEN — needs design-time resolution.

`config.Value` is structurally just `{value string, secure bool, object
bool, typ Type}`. The `secure` flag marks intent; the `value` field holds
whatever string you put in it. Today the command handler encrypts before
creating the Value, but nothing in the type enforces this. Two viable
options:

**Option A — Use `config.Value` directly (with delayed encryption):**
The command handler creates `config.Value` with plaintext + `secure=true`.
Each editor encrypts on `Save`:
- `LocalConfigEditor`: encrypts via stack's secrets manager
- `escConfigEditor`: wraps in `fn::secret`

Pros: No new type. Leverages existing `config.Map.Set(k, v, path)` for
the local case, which handles all nested-object/path logic internally.
Cons: A `config.Value` with `secure=true` and plaintext is an invalid
state for serialization — if accidentally saved directly, secrets leak.

**Option B — Introduce `NormalizedValue{Plaintext, Secret, Object}`:**
Separate type that explicitly represents "pre-encryption" state.

Pros: Can't accidentally serialize a plaintext secret as a `config.Value`.
Cons: New type. Must reimplement path handling outside `config.Map`.

**Key constraint**: `--path` resolution is inseparable from `config.Map`
in the local case. `config.Map.Set(k, v, path=true)` calls
`parseKeyPath` → `resource.PropertyPath` → `object.Set` internally. If
the editor takes `NormalizedValue`, path logic must be duplicated or
extracted.

**Leaning**: Option A is simpler and avoids path duplication. The risk of
accidental serialization can be mitigated by ensuring the editor is the
only path to persistence (command handlers never call `ps.Config.Set`
directly after refactor).

**Alternatives considered**:
- `resource.PropertyValue` (`sdk/go/common/resource/properties.go:76`):
  General-purpose value type with `Secret` wrapper and `V any`. Could
  work but carries engine-level semantics (Computed, Output, Dependencies)
  that don't apply to config. Heavier than needed.
- `interface{}`: Loses type safety and secret intent.

## R4: Path Resolution and the Editor Boundary

**Decision**: Include `path bool` in the editor interface. Path handling
is inseparable from `config.Map` in the local case.

**Rationale**: `config.Map.Set(k, v, path=true)` internally calls
`parseKeyPath(k)` → `resource.ParsePropertyPathStrict(k.Name())` →
`resource.PropertyPath`, then uses the internal `object` type to navigate
and set nested values. This logic is deeply embedded in `config.Map` and
not easily extracted.

For the local editor, the simplest implementation is to delegate directly
to `config.Map.Set(k, v, path)` — which already handles everything. This
means the editor must accept the `path` parameter.

For the ESC editor, the equivalent operation is translating the path into
a nested YAML path within the `pulumiConfig` section. The ESC YAML
structure mirrors config.Map's nested object model, so the translation is
straightforward: `config.Map` path segments map to YAML path segments
under `pulumiConfig`.

**Interface implication**:
```go
type ConfigEditor interface {
    Set(ctx context.Context, key config.Key, value config.Value, path bool) error
    Remove(ctx context.Context, key config.Key, path bool) error
    Save(ctx context.Context) error
}
```

**Alternatives considered**:
- Resolve paths before the editor boundary: Rejected — path resolution
  is tightly coupled to `config.Map` internals (`object.Set`, container
  creation). Extracting it would require exposing private types.
- Use `resource.PropertyPath` as the key type: Over-abstracted for this
  use case. The editor should take the same inputs the command handler
  already has (`config.Key` + `path bool`).

## R5: Conflict Detection Strategy

**Decision**: Check for conflicts at the point where config is loaded for
stack operations (`pulumi up/preview/destroy`). A conflict exists when a
stack has a service-backed link AND the local `Pulumi.<stack>.yaml`
contains meaningful config (non-empty `config:` map or environment imports).
Metadata-only fields (`encryptionsalt`, `secretsprovider`) do not trigger
conflict detection.

**Rationale**: The conflict check runs in the config loading path
(`io.go:LoadProjectStack` or the stack operation preamble). It compares:
1. `stack.ConfigLocation().IsRemote` — is this a service-backed stack?
2. Does a local config file exist with meaningful content?

If both are true, a hard error is raised with actionable guidance:
"Both service-backed and local configuration exist. Delete the local
config file to use service-backed config, or run `pulumi config env eject`
to return to local config."

**Alternatives considered**:
- Merge local and remote (last-write-wins): Rejected — ambiguous and
  error-prone. Users would not know which source is authoritative.
- Silently prefer remote: Rejected — could silently ignore local changes
  a user intended to keep.

## R6: Stack Deletion and Environment Cleanup

**Decision**: Stack deletion is handled service-side. When the Pulumi Cloud
API deletes a stack, it soft-deletes the linked ESC environment. The CLI
does not need to orchestrate this — the service handles it atomically.

**Rationale**: The service already manages the stack-environment link. Doing
cleanup client-side would create a window where the stack is deleted but
the environment is orphaned (if the CLI crashes between the two operations).
Server-side handling is atomic and handles edge cases (deletion protection,
cross-references) authoritatively.

The CLI surfaces warnings if the service reports that the environment
could not be cleaned up (e.g., deletion protection enabled).

**Alternatives considered**:
- CLI-orchestrated deletion: Rejected — creates race conditions and
  requires the CLI to understand ESC deletion protection rules.

## R7: config env (bare) Command Behavior

**Decision**: `pulumi config env` (bare, no subcommand) prints the config
source for the current stack, following the `pulumi stack` (bare) pattern.

**Rationale**: Today `config env` has no handler — it just lists subcommands.
Adding a handler that prints the config source gives users a quick way to
check whether their stack is service-backed or local, and which ESC
environment (if any) is linked. This follows the established pattern where
`pulumi stack` (bare) prints current stack info.

For service-backed stacks: prints ESC environment name + pin info.
For local stacks: prints the config file path (e.g., `Pulumi.dev.yaml`).

**Alternatives considered**:
- New `config env show` subcommand: Rejected — adds command surface when
  the bare command is available and follows existing convention.
- Print "no linked environment" for local stacks: Rejected — confusing
  for stacks with ESC environment imports.
