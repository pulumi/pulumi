# Tasks: Service-Backed Configuration

**Input**: Design documents from `/specs/001-service-backed-config/`
**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/

**Tests**: Included where plan.md explicitly specifies them (behavior-preserving tests for refactored commands, unit tests for new implementations).

**Organization**: Tasks are grouped by user story to enable independent implementation and testing of each story.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

---

## Phase 1: Setup

**Purpose**: Create the ConfigEditor abstraction that all subsequent work builds on

- [ ] T001 Create editor.go with ConfigEditor interface (Set, Remove, Save methods), LocalConfigEditor struct, and NewConfigEditor factory stub that always returns LocalConfigEditor in pkg/cmd/pulumi/config/editor.go

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Implement LocalConfigEditor and refactor existing write commands to use it. This phase preserves all existing behavior — no functional changes, no IsRemote guard removal.

**CRITICAL**: No user story work can begin until this phase is complete

- [ ] T002 Implement LocalConfigEditor: Set() with eager encryption via config.Encrypter then config.Map.Set(), Remove() delegating to config.Map.Delete(), Save() via workspace.SaveProjectStack in pkg/cmd/pulumi/config/editor.go
- [ ] T003 Refactor configSetCmd and configRmCmd to obtain a ConfigEditor via NewConfigEditor and call editor.Set()/Remove()/Save() for local stacks, keeping all IsRemote error guards intact in pkg/cmd/pulumi/config/config.go
- [ ] T004 Refactor configSetAllCmd and configRmAllCmd to use ConfigEditor for local stacks (loop of Set/Remove calls, single Save), keeping all IsRemote error guards intact in pkg/cmd/pulumi/config/config.go
- [ ] T005 [P] Add behavior-preserving unit tests for LocalConfigEditor: plain value set, secret set with encryption verification, --path set, remove existing key, remove nonexistent key (no-op), set-all batch, rm-all batch in pkg/cmd/pulumi/config/editor_test.go

**Checkpoint**: Local config behavior unchanged. ConfigEditor abstraction ready for ESC implementation.

---

## Phase 3: User Story 1 — Create a New Stack with Service-Backed Config (Priority: P1)

**Goal**: Users can create service-backed stacks via `pulumi stack init --remote-config` and `pulumi new --remote-config`. The ESC environment is created and linked automatically.

**Independent Test**: Run `pulumi stack init dev --remote-config`, verify no local config file, verify ESC environment exists, verify `pulumi config set/get` works (requires US2).

**Note**: Full independent testing of US1 requires US2 (escConfigEditor) to be complete, since US1 quickstart includes `config set/get`. US1 stack creation and US2 config read/write are co-dependent P1 stories.

### Implementation for User Story 1

- [ ] T006 [US1] Unhide --remote-config flag and add interactive config-location prompt (service-backed vs local) when logged into Pulumi Cloud in pkg/cmd/pulumi/stack/stack_init.go
- [ ] T007 [US1] Add non-interactive default to local config (no prompt for --non-interactive, no TTY, or --yes), backend validation error for self-hosted/filestate backends, and flag conflict errors for --remote-config + --secrets-provider and --remote-config + --copy-config-from in pkg/cmd/pulumi/stack/stack_init.go
- [ ] T008 [P] [US1] Add --remote-config flag and --remote-stack-config alias to pulumi new with interactive prompt for Pulumi Cloud backends and --yes defaulting to local config in pkg/cmd/pulumi/newcmd/new.go
- [ ] T009 [P] [US1] Add unit tests for stack init --remote-config: interactive prompt shown, non-interactive defaults to local, self-hosted backend returns error, --secrets-provider conflict error, --copy-config-from conflict error in pkg/cmd/pulumi/stack/stack_init_test.go

**Checkpoint**: Service-backed stacks can be created. Config read/write depends on US2.

---

## Phase 4: User Story 2 — Read and Write Config on Service-Backed Stacks (Priority: P1)

**Goal**: `pulumi config set/get/rm/set-all/rm-all` and `pulumi config` (list) work transparently against the ESC environment. Unsupported commands return clear errors.

**Independent Test**: On a service-backed stack, run `config set foo bar`, `config get foo`, `config set --secret dbpass hunter2`, `config rm foo`, `config` (list), verify all values are stored in and read from the ESC environment.

### Implementation for User Story 2

