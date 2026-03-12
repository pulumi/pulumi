# Feature Specification: Service-Backed Configuration

**Feature Branch**: `001-service-backed-config`
**Created**: 2026-03-10
**Status**: Draft
**Input**: CLI design and API design documents for service-backed configuration

## Scope

### What this feature is

Service-backed configuration stores a Pulumi stack's configuration in a
Pulumi ESC environment managed by Pulumi Cloud, instead of in a local
`Pulumi.<stack>.yaml` file. Existing `pulumi config` commands work
transparently against the backing environment. New commands manage the
link, versioning, migration, and rollback. The CLI flag name for
enabling this feature is `--remote-config`; in this spec, that flag
refers specifically to service-backed configuration.

### Supported backends

Service-backed configuration requires **Pulumi Cloud** as the backend.
Self-hosted backends and the local (filestate) backend are not supported.
The CLI MUST return a clear error when `--remote-config` is used with an
unsupported backend.

### Non-goals (out of scope for v1)

- **Custom environment naming.** The ESC environment name is always
  `<project>/<stack-name>`. Linking to an arbitrary environment is not
  supported in v1.
- **Team permission propagation.** When `--teams` is passed to
  `stack init`, permissions are not automatically granted on the created
  ESC environment. Users set ESC permissions separately.
- **Environment resolution map.** Recording the full resolution tree of
  imported environments and their versions during `pulumi up` is a
  separate enhancement that benefits all ESC-backed stacks.
- **Default org policy.** An org-level setting to default all new stacks
  to service-backed config is deferred.
- **`pulumi config cp` with service-backed stacks.** `config cp` does not
  support service-backed stacks as source or destination in v1. If either
  stack is service-backed, the command returns an error.
- **`--copy-config-from` with service-backed stacks.** If passed alongside
  `--remote-config` (or if service-backed config is selected interactively),
  `stack init` returns an error explaining this is not yet supported.
- **`--path` flag and ESC section targeting.** `--path` retains its
  current meaning (nested navigation within a config value). For v1, all
  `config set/rm` operations target `pulumiConfig`. A new flag (e.g.,
  `--section`) for targeting other ESC YAML sections such as
  `environmentVariables` is deferred.
- **`--rev` flag for reading other config versions.** Deferred.
- **Running `pulumi preview` against a different revision.** Deferred.
- **Retroactive enablement on existing stacks without local config.**
  In v1, service-backed config is enabled at stack creation
  (`stack init --remote-config`) or via migration from existing local
  config (`config env init --migrate`). A "fresh link" flow for stacks
  with no local config file is deferred to a follow-up.
- **`--draft` flag on mutation commands.** The `--draft` flag for creating
  ESC change requests via `config set`, `config rm`, `config rm-all`,
  `config set-all`, and `config edit` is deferred beyond v1. For v1,
  mutations apply directly to the backing ESC environment.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Create a New Stack with Service-Backed Config (Priority: P1)

A developer creates a new Pulumi stack and opts into service-backed
configuration so that the stack's config is stored in a Pulumi ESC
environment managed by Pulumi Cloud rather than in a local file. This
eliminates config drift across machines and enables versioning
and centralized access control.

**Why this priority**: This is the foundational entry point. Without the
ability to create service-backed stacks, no other service-backed
features are usable.

**Independent Test**: Create a stack with `pulumi stack init --remote-config`,
verify no local config file is created, verify the ESC environment exists
with the correct naming convention, and verify `pulumi config set/get`
works against the remote environment.

**Acceptance Scenarios**:

