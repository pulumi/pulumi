# Contract: ConfigEditor Interface

**Package**: `pkg/cmd/pulumi/config`
**Consumers**: `configSetCmd`, `configRmCmd`, `configSetAllCmd`, `configRmAllCmd`
**Implementations**: `LocalConfigEditor`, `escConfigEditor`

## Interface

```go
type ConfigEditor interface {
    Set(ctx context.Context, key config.Key, value config.Value, path bool) error
    Remove(ctx context.Context, key config.Key, path bool) error
    Save(ctx context.Context) error
}
```

Uses existing `config.Key` and `config.Value` types. No new value types.
For secrets, the command handler passes `config.Value` with `secure=true`
and **plaintext** in the value field. Each editor implementation encrypts
on `Save` using its backend's strategy.

## Behavioral Contract

### Set

- MUST store the value under the given key.
- If `path` is true, the key's name is treated as a property path
  (e.g., `db.host` navigates nested objects).
- If `value.Secure()` is true, the implementation encrypts before persisting:
  - `LocalConfigEditor`: encrypts via the stack's `config.Encrypter`,
    then delegates to `config.Map.Set(key, encryptedValue, path)`
  - `escConfigEditor`: wraps in `fn::secret` in the ESC YAML definition
- Mutations are buffered in memory — not persisted until `Save()`.
- Calling `Set` on the same key multiple times before `Save` uses the last value.

### Remove

- MUST delete the key from config.
- If `path` is true, removes a nested value within an object.
- No-op if the key does not exist (not an error).
- Mutations are buffered — not persisted until `Save()`.

### Save

- MUST flush all buffered mutations atomically.
- `LocalConfigEditor`: writes to `Pulumi.<stack>.yaml` via `workspace.SaveProjectStack`.
- `escConfigEditor`: calls `UpdateEnvironmentWithProject` with the modified YAML.
  MUST include the revision etag for optimistic concurrency. If the etag
  mismatches (concurrent modification), returns an error.
- After `Save`, the editor MAY be reused for another batch of mutations
  (must re-read the latest state).

## Error Conditions

| Condition | Behavior |
|-----------|----------|
| Network failure (ESC) | `Save` returns error with context |
| Etag mismatch (ESC) | `Save` returns error suggesting retry |
| Encryption failure (local) | `Set` returns error |
| Missing ESC permissions | `Save` returns permission error |
| Pinned stack | Caller checks before creating editor — editor itself does not enforce |

## Obtaining an Editor

```go
editor, err := stack.ConfigEditor(ctx, projectStack)
```

- `backend.Stack.ConfigEditor(ctx, ps)` returns the appropriate implementation.
- DIY backend: always returns `LocalConfigEditor`.
- Cloud backend: returns `escConfigEditor` when `ConfigLocation().IsRemote`,
  otherwise `LocalConfigEditor`.

## Testing Contract

Mock implementation MUST support:
- Recording all `Set`/`Remove` calls with arguments (including path flag)
- Configurable `Save` behavior (success, error, etag conflict)
- Verifying call counts and argument values
