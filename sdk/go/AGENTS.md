# Go SDK (`sdk/go/`)

## Commands

All commands run from `sdk/go/`. Prefix with `mise exec --` if mise is not activated.

- **Build:** `mise exec -- make build`
- **Lint:** `mise exec -- make lint`
- **Fast tests:** `mise exec -- make test_fast`
- **Full tests:** `mise exec -- make test_all`

## If you change...

- Go files → `mise exec -- make lint && mise exec -- make test_fast`

## Deprecated code

- `sdk/go/common/resource/plugin` is functionally deprecated. It was forked to `pkg/resource/plugin` (#23700), and the engine and CLI only use the fork. Do not add features or make other changes to the `sdk` copy; make them in `pkg/resource/plugin` instead. The exception is the minimum needed to keep it compiling, such as a stub for a new RPC on its provider server.