1. **Given** a user is logged into Pulumi Cloud, **When** they run `pulumi stack init dev`, **Then** they are prompted to choose between service-backed and local configuration.
2. **Given** a user runs `pulumi stack init dev --remote-config`, **When** the stack is created, **Then** an ESC environment `<project>/dev` is created with initialized `pulumiConfig` and `environmentVariables` sections.
3. **Given** a user runs `pulumi stack init dev --remote-config`, **When** the stack is created, **Then** no local config file is created.
4. **Given** a user runs `pulumi stack init dev --remote-config`, **When** the stack is created, **Then** the backing environment is linked to the stack.
5. **Given** a non-interactive session (CI/CD, `--non-interactive`, no TTY), **When** `pulumi stack init dev` is run without `--remote-config`, **Then** the CLI defaults to local configuration without prompting.
6. **Given** a user runs `pulumi new` interactively, **When** the template creates a stack, **Then** the user is prompted to choose service-backed or local config, and `--remote-config` is supported as a flag.
7. **Given** a user runs `pulumi new --yes` (non-interactive), **When** the stack is created, **Then** the CLI defaults to local config unless `--remote-config` is also passed.
7a. **Given** a user runs `pulumi new` with a template that prompts for config values (e.g. `aws:region`), **When** service-backed config is selected, **Then** the template config values are saved to the ESC environment's `pulumiConfig` section — not to a local `Pulumi.<stack>.yaml` file. Secret template values are wrapped in `fn::secret`.
8. **Given** a user is logged into a self-hosted or local (filestate) backend, **When** they run `pulumi stack init dev --remote-config`, **Then** the CLI returns an error explaining that service-backed config requires Pulumi Cloud.
9. **Given** a user runs `pulumi stack init dev --remote-config` and an ESC environment `<project>/dev` already exists, **When** the service detects the collision, **Then** the CLI returns a clear error indicating the environment already exists.
10. **Given** the service does not yet support service-backed configuration for this organization, **When** a user runs `pulumi stack init dev --remote-config`, **Then** the CLI surfaces the service's rejection clearly (e.g., "Service-backed configuration is not yet available for this organization").
11. **Given** a user runs `pulumi new --remote-stack-config`, **When** the template creates a stack, **Then** the flag is accepted as a hidden alias for `--remote-config` and service-backed configuration is enabled.
12. **Given** a user runs `pulumi stack init dev --remote-config --secrets-provider passphrase`, **When** the command is evaluated, **Then** it fails with a clear error explaining that local secrets providers cannot be used with service-backed configuration during stack creation.
13. **Given** a user runs `pulumi stack init dev --remote-config --copy-config-from other-stack`, **When** the command is evaluated, **Then** it fails with a clear error explaining that `--copy-config-from` is not yet supported with service-backed configuration.

---

### User Story 2 - Read and Write Config on Service-Backed Stacks (Priority: P1)

A developer uses the familiar `pulumi config set`, `pulumi config get`,
`pulumi config rm`, and `pulumi config` (list) commands and they work
transparently against the backing ESC environment. The experience is
identical to local config for basic operations.

**Why this priority**: The core value proposition is transparency —
existing commands must work seamlessly before any new commands matter.

**Independent Test**: On a service-backed stack, run `config set foo bar`,
`config get foo`, `config set --secret dbpass hunter2`, `config rm foo`,
`config` (list), and verify all values are stored in and read from the
ESC environment.

**Acceptance Scenarios**:

1. **Given** a service-backed stack, **When** a user runs `pulumi config set aws:region us-west-2`, **Then** the value is written to the `pulumiConfig` section of the ESC environment.
2. **Given** a service-backed stack with config values, **When** a user runs `pulumi config get aws:region`, **Then** the resolved value is returned from the ESC environment.
3. **Given** a service-backed stack, **When** a user runs `pulumi config set --secret dbPassword s3cret`, **Then** the value is stored as an ESC secret (encrypted at rest).
4. **Given** a service-backed stack, **When** a user runs `pulumi config` (list), **Then** all `pulumiConfig` values are displayed with a header indicating the config source (ESC environment name, revision, and tag if pinned).
5. **Given** a service-backed stack, **When** a user runs `pulumi config rm aws:region`, **Then** the key is removed from the ESC environment.
6. **Given** a service-backed stack, **When** a user runs `pulumi config set-all` or `pulumi config rm-all`, **Then** multiple values are set or removed in a single environment update.
7. **Given** a service-backed stack (as source or destination), **When** a user runs `pulumi config cp`, **Then** the command fails with a clear error explaining that `config cp` does not yet support service-backed stacks.
8. **Given** a service-backed stack, **When** the user runs a `pulumi config` command with `--config-file`, **Then** the command fails with a clear error explaining that custom local config files are not applicable to service-backed stacks.
9. **Given** a user with stack read/write access but no ESC write permission on the backing environment, **When** they run `pulumi config set foo bar`, **Then** the CLI returns a permission error explaining that ESC environment write access is required.
10. **Given** a user with stack read access but no ESC read permission, **When** they run `pulumi config get foo`, **Then** the CLI returns a permission error explaining that ESC environment read access is required.