- [ ] T010 [US2] Implement escConfigEditor struct with Set() (translate config.Key to pulumiConfig YAML path, wrap secrets in fn::secret, handle --path via resource.ParsePropertyPathStrict), Remove() (delete from YAML), and Save() (UpdateEnvironmentWithProject with revision etag for optimistic concurrency) in pkg/cmd/pulumi/config/editor.go
- [ ] T011 [US2] Update NewConfigEditor factory to return escConfigEditor when stack.ConfigLocation().IsRemote, loading the environment definition YAML and current revision from the ESC API in pkg/cmd/pulumi/config/editor.go
- [ ] T012 [US2] Remove IsRemote error guards from configSetCmd, configRmCmd, configSetAllCmd, and configRmAllCmd — these commands now transparently use the ConfigEditor returned by the factory in pkg/cmd/pulumi/config/config.go
- [ ] T013 [US2] Add config source annotation header to config list output showing ESC environment name, revision number, and tag name (if pinned) for service-backed stacks in pkg/cmd/pulumi/config/config.go
- [ ] T014 [US2] Add error guards for service-backed stacks: --config-file flag returns error, config cp returns "not supported in v1" error, config refresh prints deprecation message pointing to `config restore <rev>` in pkg/cmd/pulumi/config/config.go
- [ ] T015 [P] [US2] Add unit tests for escConfigEditor: set plain value under pulumiConfig, set secret value with fn::secret wrapping, remove key, --path nested navigation, etag conflict error on Save, permission error handling in pkg/cmd/pulumi/config/editor_test.go

**Checkpoint**: All P1 config operations work transparently on service-backed stacks.

---

## Phase 5: User Story 3 — Deploy and Destroy with Service-Backed Config (Priority: P1)

**Goal**: `pulumi up`, `pulumi preview`, and `pulumi destroy` resolve config from ESC. Conflict detection prevents ambiguous states.

**Independent Test**: Set config on a service-backed stack, run `pulumi preview` and `pulumi up`, verify config values reach the program. Create a local config file alongside, verify hard error on next operation.

### Implementation for User Story 3

- [ ] T016 [US3] Upgrade conflict detection from warning to hard error in LoadProjectStack when stack.ConfigLocation().IsRemote and local Pulumi.<stack>.yaml contains non-empty config: map or environment imports; exempt metadata-only files (encryptionsalt, secretsprovider without config data) in pkg/cmd/pulumi/stack/io.go
- [ ] T017 [P] [US3] Add conflict detection unit tests: hard error when local file has config values, hard error when local file has environment imports, no error for metadata-only local file, no error when no local file exists in pkg/cmd/pulumi/stack/io_test.go

**Checkpoint**: All P1 stories complete. Service-backed stacks can be created, configured, and deployed.

---

## Phase 6: User Story 5 — Eject from Service-Backed Config (Priority: P2)

**Goal**: Users return to local config via `pulumi config env eject`. All resolved values are preserved in a local file.

**Independent Test**: Set up a service-backed stack with config values, run `pulumi config env eject`, verify local file created with all values, verify service-backed link removed.

### Implementation for User Story 5

- [ ] T020 [US5] Create config_env_eject.go with eject command: show confirmation prompt listing actions, resolve all config values from ESC environment, prompt for local secrets provider when secrets exist, write resolved values to Pulumi.<stack>.yaml in pkg/cmd/pulumi/config/config_env_eject.go
- [ ] T021 [US5] Implement service-backed link removal, ESC environment deletion (default) with --keep-env flag to preserve, non-interactive mode (require --secrets-provider when secrets exist, proceed without prompts), and edge case handling: deletion-protected env preserved with message, stale/deleted env cleaned up with warning, deletion failure still completes eject with warning in pkg/cmd/pulumi/config/config_env_eject.go

**Checkpoint**: Users can safely leave service-backed config with no data loss.

---

## Phase 7: User Story 6 — Pin and Version Config (Priority: P2)

**Goal**: Users pin to a specific revision or tag, and unpin with `latest`.

**Independent Test**: Make config changes to create revisions, pin to a revision, verify `pulumi up` uses pinned values, unpin and verify latest is used.

### Implementation for User Story 6 (Pin)

- [ ] T022 [P] [US6] Create config_pin.go with pin command: accept revision number, tag name, or "latest" keyword (unpin); update stack environment reference with @version suffix; validate against retracted revisions and deleted tags; no-op with message for local stacks in pkg/cmd/pulumi/config/config_pin.go
- [ ] T024 [US6] Add mutation rejection for pinned stacks: before creating ConfigEditor in set/rm/set-all/rm-all handlers, check if stack is pinned (revision or tag) and return "unpin first" error; also reject config edit on pinned stacks in pkg/cmd/pulumi/config/config.go

**Checkpoint**: Config pinning and version selection work for service-backed stacks.

