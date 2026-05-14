# The Cloud-Ready CLI — Best Practices

Synthesized from the body of [epic #22959](https://github.com/pulumi/pulumi/issues/22959) and the merged PRs in [`COMPLETED.md`](./COMPLETED.md). The three Notion guideline docs referenced in the epic (Refined Proposal, CLI Command Guidelines, CLI Option Guidelines) are private and were not accessible — anything from those should be reconciled against this list.

> **Cite format:** rules below cite the issue or PR that established them, plus a representative source file where applicable. Where the merged code contradicts a stated decision, both are recorded — see [Contradictions to resolve](#contradictions-to-resolve) at the end.

---

## 1. Authoritative decisions from #22959

These are explicit decisions in the epic body. Treat as the contract; merged code already drifts on a few — see contradictions.

- **Required inputs are positional arguments** — except context inputs (`--org`, `--stack`, `--project`) which are flags.
- **`edit` flags are ternary.** If a flag was not `.Changed`, the field is not modified. Booleans use `--active[=true]` / `--active=false`.
- **List flags are repeated.** `--target x --target y`. Editing membership of a list uses `--add-X` / `--remove-X`. `edit` is implemented as PATCH; the command does a GET, applies the diff, then PATCHes the full list back.
- **`new` and `edit` print the resulting structure** on success.
- **Default `--output` is human-readable, not JSON.** Prettiness level is flexible for now.
- **Errors and diagnostics go to stderr.** They may use a different format than the success channel.
- **Tests inject dependencies** via `NewXxxCmd(injected...) *cobra.Command`.
- **Cobra owns type validation and required-args checks**; semantic validation lives in the service. Do not write tests for cobra commands.
- **Use the shared `--output` flag helper** introduced in [#23112](https://github.com/pulumi/pulumi/pull/23112).
- **No wizards for `edit`** (for now) — all changes go through flags.

### Pagination policy from #22959
- Each paginated endpoint exposes a CLI command with `--count` (number of results). If `--count` exceeds the first-page size, the command fetches additional pages automatically.
- Default `--count` equals the size of the first page.
- `--all` flag, mutually exclusive with `--count`.

---

## 2. Command naming & taxonomy

- **Verbs**: `list` (alias `ls`), `get`, `new`, `edit`, `remove`, plus operation-specific verbs like `ping`, `search`, `describe`, `cancel`. _(PRs [#23074](https://github.com/pulumi/pulumi/pull/23074), [#23077](https://github.com/pulumi/pulumi/pull/23077), [#23082](https://github.com/pulumi/pulumi/pull/23082), [#23088](https://github.com/pulumi/pulumi/pull/23088), [#23089](https://github.com/pulumi/pulumi/pull/23089), [#23101](https://github.com/pulumi/pulumi/pull/23101), [#23106](https://github.com/pulumi/pulumi/pull/23106), [#23114](https://github.com/pulumi/pulumi/pull/23114))_
- **Noun-then-verb** ordering. Nest nouns when meaningful: `pulumi insights resource search`, `pulumi stack webhook list`.
- **`[EXPERIMENTAL]`** prefix in `Short`/`Long` on all commands. All added commands should be hidden, with a comment saying `// AI Generated - needs human review`

---

## 3. Required vs optional inputs

- Required inputs are positional arguments. If multiple inputs are required, there will be multiple positional args. (ignoring `--stack`, `--org` and `--project`).
- **Context is flags**: `--org`, `--stack`/`-s`, `--account`, `--project`. `--stack` always carries help text _"Defaults to the current stack"_.
- **Filters are flags, not args**: `--name`, `--org`, `--search` ([#23074](https://github.com/pulumi/pulumi/pull/23074)); `--query`/`-q`, `--sort`, `--asc`, `--properties`, `--collapse` ([#23087](https://github.com/pulumi/pulumi/pull/23087)).
- **Required non-context flags** combine `cmd.MarkFlagRequired("…")` with a manual guard in the run function so direct test invocations also error cleanly (e.g. `insights_resource_get.go:107-122`).

---

## 4. Flag conventions

- **List-valued flags use `StringArrayVar`** (preserves repeated values), not `StringSliceVar` (which splits on `,`). Exception: `--sort` allows comma-separation via `StringSliceVar` ([#23087](https://github.com/pulumi/pulumi/pull/23087)).
- **Booleans default to a safe value.** E.g. `--asc` on `deployment list` ([#23114](https://github.com/pulumi/pulumi/pull/23114)), `insights resource search` ([#23087](https://github.com/pulumi/pulumi/pull/23087)). Webhook `--active` defaults to `true` on `new` (`stack_webhook_new.go:144`).
- **Validate enum/format flags before any network call** so typos don't burn requests (`--sort` against a whitelist in [#23087](https://github.com/pulumi/pulumi/pull/23087); `--output` validated in [#23077](https://github.com/pulumi/pulumi/pull/23077), [#23114](https://github.com/pulumi/pulumi/pull/23114)).
- **`--yes` / `-y`** on create-shaped commands accepts defaults non-interactively ([#23101](https://github.com/pulumi/pulumi/pull/23101) — `stack_webhook_new.go:150`).
- **Args parsing** goes through `constrictor.AttachArguments(cmd, ...)`, never raw `cobra.NoArgs` / `cobra.ExactArgs`. Use `constrictor.NoArgs` for no positionals or `&constrictor.Arguments{Arguments: [...], Required: N}` for positionals (`stack_get.go:64`, `stack_webhook.go:49-56`).

---

## 5. Output format

- **Default = human-pretty.** Tables (via `github.com/jedib0t/go-pretty/v6/table`, `table.StyleLight`, **uppercase column headers**) for lists; aligned key/value pairs for single records (`stack_webhook_get.go:179-223`). Tables auto-fit terminal width with a 120-col fallback for non-TTY ([#22874](https://github.com/pulumi/pulumi/pull/22874)).
- **`--output json`** uses `json.NewEncoder(w)` with `SetEscapeHTML(false)` and `SetIndent("", "  ")` — never `json.Marshal`/`Println`.
- **JSON envelope is a dedicated struct with explicit `json:` tags**, not the raw apitype. Lists ship `{ "<noun>s": [...], "count": N, "page": P, "itemsPerPage": K, "total": T }` (e.g. `webhookListEnvelope` in `stack_webhook_list.go`; `deploymentListEnvelope` in `deployment_list.go`).
- **Nil slices become `[]`** before encoding so scripts can rely on the array key existing (`deployment_list.go:258`, `stack_webhook_list.go:151-158`).
- **Empty results print a single line**, not an empty table: _"No webhooks configured for this stack."_, _"No deployments found for this stack."_ ([#23087](https://github.com/pulumi/pulumi/pull/23087), [#23114](https://github.com/pulumi/pulumi/pull/23114)).
- **Empty cells fall back to `-`** to keep alignment ([#23087](https://github.com/pulumi/pulumi/pull/23087)).
- **Footer pattern** on tables: `"N webhook(s)\n"` or `"Showing N of M deployment(s) (page P)"` (`stack_webhook_list.go:255-294`, `deployment_list.go:212-237`).
- **Pagination hint** surfaced in human output: _"More results available. Re-run with --cursor \"next-token\" to continue."_ ([#23087](https://github.com/pulumi/pulumi/pull/23087)).
- **Always use `cmd.OutOrStdout()` / `cmd.ErrOrStderr()`**, never `os.Stdout` directly (reviewer feedback in [#22769](https://github.com/pulumi/pulumi/pull/22769)).

---

## 6. Help text & examples

- **`Use:`** is the bare verb (`"list"`, `"get"`, `"new"`).
- **`Short:`** is a single-sentence imperative, no trailing period: _"List deployments for a stack"_.
- **`Long:`** opens with `[EXPERIMENTAL] ` (while hidden), then 2–3 short paragraphs, then a closing note about default output and `--output=json`. Build as concatenated string literals (`stack_webhook_list.go:61-65`, `stack_get.go:42-49`).
- **`Example:`** present on create-shaped commands. Comment lines prefixed `# `, 2-space indent, blank line between examples (`stack_webhook_new.go:93-103`, `deployment_list.go:87-96`).

---

## 7. File layout, constructors, and dependency injection

- **One package per top-level noun** under `pkg/cmd/pulumi/<noun>/` (`stack/`, `deployment/`, `org/`, `policy/`, `insights/`, `env/`, `templatecmd/`). Set up by [#23071](https://github.com/pulumi/pulumi/pull/23071).
- **File-per-verb, not file-per-noun**: `stack_webhook_list.go`, `stack_webhook_get.go`, `stack_webhook_new.go`, `deployment_list.go`, `stack_get.go`. The parent noun file (`stack_webhook.go`, `deployment.go`) holds only the group command and shared helpers.
- **Test files live next to implementation**, `_test.go` suffix, **same package** (not `_test`), so tests can reach unexported types.
- **Copyright header goes on every new file**:

```md
// Copyright 2026, Pulumi Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
```

- **Two constructors per verb**: `newXxxCmd()` (production wiring) and `newXxxCmdWith(factory)` (test seam). See `stack_webhook_list.go:48-52`, `stack_webhook_new.go:59-63`, `deployment_list.go:69-73`.
- **The injectable is a factory function**, not a client struct: `func(ctx, stackFlag string) (XxxClient, client.StackIdentifier, error)`. Examples: `stackWebhookListClientFactory`, `deploymentListClientFactory`.
- **`XxxClient` is a narrow per-command interface** containing only the single API method this command calls (e.g. `stackWebhookGetClient` only has `GetStackWebhook`). The real `*client.Client` satisfies it implicitly.
- **Resolve the backend** with `stack.RequireCloudStack(ctx, cmdutil.Diag(), pkgWorkspace.Instance, cmdBackend.DefaultLoginManager, stackFlag)` (`pkg/cmd/pulumi/stack/io.go:547`). When cross-package, open-code the same logic with the right error wording (`deployment_list.go:123-158`).
- **Run signature** is `func runXxx(ctx, w io.Writer, factory, ..., output string) error` so tests pass a `&bytes.Buffer{}`.
- The injected factory is not optional, and you will assert it's not `nil` with `contract.Assertf`.

---

## 8. Service-client calls & errors

- **Wrap API errors with a verb-prefixed message**: `"listing stack webhooks: %w"`, `"creating stack webhook: %w"`, `"reading stack webhook: %w"`. Examples in `stack_webhook_list.go:111`, `stack_webhook_new.go:351`, `stack_webhook_get.go:93`.
- **Backend-required commands** fail clearly: `"… requires the Pulumi Cloud backend; run 'pulumi login'"` (`deployment_list.go:138, 155`).

---

## 9. Tests

- **Same-package tests** (`package stack`, not `package stack_test`).
- **Per-command mock** implementing the narrow client interface, with stored fields for fixtures and an optional `captured *capturedListCall` to assert on inputs (`deployment_list_test.go:33-57`).
- **Drive `runXxx` directly with `&bytes.Buffer{}`** — don't parse cobra flags inside tests.
- **`t.Parallel()` on every test.** Use `t.Context()` for the context.
- **Assertions**: `assert.JSONEq` on the full envelope for JSON output; for tables, use `assert.Equals` on the full output.
- **Table-driven** reserved for pure helpers (`TestCapitalizeFirst` in `stack_get_test.go:229`); each scenario is otherwise its own `Test*` function.
- Do not test code that you did not write. Don't write tests that flags are handled correctly.
-  Two parallel tests may not use the same global `cmdStack.ConfigFile`, it races. Tests that need it should not run in parallel.

---

## Contradictions

Some merged code does not follow the above guidelines. The guidelines take precedence over what you observe.