---

### User Story 3 - Deploy and Destroy with Service-Backed Config (Priority: P1)

A developer runs `pulumi up`, `pulumi preview`, or `pulumi destroy` on
a service-backed stack and the engine resolves configuration from the
ESC environment. Conflict detection prevents ambiguous states where both
local and remote config exist.

**Why this priority**: Deployment and teardown are the critical path. If
stack operations don't work correctly with service-backed config, the
feature is unusable.

**Independent Test**: Set config on a service-backed stack, run
`pulumi preview`, `pulumi up`, and `pulumi destroy`, verify the config
values are available to the program, and verify conflict detection when
a local config file also exists.

**Acceptance Scenarios**:

1. **Given** a service-backed stack with config values, **When** a user runs `pulumi up`, **Then** the program receives the correct config values resolved from the ESC environment.
2. **Given** a service-backed stack AND a local `Pulumi.<stack>.yaml` with config values exists, **When** a user runs `pulumi up`, `pulumi preview`, or `pulumi destroy`, **Then** a hard error is returned explaining the conflict and how to resolve it.
3. **Given** a local `Pulumi.<stack>.yaml` that contains only metadata fields such as `encryptionsalt` or `secretsprovider`, and does not contain a non-empty `config:` map or local environment imports, **When** a user runs `pulumi up`, **Then** no conflict error is raised.
4. **Given** a service-backed stack and a local `Pulumi.<stack>.yaml` that contains local environment imports but no local `config:` entries, **When** the user runs `pulumi up`, `pulumi preview`, or `pulumi destroy`, **Then** a hard error is returned explaining the configuration conflict and how to resolve it.
5. **Given** a user with a CLI version that does not support service-backed config, **When** they run `pulumi up` on a service-backed stack, **Then** the service rejects the update with a clear error asking them to upgrade.

---

### User Story 4 - Migrate Existing Stack to Service-Backed Config (Priority: P3)

A developer with an existing stack using local config files migrates to
service-backed configuration. All config values and environment imports
are carried over to the new ESC environment.

**Why this priority**: Migration is the path for existing users. Without
it, service-backed config is only available for new stacks, limiting
adoption.

**Independent Test**: Create a stack with local config containing
several values and environment imports, run
`pulumi config env init --migrate`, verify all values are in the new ESC
environment, and verify local operations work post-migration.

**Acceptance Scenarios**:

1. **Given** a stack with local config (`Pulumi.dev.yaml`) containing 5 config values and 2 environment imports, **When** the user runs `pulumi config env init --migrate`, **Then** the ESC environment is created with all config values under `pulumiConfig`, environment imports are carried over, and the user is prompted to delete the local file.
2. **Given** a migration that partially failed (environment created but link not stored), **When** the user retries `pulumi config env init --migrate`, **Then** the migration resumes from where it left off (merges into existing environment) rather than failing with a conflict.
3. **Given** a stack that already uses service-backed config, **When** the user runs `pulumi config env init --migrate`, **Then** an error is returned indicating the stack is already service-backed.
4. **Given** a stack using passphrase or KMS encryption, **When** the user runs `--migrate`, **Then** secrets are decrypted with the current provider and re-encrypted via ESC.
5. **Given** an ESC environment `<project>/<stack>` already exists (from a previous attempt or external creation), **When** the user runs `pulumi config env init --migrate`, **Then** the local stack's config values are merged into the existing environment, conflicting `pulumiConfig` keys are overwritten by the local values, and the CLI warns about keys that were overwritten.
6. **Given** a successful migration, **When** the user declines the prompt to delete the local config file, **Then** the migration is still considered complete (stack is service-backed), and the local file remains. Conflict detection will produce a hard error on the next `pulumi up` until the local file is removed.

---

### User Story 5 - Eject from Service-Backed Config (Priority: P2)

A developer decides to return to local config files. They eject from
service-backed configuration, and all current config values are written
to a local `Pulumi.<stack>.yaml` file.

