# Agent Instructions

## What this repo is
The core Pulumi SDK and CLI. Go monorepo with multiple Go modules (`pkg/`, `sdk/`, `tests/`) and language-specific SDKs (`sdk/nodejs/`, `sdk/python/`, `sdk/go/`, `sdk/pcl/`). Builds the `pulumi` CLI binary and language host binaries.

## Repo structure
- `pkg/` — Core engine: CLI commands, deployment engine, codegen, resource management, backends
- `sdk/` — Language SDKs and shared Go SDK code (`sdk/go/`, `sdk/nodejs/`, `sdk/python/`, `sdk/pcl/`)
- `sdk/go/pulumi-language-go/` — Go language host binary
- `sdk/nodejs/cmd/pulumi-language-nodejs/` — Node.js language host binary
- `sdk/python/cmd/pulumi-language-python/` — Python language host binary
- `tests/` — Integration and acceptance tests
- `proto/` — Protobuf definitions for gRPC interfaces between engine, language hosts, and providers
- `developer-docs/` — Internal developer documentation (Sphinx)
- `build/` — Build system scaffolding (`common.mk`)
- `scripts/` — CI and development helper scripts
- `changelog/` — Pending changelog entries

## Command canon
All commands assume you're at the repo root.

- **Build CLI:** `make build` (builds `bin/pulumi` + proto + WASM display)
- **Build a single SDK:** `cd sdk/nodejs && make build` (or `sdk/python`, `sdk/go`, `sdk/pcl`)
- **Lint:** `make lint` (runs golangci-lint across all Go modules + validates pulumi.json schema)
- **Lint fix:** `make lint_fix`
- **Format Go:** `make format` (runs gofumpt)
- **Fast tests:** `make test_fast` (unit tests, uses `-short` flag)
- **Full tests:** `make test_all` (unit + integration tests)
- **Test a single Go package:** `cd pkg && go test -count=1 -tags all ./codegen/go/...`
- **Tidy check:** `make tidy` (verifies `go mod tidy` is clean)
- **Proto generation:** `make build_proto` (regenerates from `proto/*.proto`)
- **Proto check:** `make check_proto` (fails if proto output is stale)
- **Changelog entry:** `make changelog` (interactive — creates a file in `changelog/`)
- **Go workspace:** `make work` (creates `go.work` for cross-module development)

### Per-SDK test commands
- `cd sdk/nodejs && make test_fast`
- `cd sdk/python && make test_fast`
- `cd sdk/go && make test_fast`

## Key invariants
- The repo has multiple Go modules (`pkg/go.mod`, `sdk/go.mod`, `tests/go.mod`, etc.). Changes to `go.mod` in one module may require updates in others. Run `make tidy` to verify.
- Proto-generated files in `sdk/proto/go/`, `sdk/nodejs/proto/`, `sdk/python/lib/pulumi/runtime/proto/` must stay in sync with `proto/*.proto`. CI enforces this via `make check_proto`.
- `pkg/codegen/schema/pulumi.json` is the metaschema. It must be valid JSON Schema and pass biome formatting.
- Changelog entries are required for most PRs. Run `make changelog` to create one.
- PRs are squash-merged — the PR description becomes the commit message.

## Forbidden actions
- Do not run `git push --force`, `git reset --hard`, or `rm -rf` without explicit approval.
- Do not skip linting or bypass pre-commit hooks (`--no-verify`).
- Do not edit generated proto files by hand — edit `proto/*.proto` and run `make build_proto`.
- Do not add external runtime dependencies without discussion.
- Do not fabricate test output or changelog entries.
- Do not edit files in `sdk/proto/go/`, `sdk/nodejs/proto/`, or `sdk/python/lib/pulumi/runtime/proto/` directly.

## Escalate immediately if
- A change touches the public SDK surface (`sdk/go/...`, `sdk/nodejs/...`, `sdk/python/...` exported types/functions).
- A change touches the public interface of the `pulumi` SDK (`pkg/cmd/pulumi/...`, built using `github.com/spf13/cobra`).
- Requirements are ambiguous or conflicting.
- Tests fail after two debugging attempts.
- A change affects the deployment engine's state serialization or resource lifecycle.
- A change affects the events emitted by the engine.
- A change requires modifying protobuf definitions.

## If you change...
- Any `.go` file → `make format && make lint && make test_fast`
- `proto/*.proto` → `make build_proto && make check_proto` and commit generated files
- `go.mod` or `go.sum` in any module → run `make tidy` (runs `./scripts/tidy.sh --check`)
- `pkg/codegen/schema/pulumi.json` → `make lint_pulumi_json`
- `sdk/nodejs/` TypeScript files → `cd sdk/nodejs && make lint && make test_fast`
- `sdk/python/` Python files → `cd sdk/python && make lint && make test_fast`
- Anything in `pkg/codegen/` → run codegen tests: `cd pkg && go test -count=1 -tags all ./codegen/...`
- Anything in `pkg/backend/display/...` → add a test that uses pre-constructed, JSON-serialized engine events (ref. testProgressEvents)
- Anything that adds or changes the engine, resource options, or the provider interface → add a test to `pkg/engine/lifecycletest/`

## Go module layout
This repo uses multiple Go modules, NOT a single `go.mod`. The modules are:
- `pkg/go.mod` — engine, CLI, codegen
- `sdk/go.mod` — shared Go SDK
- `tests/go.mod` — integration tests
- `sdk/go/pulumi-language-go/go.mod` — Go language host
- `sdk/nodejs/cmd/pulumi-language-nodejs/go.mod` — Node.js language host
- `sdk/python/cmd/pulumi-language-python/go.mod` — Python language host
- `sdk/pcl/go.mod` — PCL runtime

Use `make work` to create a `go.work` file for cross-module development with proper replace directives.
