# Go SDK (`sdk/go/`)

## Commands

All commands run from `sdk/go/`. Prefix with `mise exec --`.

- **Build:** `mise exec -- make build`
- **Lint:** `mise exec -- make lint`
- **Fast tests:** `mise exec -- make test_fast`
- **Full tests:** `mise exec -- make test_all`

## If you change...

- Go files → `mise exec -- make lint && mise exec -- make test_fast`