---

## Phase 8: User Story 7 — Edit, View, and Inspect Service-Backed Config (Priority: P2)

**Goal**: Users discover linked environment info, edit config in `$EDITOR`, and open ESC console in browser.

**Independent Test**: On a service-backed stack, run `pulumi config env` to see linked environment, `pulumi config web` to open browser, `pulumi config edit` to bulk-edit.

### Implementation for User Story 7

- [ ] T025 [US7] Add config env (bare) handler: for service-backed stacks print ESC environment name with pin info (revision/tag), for local stacks print config file path; add --json flag with source type, environment, organization, project, version, pinned, tag fields in pkg/cmd/pulumi/config/config_env.go
- [ ] T026 [P] [US7] Create config_edit.go: download ESC environment YAML, open in $EDITOR (fall back to vi/notepad), upload modified YAML with etag conflict detection on save; --show-secrets flag (default false); for local stacks open Pulumi.<stack>.yaml in $EDITOR; reject on pinned stacks in pkg/cmd/pulumi/config/config_edit.go
- [ ] T027 [P] [US7] Create config_web.go: construct Pulumi Cloud console URL for the ESC environment and open in default browser; return error for local stacks in pkg/cmd/pulumi/config/config_web.go
- [ ] T028 [US7] Add service-backed error guards with actionable YAML snippets for config env add (show imports: syntax), config env rm (show which import to remove), and config env ls (point to config edit/web/env get) in pkg/cmd/pulumi/config/config_env_add.go and pkg/cmd/pulumi/config/config_env.go

**Checkpoint**: Full inspection and editing UX available for service-backed stacks.

---

## Phase 9: User Story 4 — Migrate Existing Stack to Service-Backed Config (Priority: P3)

**Goal**: Users migrate local config to service-backed via `pulumi config env init --migrate`. All values and imports are preserved.

**Independent Test**: Create a stack with local config (values + environment imports), run `pulumi config env init --migrate`, verify all values are in the ESC environment, verify local operations work post-migration.

### Implementation for User Story 4

- [ ] T018 [US4] Add --migrate flag to config env init command; implement migration: decrypt all secrets upfront (fail fast if any decryption fails), create ESC environment `<project>/<stack>`, write all config values to pulumiConfig section, carry over environment imports, link stack to environment in pkg/cmd/pulumi/config/config_env_init.go
- [ ] T019 [US4] Implement idempotent merge for existing ESC environments (local pulumiConfig values overwrite with warnings for each overwritten key), add post-migration prompt to delete local config file, and guard against already-service-backed stacks in pkg/cmd/pulumi/config/config_env_init.go

**Checkpoint**: Existing stacks can be migrated to service-backed config.

---

## Phase 10: User Story 6 (continued) — Restore Config (Priority: P3)

**Goal**: Users restore config to a previous revision, creating a new revision (history not rewritten).

**Independent Test**: Make several config changes, restore to an earlier revision, verify config values match the restored revision.

### Implementation for User Story 6 (Restore)

- [ ] T023 [US6] Create config_restore.go with restore command: read environment content from specified revision, create new revision with that content via etag-based update (fail with concurrency error if env modified between read and write); error for local stacks in pkg/cmd/pulumi/config/config_restore.go

**Checkpoint**: Config rollback via restore works for service-backed stacks.

---

## Phase 11: User Story 8 — Delete a Service-Backed Stack (Priority: P3)

**Goal**: Stack deletion cleans up the linked ESC environment automatically (server-side). CLI surfaces warnings when cleanup is skipped.

**Requires**: Backend/service changes to support automatic ESC environment cleanup on stack deletion.

**Independent Test**: Create a service-backed stack, delete with `pulumi stack rm`, verify recreating with same name succeeds (no 409 conflict).

### Implementation for User Story 8

- [ ] T029 [US8] Surface ESC environment cleanup warnings from service API response when deleting service-backed stacks via stack rm: display message when environment preserved due to deletion protection or cross-references in pkg/cmd/pulumi/stack/ stack removal commands

**Checkpoint**: Stack deletion is clean — no orphaned environments, no naming collisions on recreation.

---

## Phase 12: Polish & Cross-Cutting Concerns

**Purpose**: Finalization tasks that span multiple stories

