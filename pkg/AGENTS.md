# Core Engine (`pkg/`)

## Testing

- **Unit tests:** `cd pkg && go test -count=1 -tags all ./...`
- **Codegen tests:** `cd pkg && go test -count=1 -tags all ./codegen/...`
- **Codegen for a specific language:** `cd pkg && go test -count=1 -tags all ./codegen/go/...`
- **Lifecycle tests:** `cd pkg && go test -count=1 -tags all ./engine/lifecycletest/...`

## Comments

- Doc comments state what a function does, in a line or two. Design rationale,
  trade-offs, and "why not X" belong in the PR description, not in code
  comments.
- When you change a function's behavior, re-read the doc comments on it, its
  callers, and its neighbors, and update any the change made stale (e.g. a
  comment that names a default you just made configurable).

## If you change...

- Anything in `pkg/codegen/` → run codegen tests: `cd pkg && go test -count=1 -tags all ./codegen/...`
- Anything in `pkg/backend/display/` → add a test using pre-constructed, JSON-serialized engine events (ref. `testProgressEvents`)
- Anything that adds or changes the engine, resource options, or the provider interface → add a test to `pkg/engine/lifecycletest/`