**Why this priority**: Reversibility is a guiding principle. Users must
be able to leave service-backed config without data loss.

**Independent Test**: Set up a service-backed stack with config values,
run `pulumi config env eject`, verify a local config file is created
with all values, and verify the service-backed link is removed.

**Acceptance Scenarios**:

1. **Given** a service-backed stack with config values, **When** the user runs `pulumi config env eject`, **Then** the CLI shows a confirmation prompt listing the actions to be taken.
2. **Given** a service-backed stack with config values, **When** the user confirms `pulumi config env eject`, **Then** resolved config values are written to `Pulumi.<stack>.yaml`.
3. **Given** a service-backed stack with secret config values, **When** the user confirms `pulumi config env eject`, **Then** the CLI prompts for a local secrets provider so those values can be re-encrypted locally.
4. **Given** a service-backed stack with config values, **When** the user confirms `pulumi config env eject`, **Then** the service-backed link is removed.
5. **Given** a service-backed stack with config values and no cleanup blockers, **When** the user confirms `pulumi config env eject`, **Then** the ESC environment is deleted by default.
6. **Given** a service-backed stack that contains secret config values, **When** the user runs `pulumi config env eject --non-interactive` without `--secrets-provider`, **Then** the command fails with a clear error explaining that a local secrets provider must be specified for non-interactive eject.
7. **Given** a service-backed stack that contains secret config values, **When** the user runs `pulumi config env eject --non-interactive --secrets-provider passphrase`, **Then** the eject completes without prompting and secret values are re-encrypted using the specified local secrets provider.
8. **Given** a service-backed stack whose ESC environment has been externally deleted, **When** the user runs `pulumi config env eject`, **Then** the stale link is removed with a warning that no values could be exported (rather than failing because the environment no longer exists).
9. **Given** a user who wants to keep the ESC environment after ejecting, **When** they run `pulumi config env eject --keep-env`, **Then** the environment is preserved and a note is printed confirming it was kept.
10. **Given** a non-interactive session, **When** `pulumi config env eject --non-interactive` is run, **Then** the eject proceeds without prompting (environment is deleted by default unless `--keep-env` is passed).
11. **Given** a service-backed stack where the environment has deletion protection enabled, **When** the user runs `pulumi config env eject`, **Then** the eject completes (local file written, link removed) but the environment is preserved with a message explaining deletion protection prevented cleanup.
12. **Given** a service-backed stack where environment deletion fails for any reason (permissions, network), **When** the user runs eject, **Then** the eject still completes (local file written, link removed) and a warning is printed that the environment could not be deleted and may need manual cleanup.

---

### User Story 6 - Pin, Restore, and Version Config (Priority: P2)

A developer pins their stack's config to a specific ESC environment
version or tag for stability, and can restore config to a previous
revision if a bad change is made.

**Why this priority**: Versioning and rollback are key advantages of
service-backed config over local files. They enable safe config
management workflows. In v1, `latest` is reserved as the unpin keyword
for `pulumi config pin`.

**Independent Test**: Make several config changes to create multiple
revisions, pin to a specific revision, verify `pulumi up` uses the
pinned version, restore to a previous revision, and verify the content
is correct.

**Acceptance Scenarios**:

1. **Given** a service-backed stack, **When** the user runs `pulumi config pin 42`, **Then** the stack uses revision 42 of its ESC environment for all operations.
2. **Given** a service-backed stack, **When** the user runs `pulumi config pin stable`, **Then** the stack resolves config via the `stable` tag.
3. **Given** a service-backed stack pinned to a revision, **When** the user runs `pulumi config pin latest`, **Then** the stack is unpinned and uses the latest revision.
4. **Given** a service-backed stack, **When** the user runs `pulumi config restore 7`, **Then** a new revision is created with the content from revision 7 (history is not rewritten).
5. **Given** a stack pinned to revision 42, **When** the user runs `pulumi config set foo bar`, **Then** the CLI rejects the change with an error asking the user to unpin first.
6. **Given** a stack pinned to a retracted revision or deleted tag, **When** the user runs `pulumi up`, **Then** a clear error is shown explaining the situation with recovery guidance.
7. **Given** a stack pinned to a tag (e.g., `stable`), **When** the user runs `pulumi config set foo bar`, **Then** the CLI rejects the change with an error asking the user to unpin first.
8. **Given** a stack pinned to a tag (e.g., `stable`), **When** the user runs `pulumi config edit`, **Then** the CLI rejects the edit with an error asking the user to unpin first.
9. **Given** a service-backed stack, **When** the user runs `pulumi config restore <revision>` and the backing environment is modified by another user before the restore is applied, **Then** the command fails with a clear error explaining that the environment changed and that the user should retry.

