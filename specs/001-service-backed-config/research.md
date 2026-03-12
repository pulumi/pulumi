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

**Decision**: Use `config.Value` directly with **eager encryption in
`Set()`**. No new value types.

**Rationale**: `config.Value` is structurally just `{value string, secure
bool, object bool, typ Type}`. The `secure` flag marks intent; the `value`
field holds whatever string you put in it. The command handler passes
plaintext + `secure=true`; each editor implementation encrypts
**immediately in `Set()`** — not lazily on `Save()`.

This is critical because `config.Value` with `secure=true` is assumed by
the serialization layer (`MarshalYAML`, `unmarshalObject`) to contain
ciphertext. If a plaintext-secure Value is accidentally stored in
`config.Map` and serialized, secrets leak silently. By encrypting eagerly
in `Set()`, the `config.Map` always contains valid ciphertext for secure
values, maintaining the existing invariant.

For `LocalConfigEditor`, `Set()` encrypts via the stack's secrets manager
and then delegates to `config.Map.Set(k, encryptedValue, path)`. For
`escConfigEditor`, `Set()` wraps the plaintext in `fn::secret` when
buffering the YAML mutation. In both cases, the `config.Value` held in
memory is always in a valid state for its backend.

**Alternatives considered**:
- `NormalizedValue{Plaintext, Secret, Object}`: Separate pre-encryption
  type. Rejected — introduces a new type and forces path handling to be
  duplicated outside `config.Map` (see R4).
- Delayed encryption on `Save()`: Rejected — leaves plaintext secrets
  in `config.Map` during the buffer window. Any accidental serialization
  (logging, error handling, concurrent save) would leak secrets.
- `resource.PropertyValue` (`sdk/go/common/resource/properties.go:76`):
  General-purpose value type. Rejected — carries engine-level semantics
  (Computed, Output, Dependencies) that don't apply to config.

## R4: Path Resolution and the Editor Boundary

**Decision**: Include `path bool` in the editor interface. Path handling
is inseparable from `config.Map` in the local case.

