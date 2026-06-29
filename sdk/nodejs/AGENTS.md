# Node.js SDK (`sdk/nodejs/`)

## Commands

All commands run from `sdk/nodejs/`. Prefix with `mise exec --` if mise is not activated.

- **Build:** `mise exec -- make build`
- **Install (required before integration tests):** `mise exec -- make install`
- **Lint:** `mise exec -- make lint`
- **Lint fix:** `mise exec -- make lint_fix`
- **Fast tests:** `mise exec -- make test_fast`
- **Full tests:** `mise exec -- make test_all`

## If you change...

- TypeScript files → `mise exec -- make lint && mise exec -- make test_fast`
- Integration tests link the locally-built core SDK via `npm link @pulumi/pulumi`, so build and install it first (`mise exec -- make build install`); the `install` step registers the global `npm link`.