---

### User Story 7 - Edit, View, and Inspect Service-Backed Config (Priority: P2)

A developer uses convenience commands to bulk-edit config in their
editor, open the ESC environment in the browser, or inspect the linked
environment name. These commands are the primary way to manage the
backing environment directly and are referenced by error messages from
unsupported commands (e.g., `config cp` directs users to
`config edit`).

**Why this priority**: `config env` (bare) is the only way to discover
the linked environment name. `config edit` provides bulk-editing of the
full ESC environment. `config web` provides quick access to the ESC
console. These are required for a complete user experience.

**Independent Test**: On a service-backed stack, run `pulumi config env`
to see the linked environment, `pulumi config web` to open the browser,
and `pulumi config edit` to bulk-edit in `$EDITOR`.

**Acceptance Scenarios**:

1. **Given** a service-backed stack, **When** the user runs `pulumi config env` (bare), **Then** the linked ESC environment name is printed (with pin info if pinned).
2. **Given** a local-config stack, **When** the user runs `pulumi config env` (bare), **Then** the local config file path is printed (e.g., `Pulumi.dev.yaml`).
3. **Given** a service-backed stack, **When** the user runs `pulumi config env --json`, **Then** machine-readable output is returned with source type, environment, organization, project, name, pinned, and version fields.
4. **Given** a service-backed stack, **When** the user runs `pulumi config web`, **Then** the Pulumi Cloud console opens to the ESC environment editor page.
5. **Given** a service-backed stack, **When** the user runs `pulumi config edit`, **Then** the ESC environment definition is downloaded, opened in `$EDITOR`, and uploaded on save with conflict detection.
6. **Given** a local-config stack, **When** the user runs `pulumi config edit`, **Then** the local `Pulumi.<stack>.yaml` is opened in `$EDITOR`.
7. **Given** a service-backed stack, **When** the user opens `pulumi config edit` and the backing environment is modified by another user before save, **Then** the save is rejected with a clear conflict error and guidance to retry with the latest version.

---

### User Story 8 - Delete a Service-Backed Stack (Priority: P2)

A developer deletes a service-backed stack and the linked ESC
environment is cleaned up automatically, preventing orphaned
environments and naming collisions on future stack recreation.

**Why this priority**: Without automatic cleanup, deleted stacks leave
orphaned ESC environments that block recreation with the same name
(409 Conflict).

**Independent Test**: Create a service-backed stack, delete it with
`pulumi stack rm`, verify the ESC environment is soft-deleted, and
verify recreating a stack with the same name succeeds.

**Acceptance Scenarios**:

1. **Given** a service-backed stack, **When** the user runs `pulumi stack rm`, **Then** the linked ESC environment is soft-deleted alongside the stack.
2. **Given** a service-backed stack whose ESC environment is deletion-protected or referenced by other non-deleted environments, **When** the user runs `pulumi stack rm`, **Then** the stack is deleted but the environment is preserved with a warning explaining why it was kept.
3. **Given** a soft-deleted service-backed stack, **When** the stack is undeleted, **Then** the linked ESC environment is restored as well.

---

### Edge Cases

- What happens when `--config-file` flag is used with a service-backed stack? The CLI MUST return an error since custom config file paths are not applicable.
- What happens when `pulumi config env add`, `pulumi config env rm`, or `pulumi config env ls` are run against a service-backed stack? These commands operate on the ESC environment's `imports` section directly, adding/removing/listing imports with etag-based concurrency.
- What happens when `pulumi config refresh` is run on a service-backed stack? The CLI MUST print a deprecation warning explaining that config is already read live from ESC and there is no local file to refresh.
- What happens when two CI agents simultaneously pin a stack to different versions? The last write wins in v1; the CLI does not detect or prevent this.
- What happens when a team member hasn't pulled latest after a migration and still has the local config file? Conflict detection produces a hard error guiding them to delete the local file.
- What happens when `config restore` fails because the environment was modified by another user between read and write? The CLI MUST surface a user-friendly error suggesting a retry.
- What happens when a tag-pinned stack's config is mutated? The CLI MUST reject the mutation with an error asking the user to unpin first (same behavior as revision-pinned stacks).
- What happens when `--remote-config` and `--secrets-provider` are both passed? The CLI MUST return an error — service-backed config uses ESC-managed secrets and does not accept a local secrets provider.

