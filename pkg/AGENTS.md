# Core Engine (`pkg/`)

## Testing

- **Unit tests:** `cd pkg && go test -count=1 -tags all ./...`
- **Codegen tests:** `cd pkg && go test -count=1 -tags all ./codegen/...`
- **Codegen for a specific language:** `cd pkg && go test -count=1 -tags all ./codegen/go/...`
- **Lifecycle tests:** `cd pkg && go test -count=1 -tags all ./engine/lifecycletest/...`

## If you change...

- Anything in `pkg/codegen/` → run codegen tests: `cd pkg && go test -count=1 -tags all ./codegen/...`
- Anything in `pkg/backend/display/` → add a test using pre-constructed, JSON-serialized engine events (ref. `testProgressEvents`)
- Anything that adds or changes the engine, resource options, or the provider interface → add a test to `pkg/engine/lifecycletest/`
