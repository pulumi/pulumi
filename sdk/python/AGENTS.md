# Python SDK (`sdk/python/`)

## Commands

All commands run from `sdk/python/`. Prefix with `mise exec --` if mise is not activated.

- **Build:** `mise exec -- make build`
- **Lint:** `mise exec -- make lint`
- **Lint fix:** `mise exec -- make lint_fix`
- **Format:** `mise exec -- make format`
- **Fast tests:** `mise exec -- make test_fast`
- **Full tests:** `mise exec -- make test_all`

## If you change...

- Python files → `mise exec -- make lint && mise exec -- make test_fast`