### Unsupported commands and flags on service-backed stacks

The following return hard errors when run against a service-backed stack.
For local-config stacks, their behavior is unchanged.

| Command / Flag | Error guidance |
|----------------|----------------|
| `pulumi config cp` | Not supported with service-backed stacks as source or destination in v1 |
| `pulumi config refresh` | Deprecated; config is already read live from ESC |
| `--config-file` | Not applicable for service-backed stacks |
| `--copy-config-from` (on `stack init`) | Not supported alongside `--remote-config` in v1 |
| `--secrets-provider` (on create) | Not valid with `--remote-config` during stack creation; service-backed config uses ESC-managed secrets |

## Requirements *(mandatory)*

### Functional Requirements

**Stack creation and prompting:**

- **FR-001**: The CLI MUST support creating stacks with service-backed configuration via `--remote-config` flag on both `pulumi stack init` and `pulumi new`.
- **FR-002**: The CLI MUST prompt users to choose between service-backed and local config during interactive `stack init` and `new` when logged into Pulumi Cloud.
- **FR-003**: In non-interactive mode (including `pulumi new --yes` and no-TTY sessions), the CLI MUST default to local config unless `--remote-config` is explicitly passed.
- **FR-024**: ESC environments created for service-backed stacks MUST follow the naming convention `<project>/<stack-name>`.
- **FR-025**: The ESC environment MUST be initialized with `pulumiConfig` and `environmentVariables` sections at creation time.
- **FR-027**: The `--remote-stack-config` flag on `pulumi new` MUST remain supported as a hidden alias (not shown in help output), with `--remote-config` as the visible, preferred name.
- **FR-028**: `--copy-config-from` on `stack init` MUST return a clear error when used alongside `--remote-config` or when service-backed config is selected interactively.
- **FR-029**: `--remote-config` on unsupported backends (self-hosted, local/filestate) MUST return a clear error.

**Config read/write operations:**

- **FR-004**: `pulumi config set/get/rm/set-all/rm-all` MUST work transparently against the ESC environment for service-backed stacks. Mutations MUST use optimistic concurrency (etag-based); if the environment was modified between read and write, the command fails with a clear error and the user retries.
- **FR-005**: `pulumi config` (list) MUST display a source annotation header showing the ESC environment name, revision, and tag (if pinned).
- **FR-006**: `pulumi config cp` MUST return a clear error when either the source or destination stack is service-backed. For local-config stacks, behavior is unchanged.

**Deployment and conflict detection:**

- **FR-007**: `pulumi up`, `pulumi preview`, and `pulumi destroy` MUST resolve config from the ESC environment for service-backed stacks.
- **FR-008**: The CLI MUST raise a hard error when both a service-backed link and meaningful local stack configuration exist for the same stack. Meaningful local stack configuration includes a non-empty local `config:` map or local environment imports. Metadata-only fields such as `encryptionsalt` and `secretsprovider` do not trigger conflict detection.

**Migration and eject:**

- **FR-009**: The CLI MUST support migrating local config to service-backed via `pulumi config env init --migrate`, carrying over config values and environment imports.
- **FR-010**: Migration MUST be idempotent and safe to retry after partial failures. If the target ESC environment already exists, migration merges the local stack config into that environment. When the same `pulumiConfig` key exists in both places, the local stack's value wins and the CLI MUST warn about any overwritten keys. Migration MUST decrypt all secrets before creating or modifying the ESC environment; if any secret cannot be decrypted (e.g., forgotten passphrase, inaccessible KMS key), the command MUST fail with no partial state.
- **FR-011**: The CLI MUST support ejecting from service-backed config via `pulumi config env eject`, writing resolved values to a local file and removing the service link.
- **FR-012**: On eject, the CLI MUST show a confirmation prompt, prompt for a local secrets provider when secrets must be re-encrypted, and delete the ESC environment by default (skip deletion if deletion-protected or referenced by other environments). The `--keep-env` flag MUST suppress environment deletion. In non-interactive mode, eject MUST proceed without prompting only when all required inputs are already provided. If the stack contains secrets and eject is run non-interactively, `--secrets-provider` MUST be provided or the command MUST fail with a clear error. If environment deletion fails, eject MUST still complete (local file written, link removed) with a warning.

