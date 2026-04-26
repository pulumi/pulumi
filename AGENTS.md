# Agent Instructions

## What this repo is

The core Pulumi SDK and CLI. Go monorepo with multiple Go modules (`pkg/`, `sdk/`, `tests/`, etc.) and language-specific SDKs (`sdk/nodejs/`, `sdk/python/`, `sdk/go/`, `sdk/pcl/`). Builds the `pulumi` CLI binary and language host binaries.

## Repo structure

- `pkg/` — Core engine: CLI commands, deployment engine, codegen, resource management, backends
- `sdk/` — Language SDKs and shared Go SDK code (`sdk/go/`, `sdk/nodejs/`, `sdk/python/`, `sdk/pcl/`)
- `tests/` — Integration and acceptance tests
- `proto/` — Protobuf definitions for gRPC interfaces between engine, language hosts, and providers
- `docs/` — Internal developer documentation (Sphinx)
- `build/` — Build system scaffolding (`common.mk`)
- `scripts/` — CI and development helper scripts
- `changelog/` — Pending changelog entries

## Tool setup

This repo uses [mise](https://mise.jdx.dev/) to manage tool versions (Go, Node, Python, protoc, etc.). See `.mise.toml` for the full list. If mise is installed and activated (via `mise activate` in your shell profile), tool versions are handled automatically and you can run `make` directly. Otherwise, **prefix all `make` commands with `mise exec --`** to ensure the correct tool versions are used:

```sh
mise exec -- make build
mise exec -- make lint
```

## Command canon

All commands assume you're at the repo root.

- **Build CLI and SDKs:** `mise exec -- make build`
- **Build CLI with specific SDKs:** `SDKS="nodejs python" mise exec -- make build`
- **Build a single SDK:** `cd sdk/nodejs && mise exec -- make build` (or `sdk/python`, `sdk/go`, `sdk/pcl`)
- **Lint:** `mise exec -- make lint`
- **Lint fix:** `mise exec -- make lint_fix`
- **Format:** `mise exec -- make format`
- **Fast tests:** `mise exec -- make test_fast`
- **Full tests:** `mise exec -- make test_all`
- **Language conformance tests:** `./scripts/run-conformance.sh`
- **Test a single Go package:** `cd pkg && go test -count=1 -tags all ./codegen/go/...`
- **Tidy check:** `mise exec -- make tidy`
- **Tidy fix:** `mise exec -- make tidy_fix`
- **Proto generation:** `mise exec -- make build_proto`
- **Proto check:** `mise exec -- make check_proto`
- **Changelog entry:** `mise exec -- make changelog` (interactive)
- **Go workspace:** `mise exec -- make work`

## Key invariants

- Multiple Go modules (`pkg/go.mod`, `sdk/go.mod`, `tests/go.mod`, etc.). Changes to `go.mod` in one module may require updates in others. Run `mise exec -- make tidy` to verify.
  - Use `mise exec -- make work` to create a `go.work` file for cross-module development.
- Proto-generated files must stay in sync with `proto/*.proto`. CI enforces via `mise exec -- make check_proto`.
- `pkg/codegen/schema/pulumi.json` is the metaschema. It must be valid JSON Schema and pass biome formatting.
- Changelog entries are required for most PRs. Run `mise exec -- make changelog` to create one.
- PRs are squash-merged — the PR description becomes the commit message.
- Integration tests need the CLI and SDKs built first. `bin/` must be on `PATH`.
- Copyright headers for new files should always be stamped with the current year.

## Forbidden actions

- Do not run `git push --force`, `git reset --hard`, or `rm -rf` without explicit approval.
- Do not skip linting.
- Do not edit generated proto files by hand — edit `proto/*.proto` and run `mise exec -- make build_proto`.
- Do not edit other generated files by hand. Some tests use golden files updated with `PULUMI_ACCEPT=1`.
- Do not add external runtime dependencies without discussion.
- Do not fabricate test output or changelog entries.
- Do not make sweeping changes, refactor unrelated code, or add unnecessary abstractions.

## Escalate immediately if

- A change touches the public SDK surface (exported types/functions in `sdk/go/`, `sdk/nodejs/`, `sdk/python/`).
- A change touches the public CLI interface (`pkg/cmd/pulumi/`, built with `github.com/spf13/cobra`).
- Requirements are ambiguous or conflicting.
- Tests fail after two debugging attempts.
- A change affects the deployment engine's state serialization or resource lifecycle.
- A change affects the events emitted by the engine.
- A change requires modifying protobuf definitions.

## If you change...

- Any `.go` file → `mise exec -- make format && mise exec -- make lint && mise exec -- make test_fast`
- `proto/*.proto` → `mise exec -- make build_proto && mise exec -- make check_proto` and commit generated files
- `go.mod` or `go.sum` in any module → `mise exec -- make tidy`
- `pkg/codegen/schema/pulumi.json` → `mise exec -- make lint_pulumi_json`

See subdirectory `AGENTS.md` files (`pkg/`, `sdk/nodejs/`, `sdk/python/`, `sdk/go/`) for package-specific instructions.
