# Backend

## Backend Interface

`backend.go` defines the `Backend` interface (~40 methods):

**Stack lifecycle:** `GetStack`, `CreateStack`, `RemoveStack`, `RenameStack`, `ListStacks`
**Operations:** `Preview`, `Update`, `Refresh`, `Destroy`, `Import`, `Watch`
**State:** `ExportDeployment`, `ImportDeployment`, `GetHistory`, `GetLogs`
**Config:** `GetLatestConfiguration`, `UpdateStackTags`, `UpdateStackDeploymentSettings`

**Related interfaces:**
- `Stack` (`stack.go`) — snapshot access, tags, secrets management
- `StackReference` — opaque stack identifier with `Name()`, `Project()`, `FullyQualifiedName()`
- `EnvironmentsBackend` — optional ESC capability
- `SpecificDeploymentExporter` — optional versioned export

## Implementations

### httpstate/ (Pulumi Cloud)

- HTTP REST API via `client/client.go` → `NewClient(apiURL, apiToken, insecure, diag)`
- Auth: bearer token from `~/.pulumi/credentials.json` via `workspace.GetAccount(cloudURL)`
- OAuth browser login with nonce-based security (local webserver on random port)
- Update tokens for in-progress operations (separate from auth token)
- Journal-based persistence: batch entries sent via API, configurable via `PULUMI_JOURNALING_BATCH_SIZE` / `PULUMI_JOURNALING_BATCH_PERIOD`
- Handles 413 (Too Large) by recursive batch splitting

### diy/ (Self-Hosted)

- Pluggable storage via `Bucket` abstraction:
  - `file://` — local filesystem
  - `s3://` — AWS S3
  - `gs://` — Google Cloud Storage
  - `azblob://` — Azure Blob Storage
  - `postgres://` — PostgreSQL (custom driver in `postgres/`)
- No HTTP auth needed; filesystem/database permissions apply
- Optional passphrase-based secret encryption
- Per-stack file locking for concurrent access

## State Persistence

```
Step execution
  → SnapshotManager (tracks mutations, NOT thread-safe)
    → SnapshotPersister.Save(deployment)
      → ValidatingPersister (integrity check wrapper)
        → httpstate: journal API call
        → diy: checkpoint file write (.json/.gzip)
```

**Journal entries** (`journal.go`): serialized via `SerializeJournalEntry()` with encryption, version-tracked.

## Display Subsystem

`display/` converts engine events to CLI output:

- `ShowEvents()` — main entry, routes to JSON or human-readable
- Display types: `DisplayProgress` (live spinner), `DisplayDiff` (hierarchical diff), `DisplayWatch` (file monitor)
- `Options` struct controls: `Color`, `JSONDisplay`, `IsInteractive`, `SuppressOutputs`, etc.

## Error Types

`backenderr/backenderr.go` defines backend-specific errors:

```go
var ErrNotFound NotFoundError           // errors.Is() compatible
var ErrLoginRequired LoginRequiredError // matches registry.UnauthorizedError
var ErrForbidden ForbiddenError

// Also:
StackAlreadyExistsError{StackName}
OverStackLimitError{Message}
ConflictingUpdateError{Err}             // another update in progress
```

All implement `Unwrap()`, `Is()` for `errors.Is()`/`errors.As()` chains.

## Testing

`mock.go` provides `MockBackend` with function pointers for every interface method:

```go
be := &MockBackend{
    GetStackF: func(ctx context.Context, ref StackReference) (Stack, error) {
        return &MockStack{SnapshotF: func(...) (*deploy.Snapshot, error) { ... }}, nil
    },
}
```

Pattern: set only the callbacks you need; unconfigured methods panic.