**Pinning and versioning:**

- **FR-013**: The CLI MUST support pinning to a specific revision or tag via `pulumi config pin <version-or-tag>` and unpinning via `pulumi config pin latest`. In v1, `latest` is a reserved CLI keyword for unpinning and cannot be used as a tag name with this command. For local-config stacks, the command MUST be a no-op with a message explaining it only applies to service-backed stacks.
- **FR-014**: The CLI MUST reject mutation commands (`set`, `rm`, `edit`) on pinned stacks (both revision-pinned and tag-pinned). Mutations are only allowed when the stack is unpinned (using latest).
- **FR-015**: The CLI MUST support restoring to a previous revision via `pulumi config restore <revision>`, creating a new revision (not rewriting history). Restore always operates on the base environment regardless of any pin. The pin remains unchanged after restore.
- **FR-026**: The CLI MUST reject pinning to retracted revisions or deleted tags with a clear error.

**Edit, view, and inspect:**

- **FR-016**: The CLI MUST provide `pulumi config web` to open the ESC environment in the browser. For local-config stacks, the command MUST return an error.
- **FR-017**: The CLI MUST provide `pulumi config edit` to open the ESC environment definition in `$EDITOR` with conflict detection on save. Secrets are hidden by default (`fn::secret` wrappers show ciphertext); `--show-secrets` MAY be passed to reveal plaintext values.
- **FR-018**: `pulumi config edit` MUST work for local-config stacks (opens `Pulumi.<stack>.yaml`).
- **FR-019**: `pulumi config env` (bare) MUST print the config source for the current stack, following the `pulumi stack` (bare) pattern. For service-backed stacks, it prints the linked ESC environment name (with pin info if pinned). For local-config stacks, it prints the local config file path (e.g., `Pulumi.dev.yaml`). `--json` is supported; the JSON output MUST include the source type and, for service-backed stacks, the current version (revision number).

**Unsupported commands and error handling:**

- **FR-020**: `pulumi config env add/rm/ls` MUST operate on the ESC environment's `imports` section for service-backed stacks, using `GetEnvironment`/`UpdateEnvironmentWithProject` with etag-based optimistic concurrency. For add/rm, the CLI shows a before/after preview and prompts for confirmation.
- **FR-021**: `pulumi config refresh` MUST be deprecated for service-backed stacks. When run against a service-backed stack, it MUST print a message explaining that configuration is already read live from ESC and that there is no local config file to refresh. The message SHOULD also mention `pulumi config restore <rev>` as the command for reverting to an older config version.
- **FR-023**: The `--config-file` flag MUST return an error when used with a service-backed stack.
- **FR-030**: When the user has stack access but lacks ESC environment permissions, config commands MUST return a clear permission error explaining that ESC access is required.
- **FR-031**: When the service rejects a request because service-backed configuration is not available (e.g., feature not enabled for the organization), the CLI MUST surface that rejection as a clear, actionable error message.

**Stack deletion:**

- **FR-032**: When a service-backed stack is deleted via `pulumi stack rm`, the service MUST soft-delete the linked ESC environment alongside the stack. Deletion MUST be skipped if the environment is deletion-protected or referenced by other non-deleted environments. Stack deletion MUST NOT fail if environment cleanup fails; a warning is logged instead. If the stack is later undeleted, the environment MUST be restored as well.

### Key Entities