**Rationale**: When `--path` is used, the CLI parsing preserves the full
dotted path in the key's name. For example, `pulumi config set --path
db.host localhost` produces `Key{namespace: "myproject", name: "db.host"}`
with `path=true`. This key+flag pair flows through to `config.Map.Set`.

Inside `config.Map.Set(k, v, path=true)`, two tightly-coupled operations
happen:
1. **Parse**: `parseKeyPath(k)` calls `resource.ParsePropertyPathStrict(k.Name())`
   to split `"db.host"` into `PropertyPath{"db", "host"}`, and creates a
   new root key `Key{myproject, "db"}`.
2. **Navigate**: Uses the private `object` type to get/create nested
   containers at the remaining path segments, then stores the value.

Both `parseKeyPath` and `object` are **unexported** from
`sdk/go/common/resource/config`. The `object` type wraps `any` and
supports nested `map[string]object` / `[]object` structures, with methods
like `object.Set(prefix, path PropertyPath, new object)` for navigation.

For the local editor, the simplest implementation is to delegate directly
to `config.Map.Set(k, v, path)` — which handles all of this internally.
This means the editor must accept the `path` parameter.

For the ESC editor, the equivalent operation uses
`resource.ParsePropertyPathStrict(k.Name())` (public API) to parse the
path, then translates it to nested YAML navigation under `pulumiConfig`.
The first path segment becomes the YAML key (e.g., `myproject:db`), and
remaining segments navigate nested YAML maps.

**Interface implication**:
```go
type ConfigEditor interface {
    Set(ctx context.Context, key config.Key, value config.Value, path bool) error
    Remove(ctx context.Context, key config.Key, path bool) error
    Save(ctx context.Context) error
}
```

**Alternatives considered**:

- **Resolve paths before the editor boundary**: Rejected — the command
  handler would need to call `parseKeyPath` (unexported) or reimplement
  its logic. Then the local editor would need to reconstruct the original
  key to call `config.Map.Set`, since `config.Map` has no public API
  that accepts a pre-parsed `PropertyPath`.

- **Use `resource.PropertyPath` as the interface key type**: Rejected.
  The local editor delegates to `config.Map.Set(k, v, path)`, which
  expects the **unsplit** key with the full dotted name and `path=true`.
  There's no public API to pass `config.Map` a pre-parsed PropertyPath —
  `object.Set` is private. To use PropertyPath at the boundary, the
  local editor would need to either (a) reconstruct the original key
  from path segments (lossy roundtrip), or (b) bypass `config.Map.Set`
  and use private `object` APIs. Both are worse than the current design.
  Additionally, the `path=false` case (the common case) has no
  PropertyPath at all — the key is a literal string stored directly. A
  PropertyPath parameter would need a sentinel or separate method for
  this case.

  `PropertyPath` is the right type for *navigating* nested structures
  inside each editor implementation, but the wrong type for the interface
  boundary because the primary consumer (`config.Map`) doesn't accept it.

## R5: Conflict Detection Strategy

**Decision**: Check for conflicts at the point where config is loaded for
stack operations (`pulumi up/preview/destroy`). A conflict exists when a
stack has a service-backed link AND the local `Pulumi.<stack>.yaml`
contains meaningful config (non-empty `config:` map or environment imports).
Metadata-only fields (`encryptionsalt`, `secretsprovider`) do not trigger
conflict detection.

**Rationale**: The conflict check runs in the shared config loading path
(`pkg/cmd/pulumi/stack/io.go:LoadProjectStack`). Today this function
warns but continues when a local file exists alongside remote config
(line 72). The change upgrades this to a hard error. It compares:
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

## R8: Package Boundary for ConfigEditor

**Decision**: Define `ConfigEditor` interface in `pkg/cmd/pulumi/config`
with a factory function `NewConfigEditor(ctx, stack, ps, encrypter)`.
Do NOT add a method to `backend.Stack`.

**Rationale**: `pkg/cmd/pulumi/config` already imports `pkg/backend`. If
`backend.Stack.ConfigEditor()` returned a type defined in
`pkg/cmd/pulumi/config`, that would create an import cycle. The reverse
direction (backend returning a config-package type) is not possible.

The factory function lives in the config package alongside the interface
and switches on `stack.ConfigLocation().IsRemote`:
- If local: returns `LocalConfigEditor` (wraps `ps.Config` + file save)
- If remote: returns `escConfigEditor` (wraps ESC YAML + API save)

The `escConfigEditor` implementation also lives in the config package,
using the existing ESC client that `config.go` already imports
(`esc/cmd/esc/cli/client`). This is consistent with how config commands
already interact with ESC — the CLI package owns the orchestration, the
backend owns the transport.

**Alternatives considered**:
- `backend.Stack.ConfigEditor()` method: Creates import cycle. Would
  require moving `ConfigEditor` to `pkg/backend` or a new shared package.
- Define `ConfigEditor` in `pkg/backend`: Moves CLI-layer abstraction
  into the backend. The interface is only used by 4 CLI commands — it
  doesn't belong in the backend's public surface.
- No interface (two explicit code paths): Simpler for Phase 1 but
  doesn't scale to Phase 2. The interface is the minimal abstraction
  needed for two implementations.

## R9: Secret Exposure in Error Messages

**Decision**: Never include user-provided secret values in error messages
or guidance strings.

**Rationale**: The current `IsRemote` error guard in `configSetCmd`
(config.go:800-811) includes the plaintext value in the `pulumi env set`
guidance when `!c.Secret`. While it skips the value for `--secret` args
(uses `--secret <value>` placeholder), this pattern is fragile. A value
that looks like a secret but was passed without `--secret` would still be
echoed.

The fix: always use a placeholder in guidance strings. For `--secret`
values: `<secret-value>`. For plaintext values: include the value only
if it passed the `looksLikeSecret` check (i.e., was explicitly confirmed
as non-secret by the user).

## R10: config env add/rm/ls on Service-Backed Stacks

**Decision**: Return a hard error with an actionable YAML snippet, not
just "use config edit".

**Rationale**: `config env add <env>` is a structured operation that
users understand. Telling them to "use config edit" to add an import is
a UX regression — they'd need to know ESC YAML syntax for imports. The
error message should include the exact YAML to add:

```
config env add is not supported for service-backed stacks.

To add environment "myorg/shared/creds" as an import, add this to your
environment definition via `pulumi config edit` or `pulumi config web`:

  imports:
    - myorg/shared/creds

```

Similarly, `config env rm <env>` should show what to remove, and
`config env ls` should show how to view imports via `pulumi env get`.

**Alternatives considered**:
- Keep `config env add/rm/ls` working for service-backed stacks by
  updating ESC YAML via API: Adds scope and complexity. Deferred — can
  be added later as a convenience. The error-with-snippet approach is
  sufficient for v1.
- Plain error with no guidance: Rejected — too opaque for users who
  don't know ESC YAML format.