- [ ] T030 [P] Add changelog entries for new commands (config edit, config web, config pin, config restore, config env eject, config env init --migrate) and behavior changes (config env add/rm/ls guards, conflict detection upgrade, --remote-config prompt)
- [ ] T031 Run quickstart.md validation scenarios end-to-end against a built CLI

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies — start immediately
- **Foundational (Phase 2)**: Depends on Phase 1 — BLOCKS all user stories
- **US1 (Phase 3)** and **US2 (Phase 4)**: Both depend on Phase 2. Co-dependent for full testing (US1 needs escConfigEditor from US2 for config set/get; US2 needs stack creation from US1 for a service-backed stack). Implement in listed order.
- **US3 (Phase 5)**: Depends on Phase 2. Independent of US1/US2 for implementation but benefits from them for integration testing.
- **P2 stories — US5, US6-pin, US7 (Phases 6–8)**: All depend on P1 stories (Phases 3–5) being complete. Can proceed in parallel or in priority order.
- **P3 stories — US4, US6-restore, US8 (Phases 9–11)**: Depend on P1 stories. US8 also requires backend/service changes.
- **Polish (Phase 12)**: Depends on all desired user stories being complete.

### User Story Dependencies

- **US1 (P1)**: After Foundational. Full testing requires US2.
- **US2 (P1)**: After Foundational. Full testing requires US1 (for stack creation).
- **US3 (P1)**: After Foundational. Independent of US1/US2 for implementation.
- **US5 (P2)**: After P1 stories. Independent of other P2 stories.
- **US6-pin (P2)**: After P1 stories. Independent of other P2 stories.
- **US7 (P2)**: After P1 stories. Independent of other P2 stories.
- **US4 (P3)**: After P1 stories. Independent of other stories.
- **US6-restore (P3)**: After P1 stories. Independent of US6-pin (restore operates on base env regardless of pin).
- **US8 (P3)**: After P1 stories. Requires backend service changes; CLI work is minimal.

### Within Each Phase

- Interface/struct before implementation
- Implementation before tests (unless TDD requested)
- Factory before consumers
- Error guards after primary functionality
- Same-file tasks are sequential

### Parallel Opportunities

- **Phase 2**: T005 (editor_test.go) parallel with T003–T004 (config.go refactoring)
- **Phase 3**: T008 (new.go) and T009 (stack_init_test.go) parallel with each other and with T006–T007 (stack_init.go)
- **Phase 4**: T015 (editor_test.go) parallel with T012–T014 (config.go changes)
- **Phase 5**: T017 (io_test.go) parallel with T016 (io.go)
- **Phase 7**: T022 (config_pin.go) parallel with other phase work
- **Phase 8**: T026 (config_edit.go) and T027 (config_web.go) parallel — different new files
- **P2 stories**: US5, US6-pin, US7 can proceed in parallel across different developers
- **P3 stories**: US4, US6-restore, US8 can proceed in parallel

---

## Parallel Example: Phase 3 (US1)

```
# Sequential (same file — stack_init.go):
Task T006: "Unhide --remote-config, add interactive prompt"
Task T007: "Non-interactive default, backend validation, flag conflicts"

# Parallel with T006–T007 (different files):
Task T008: "Add --remote-config to pulumi new in newcmd/new.go"
Task T009: "Unit tests for stack init in stack_init_test.go"
```

## Parallel Example: Phase 8 (US7)

```
# Parallel (different new files):
Task T026: "Create config_edit.go"
Task T027: "Create config_web.go"

# After T026–T027 (may touch same files):
Task T028: "Add error guards for config env add/rm/ls"
```

---

## Implementation Strategy

### MVP First (P1 Stories: US1 + US2 + US3)

1. Complete Phase 1: Setup (T001)
2. Complete Phase 2: Foundational (T002–T005)
3. Complete Phase 3: US1 — Stack creation (T006–T009)
4. Complete Phase 4: US2 — Config read/write (T010–T015)
5. Complete Phase 5: US3 — Deploy/destroy (T016–T017)
6. **STOP and VALIDATE**: Run quickstart.md US1–US3 scenarios
7. Deploy/demo if ready

### Incremental Delivery (P2 Stories)

8. Add US5: Eject (T020–T021) → Validate US5 quickstart
9. Add US6-pin: Pin/Version (T022, T024) → Validate US6 pin quickstart
10. Add US7: Edit/View/Inspect (T025–T028) → Validate US7 quickstart

### Deferred (P3 Stories)

11. Add US4: Migration (T018–T019) → Validate US4 quickstart
12. Add US6-restore: Restore (T023) → Validate US6 restore quickstart
13. Add US8: Stack deletion (T029) → Requires backend changes first
14. Polish (T030–T031)

### Suggested MVP Scope

P1 stories only (US1 + US2 + US3): Phases 1–5, tasks T001–T017 (17 tasks). This delivers a fully functional service-backed config experience: create, configure, deploy.
