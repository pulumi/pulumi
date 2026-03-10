# Contract: New CLI Commands

**Package**: `pkg/cmd/pulumi/config`

## New Commands

### `pulumi config env` (bare)

```
Usage: pulumi config env

Print the config source for the current stack.
```

**Service-backed output**: `Environment: myorg/myproject/dev@rev42 (pinned)`
**Local output**: `Config file: Pulumi.dev.yaml`
**Flags**: `--json` — machine-readable output
**JSON schema** (service-backed):
```json
{
  "source": "esc",
  "organization": "myorg",
  "project": "myproject",
  "environment": "myproject/dev",
  "pinned": true,
  "version": 42,
  "tag": "stable"
}
```
**JSON schema** (local):
```json
{
  "source": "local",
  "file": "Pulumi.dev.yaml"
}
```

---

### `pulumi config edit`

```
Usage: pulumi config edit [flags]

Open the stack's configuration in $EDITOR.
```

**Service-backed**: Downloads ESC environment YAML, opens in `$EDITOR`,
uploads on save with etag-based conflict detection.
**Local**: Opens `Pulumi.<stack>.yaml` in `$EDITOR`.
**Flags**: `--show-secrets` — reveal plaintext values (default false)
**Errors on pinned stacks**: Rejects with "unpin first" message.

---

### `pulumi config web`

```
Usage: pulumi config web

Open the ESC environment in the Pulumi Cloud console.
```

**Service-backed**: Opens browser to ESC environment editor.
**Local**: Returns error — no web console for local config.

---

### `pulumi config pin`

```
Usage: pulumi config pin <version-or-tag>

Pin the stack's config to a specific revision or tag.
Use "latest" to unpin.
```

**Behavior**: Updates the stack's environment reference to include
`@<version>` suffix. `latest` removes the suffix (unpin).
**Local stacks**: No-op with informational message.
**Errors**: Rejects retracted revisions or deleted tags.

---

### `pulumi config restore`

```
Usage: pulumi config restore <revision>

Restore config to a previous revision (creates new revision).
```

**Behavior**: Reads content from revision N, writes as a new revision.
History is not rewritten. Pin state is unchanged.
**Concurrency**: Uses etag — fails if environment modified concurrently.
**Local stacks**: Returns error — no revision history for local config.

---

### `pulumi config env init --migrate`

```
Usage: pulumi config env init --migrate [flags]

Migrate local config to a service-backed ESC environment.
```

**Extended from existing `config env init`**. The `--migrate` flag:
1. Decrypts all secrets (fails fast if decryption fails)
2. Creates ESC environment `<project>/<stack>`
3. Writes all config values to `pulumiConfig`
4. Carries over environment imports
5. Links the stack
6. Prompts to delete local config file

**Idempotent**: If environment exists, merges (local values win with warnings).

---

### `pulumi config env eject`

```
Usage: pulumi config env eject [flags]

Return to local config files from service-backed config.
```

**Behavior**:
1. Show confirmation prompt
2. Resolve all config values from ESC
3. Prompt for local secrets provider (if secrets exist)
4. Write `Pulumi.<stack>.yaml`
5. Remove service-backed link
6. Delete ESC environment (unless `--keep-env`)

**Flags**:
- `--keep-env` — preserve ESC environment after eject
- `--secrets-provider` — specify local secrets provider
- `--non-interactive` / `--yes` — skip prompts

## Unsupported Command Guards

These commands MUST return hard errors for service-backed stacks:

| Command | Error message includes |
|---------|----------------------|
| `config env add` | "Use `pulumi config edit` or `pulumi config web`" |
| `config env rm` | "Use `pulumi config edit` or `pulumi config web`" |
| `config env ls` | "Use `pulumi config edit`, `pulumi config web`, or `pulumi env get`" |
| `config cp` | "Not supported with service-backed stacks in v1" |
| `config refresh` | "Config is read live from ESC. Use `pulumi config restore <rev>` to revert." |
| Any with `--config-file` | "Not applicable for service-backed stacks" |
