# Engine

## Deployment Flow

Entry points with identical signatures:
```go
func Update(u UpdateInfo, ctx *Context, opts UpdateOptions, dryRun bool) (*deploy.Plan, display.ResourceChanges, error)
func Destroy(u UpdateInfo, ctx *Context, opts UpdateOptions, dryRun bool) (...)
func Refresh(u UpdateInfo, ctx *Context, opts UpdateOptions, dryRun bool) (...)
```

**Internal flow:**
1. Create `deploymentContext` (OpenTracing + OTel spans)
2. Create `eventEmitter` with buffered channel (`events.go:434`)
3. Create `deployment` struct (`deployment.go:179`)
4. `deployment.run()` spawns goroutine for `deploy.Deployment.Execute(ctx)`
5. Select on: panic errors, cancellation, or completion
6. Emit summary event with duration and resource changes

**Key internal components** (see `developer-docs/architecture/resource-registration.md`):
- **Resource monitor**: gRPC `ResourceMonitor` service — shim between language SDKs and engine. Handles default provider resolution (one per package+version) and dispatches `Construct` for multi-language components.
- **Step generator**: Processes `RegisterResourceEvent`s **serially** (critical path). Issues steps fire-and-forget to executor.
- **Step executor**: Executes step chains in parallel via worker pool.

**Refresh gotcha**: During refresh, the Diff call has an **inverted polarity** — Read results are passed as `olds` and pre-refresh inputs as `news`. The CLI then reverses the display so the user sees the expected direction.

## Event System

`events.go` defines event types:

| Event | When |
|-------|------|
| `PreludeEvent` | Operation start (config info) |
| `ResourcePreEvent` | Before step execution |
| `ResourceOutputsEvent` | After step (outputs available) |
| `ResourceOperationFailed` | Step failure |
| `SummaryEvent` | Operation end (duration, changes) |
| `DiagEvent` | Diagnostics (debug/info/warn/error) |
| `PolicyViolationEvent` | Policy check failure |
| `ProgressEvent` | Downloads, installs |
| `CancelEvent` | Cancellation signal |

Events flow through a buffered channel via `queueEvents()` goroutine to prevent blocking senders.

**State locking**: `StepEventMetadata` uses `LockState()`/`UnlockState()` to prevent concurrent mutation of resource state during event emission.

## Plugin Management

`plugins.go` — `PluginManager` interface:
- `GetPluginPath()`, `DownloadPlugin()`, `InstallPlugin()`
- Default impl delegates to workspace methods
- `PluginSet` map with `Deduplicate()` to remove less-specific entries

Plugin gathering (`update.go:268-331`):
1. Query language host for program requirements
2. Query snapshot manifest + provider versions
3. Merge via `Union()` + `Deduplicate()`
4. Background async installation via `installManager`

## Cancellation Model

Three-level context hierarchy:

1. **Caller context** (`Context.Cancel`): `Canceled()` channel for graceful stop, `Terminated()` for force stop
2. **Plugin context**: `context.WithoutCancel(ctx)` — decoupled so providers shut down gracefully after main cancel
3. **Source context**: `context.WithCancel()` — signals to deployment source, canceled on caller cancel

The plugin context goroutine (`deployment.go:212`) keeps plugins alive until `Terminated()` fires.

## Engine ↔ Deploy Interaction

`updateActions` (`update.go:640`) implements `deploy.Events`:
- `OnResourceStepPre(step)` — emits `ResourcePreEvent`
- `OnResourceStepPost(ctx, step, status, err)` — emits outputs or failure event
- `OnResourceOutputs(step)` — emits `ResourceOutputsEvent`
- `OnSnapshotWrite(snap)` — persists via `SnapshotManager`

## Error Handling

- `DecryptError` (`errors.go`): config key can't be decrypted, has `Key` field + wrapped `Err`
- `AsDecryptError(err)`: helper for type assertion
- `trySendEvent()` (`events.go:908`): recovers from panic on closed channel (handles Ctrl+C races)
- Partial failure: on `StatusPartialFailure`, preserves old inputs with new outputs

## Lifecycle Tests

`lifecycletest/framework/framework.go` — test framework for engine operations:

```go
type TestOp func(UpdateInfo, *Context, UpdateOptions, bool) (*deploy.Plan, display.ResourceChanges, error)
```

- `op.Run(project, target, opts, dryRun, validate)` — execute with snapshot validation
- Uses `TestJournal` (in-memory) + `ValidatingPersister` + `SnapshotManager`
- Validates snapshot integrity at every journal entry point
- Compares snapshots from journal, snapshot manager, and persister

## Fuzzing

`lifecycletest/fuzz_test.go` — property-based testing with Rapid:
- Gated by `PULUMI_LIFECYCLE_TEST_FUZZ` env var
- Generates random fixtures via `fuzzing.GeneratedFixture()`
- Exclusion rules in `fuzzing/exclude.go` (16+ known bugs)
- Reproduction code written to `PULUMI_LIFECYCLE_TEST_FUZZING_REPRO_DIR`
- Also supports loading real state files via `PULUMI_LIFECYCLE_TEST_FUZZ_FROM_STATE_FILE`
