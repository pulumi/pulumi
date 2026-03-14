# Resource Deploy

## Step Interface

`step.go` defines the `Step` interface (11 implementations):

| Type | Op | Purpose |
|------|----|---------|
| `SameStep` | Same | Resource unchanged, retain state |
| `CreateStep` | Create | New resource (also `CreateReplacementStep`) |
| `DeleteStep` | Delete | Remove resource (also `DeleteReplacementStep`) |
| `UpdateStep` | Update | In-place update |
| `ReplaceStep` | Replace | Marks resource for replacement |
| `RemovePendingReplaceStep` | RemovePendingReplace | Clears pending replacement flag |
| `ReadStep` | Read | Reads external resource state |
| `RefreshStep` | Refresh | Syncs state from provider (async via promise) |
| `ImportStep` | Import | Imports existing resource (async via promise) |
| `DiffStep` | — | Async diff computation |
| `ViewStep` | — | Component resource views |

All steps implement: `Apply()`, `Op()`, `URN()`, `Type()`, `Old()`, `New()`, `Res()`, `Logical()`, `Deployment()`.

## Step Generator Algorithm

`step_generator.go` — core decision tree for what to do with each resource.

**Critical design constraint:** The step generator processes `RegisterResourceEvent`s **serially**. This is on the deployment's critical path — any blocking in the generator slows the entire deployment. Steps issued to the executor are **fire-and-forget**; the generator moves on immediately.

```
GenerateSteps(event)
  → generateSteps()
    → Generate URN from type + name + parent
    → getOldResource() via URN/alias lookup
    → Pre-process ignoreChanges (copy old values for ignored properties)
    → if import: async ImportStep → continueStepsFromImport()
    → Call provider Check(inputs, oldInputs) → checked inputs
    → Run analyzers for policy validation
    → if no old: CreateStep (or SkippedCreate if not targeted)
    → if old: diff resource:
      1. Check if marked for replacement (--target-replace)
      2. Check if provider changed (forces replace)
      3. Call provider Diff(checkedInputs, oldState, ignoreChanges)
      4. Apply replace-on-change options
    → no changes: SameStep
    → changes + no replace: UpdateStep
    → changes + replace:
      → Re-Check with empty old inputs (fresh defaults for replacement)
      → if createBeforeDelete: CreateStep → DeleteStep (later)
      → if deleteBeforeReplace: calculate dependent replacements →
          DeleteSteps for dependents → DeleteStep → CreateStep
```

**After program exit:** Generator computes resources to delete by diffing registered vs existing sets, decomposes into antichains (parallel-safe groups) using poset decomposition, issues delete antichains in reverse dependency order.

Key fields on `stepGenerator`:
- `mode`: updateMode / destroyMode / refreshMode
- `urns`, `reads`, `deletes`, `replaces`, `updates`, `creates`, `sames`: tracking maps
- `skippedCreates`: resources not created due to `--target`
- `refreshAliasLock`: mutex protecting `depGraph.Alias()` calls in goroutines

## Step Executor Parallelism

`step_executor.go` uses a chain/antichain model:

- **Chain**: sequence of steps executed serially (e.g., Create → Replace → Delete)
- **Antichain**: set of steps executed in parallel (independent resources)

```go
executor.ExecuteSerial(chain)    // submit chain, returns completion token
executor.ExecuteParallel(anti)   // submit antichain, waits for all
```

Worker goroutines acquire `workerLock.RLock()` per step. The write lock is used for cancellation/shutdown.

## Concurrency — Watch Out

1. **`refreshAliasLock`** (`step_generator.go:88`): Protects `depGraph.Alias()` calls in goroutines. Comment at line 771 explains the race.

2. **State mutation race** (`step_executor.go:500`): ReplaceStep state mutation skipped to avoid data race (issue #14994).

3. **Parent registration race** (`step_generator.go:221`): Stack resource races step executor writing state.

4. **Plan copy race** (`plan.go:82-83`): Concurrent goroutines copying plan state (issue #21681).

5. **Async promises**: RefreshStep and ImportStep use `promise.CompletionSource` with goroutines that block on results. If promise never resolves, goroutine leaks.

## Concurrent-Safe Types

- `goals`, `news`, `reads`: `gsync.Map` (concurrent map)
- `pendingNews`: `gsync.Map` in step executor
- Provider registry: `sync.Mutex` protected
- Per-resource: `resource.State.Lock` (RWMutex) for output marking

## Testing with deploytest/

Mock implementations for testing:

| File | Mock Type |
|------|-----------|
| `deploytest/provider.go` | `Provider` with `CheckF`, `DiffF`, `CreateF`, `UpdateF`, `DeleteF` callbacks |
| `deploytest/pluginhost.go` | Plugin host |
| `deploytest/languageruntime.go` | Language runtime |
| `deploytest/resourcemonitor.go` | Resource monitor |
| `deploytest/analyzer.go` | Analyzer plugin |
| `deploytest/backendclient.go` | Backend client |
| `deploytest/sink.go` | `NoopSink` diagnostic sink |

Pattern: set callback fields to customize behavior per test.

## Providers Registry

`providers/registry.go` manages provider lifecycle:
- Thread-safe with `sync.Mutex`
- Tracks configured vs unconfigured providers
- Internal metadata stored in `__internal` property map
- `ProviderRequest` is the caching key (version + name + URL + checksums)
