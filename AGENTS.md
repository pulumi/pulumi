# Agent Instructions

## What this repo is
The core Pulumi SDK and CLI. Go monorepo with multiple Go modules (`pkg/`, `sdk/`, `tests/`, etc.) and language-specific SDKs (`sdk/nodejs/`, `sdk/python/`, `sdk/go/`, `sdk/pcl/`). Builds the `pulumi` CLI binary and language host binaries.

## Repo structure
- `pkg/` — Core engine: CLI commands, deployment engine, codegen, resource management, backends
- `sdk/` — Language SDKs and shared Go SDK code (`sdk/go/`, `sdk/nodejs/`, `sdk/python/`, `sdk/pcl/`)
- `tests/` — Integration and acceptance tests
- `proto/` — Protobuf definitions for gRPC interfaces between engine, language hosts, and providers
- `developer-docs/` — Internal developer documentation (Sphinx)
- `build/` — Build system scaffolding (`common.mk`)
- `scripts/` — CI and development helper scripts
- `changelog/` — Pending changelog entries

## Command canon
All commands assume you're at the repo root.

- **Build CLI and SDKs:** `make build` (builds `bin/pulumi`, proto, WASM display, and language SDKs)
- **Build CLI and certain SDKs:** `SDKS=nodejs python" make build` (builds `bin/pulumi`, proto, WASM display, nodejs and python SDKs)
- **Build a single SDK:** `cd sdk/nodejs && make build` (or `sdk/python`, `sdk/go`, `sdk/pcl`)
- **Lint:** `make lint` (runs golangci-lint across all Go modules + validates pulumi.json schema)
- **Lint fix:** `make lint_fix`
- **Format Code:** `make format` (runs gofumpt and formatting for python and nodejs)
- **Fast tests:** `make test_fast` (unit tests, uses `-short` flag)
- **Full tests:** `make test_all` (unit + integration tests)
- **Language conformance tests:** `./scripts/run-conformance.sh` (runs language conformance tests against language SDKs, exercises codegen and runtime)
- **Test a single Go package:** `cd pkg && go test -count=1 -tags all ./codegen/go/...`
- **Tidy check:** `make tidy` (verifies `go mod tidy` is clean)
- **Tidy fix:** `make tidy_fix` (verifies `go mod tidy` is clean and repairs errors if it is not)
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
    - Use `make work` to create a `go.work` file for cross-module development with proper replace directives.
- Proto-generated files in `sdk/proto/go/`, `sdk/nodejs/proto/`, `sdk/python/lib/pulumi/runtime/proto/` must stay in sync with `proto/*.proto`. CI enforces this via `make check_proto`.
- `pkg/codegen/schema/pulumi.json` is the metaschema. It must be valid JSON Schema and pass biome formatting.
- Changelog entries are required for most PRs. Run `make changelog` to create one.
- PRs are squash-merged — the PR description becomes the commit message.
- Integration tests need the CLI and SDKs to be built prior to running.
    - For the nodejs SDK, you must run `make install` to ensure thatr the SDK is available to `yarn link`.
    - `bin/` must be on the `PATH` when running integration tests, as that directory contains the built CLI and language hosts

## Forbidden actions
- Do not run `git push --force`, `git reset --hard`, or `rm -rf` without explicit approval.
- Do not skip linting.
- Do not edit generated proto files by hand — edit `proto/*.proto` and run `make build_proto`.
- Do not edit other generated files by hand. Some tests use golden files that are generated or updated by running the tests with `PULUMI_ACCEPT=1`.
- Do not add external runtime dependencies without discussion.
- Do not fabricate test output or changelog entries.
- Do not edit files in `sdk/proto/go/`, `sdk/nodejs/proto/`, or `sdk/python/lib/pulumi/runtime/proto/` directly.
- Do not make sweeping changes, refactor unrelated code, or add unnecessary abstractions.

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