- **Stack Config Link**: The association between a Pulumi stack and its backing ESC environment. Contains the environment reference (with optional `@version` pin suffix) and secrets provider metadata.
- **ESC Environment (Stack)**: The ESC environment that stores a stack's configuration. Named `<project>/<stack-name>`. Contains `pulumiConfig` (stack config values), `environmentVariables`, and optional imports from other environments.
- **Config Source Mode**: A stack is either "service-backed" (config in ESC) or "local" (config in `Pulumi.<stack>.yaml`). Never both simultaneously.
- **Pin Target**: An optional version qualifier on the environment reference — a revision number (immutable), a tag name (mutable pointer), or absent (latest).

## Clarifications

### Session 2026-03-10

- Q: Should service-backed stacks have special offline/disconnected behavior? → A: No — same behavior as existing ESC imports (network required, fail with error when cloud unreachable). No caching or fallback.
- Q: Should eject delete the ESC environment? → A: Yes, delete by default with a confirmation prompt. Offer `--keep-env` to opt out. Skip deletion if deletion-protected or referenced by others. Non-interactive mode proceeds without prompting.
- Q: How should feature flag gating work? → A: The CLI always includes the new commands and prompts. When the service rejects a request because the feature is not enabled, the CLI surfaces that error clearly.
- Q: Should `config edit` show secrets by default? → A: No. `--show-secrets` defaults to false, matching all other pulumi/esc commands. Without it, users see `fn::secret: { ciphertext: ... }` which is opaque but round-trips safely. Users opt in with `--show-secrets` to see plaintext.
- Q: Should the ESC environment be soft-deleted when a stack is soft-deleted? → A: Yes. Soft-delete the environment alongside the stack to prevent 409 Conflict on stack recreation. Skip if deletion-protected or referenced by other non-deleted stacks. Restore the environment when the stack is undeleted.
- Q: Can existing stacks adopt service-backed config without local config to migrate? → A: Deferred to follow-up. V1 supports creation via `stack init --remote-config` or migration via `config env init --migrate` (which requires a local config file).
- Q: Should FR-032 (stack deletion) have acceptance scenarios, and should stale US8/FR-022 checklist references be cleaned up? → A: Yes — added US8 (stack deletion, P2) with 3 acceptance scenarios. Checklist updated to reflect US8 is now stack deletion; draft workflow deferred to non-goals.
- Q: What concurrency model should `config set/rm` use for service-backed stacks? → A: Optimistic concurrency via etags. If the environment was modified between read and write, fail with a clear error and let the user retry. Short conflict window makes this sufficient.
- Q: What should `pulumi config env` (bare) show for local stacks? → A: Follow the `pulumi stack` (bare) pattern — print the config source. For service-backed: linked ESC environment. For local: the config file path (e.g., `Pulumi.dev.yaml`). No confusing "no linked environment" message.
- Q: How should migration handle secrets decryption failure? → A: Fail before creating the environment — decrypt all secrets first, only proceed if all succeed. No partial state.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: All P1 acceptance scenarios pass: a service-backed stack can be created, configured, and deployed without any local config file.
- **SC-002**: Each existing `pulumi config` subcommand (`set`, `get`, `rm`, `set-all`, `rm-all`, list) produces the same user-visible result on a service-backed stack as it does on a local-config stack (verified by running each command's acceptance scenarios). `config cp` is out of scope for service-backed stacks in v1.
- **SC-003**: Migration scenarios (US4) preserve all config values and environment imports — verified by comparing `pulumi config` output before and after migration.
- **SC-004**: Eject scenarios (US5) preserve all resolved config values — verified by comparing `pulumi config` output before and after eject.
- **SC-005**: Every conflict scenario (local file + service-backed link) defined in US3 produces a hard error with actionable guidance.
- **SC-006**: Running `pulumi stack init` and `pulumi new` without `--remote-config` in non-interactive mode produces identical behavior to today (no prompts, local config created).
- **SC-007**: Pin scenarios (US6) are verified: `pulumi up` on a pinned stack uses exactly the pinned revision's values, confirmed by diffing resolved config before and after pinning.
- **SC-008**: Restore scenario (US6.4) is verified: after restoring to revision N, `pulumi config` output matches the config values from revision N.
- **SC-009**: An older CLI version receives a clear upgrade error when running `pulumi up` on a service-backed stack.
- **SC-010**: ESC permission errors are surfaced with a message that distinguishes "missing stack access" from "missing ESC environment access" (verified by US2 scenarios 9-10).
