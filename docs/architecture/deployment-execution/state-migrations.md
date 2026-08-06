(state-migrations)=
# Component state migrations

Component state migrations let a component change the shape of its previously
persisted resource state before the engine compares that state with the
component's current registrations. A migration can rename, split, merge, or
otherwise reorganize the component and its descendants without treating the
change as a set of unrelated deletes and creates.

Migrations edit Pulumi's state; they do not directly perform provider
operations. A prior managed custom resource and its successor must retain the
same physical ID, provider, extension, ownership, and provider-lifecycle state.

For example, suppose component `A` previously contained child `B`, but its new
implementation replaces that child with `D`. A migration returns the updated
subtree `[A, D]` and identifies `D` as `B`'s successor. The engine can then
rewrite references from `B` to `D` without deleting and recreating the physical
resource.

## Migration execution flow

Migrations use the engine's [callback system](callbacks): the callback runs in
the language SDK process and the engine invokes it across the callback service.
A component registration can carry an ordered list of these callbacks. Before
old-state lookup and provider diffing for that registration, the step generator:

1. Resolves the prior component root using the current URN and its aliases.
2. Captures that root and its transitive descendants from
   `Deployment.prev.Resources` in snapshot order.
3. Serializes those `pkgresource.State` values as `ResourceV3` checkpoint JSON
   and invokes each callback in order.
4. Deserializes and validates the final returned subtree and its successor
   mappings.
5. Prepares, persists, and commits one `StateMigrationTransaction`, including
   rewrites for state that has already materialized.
6. Publishes the successor rewrite rule `stateMigrationRewrite` for state that
   materializes after the commit.
7. Resumes registration planning against the migrated prior state.

After commit, the deployment applies the published `stateMigrationRewrite` to
registrations, step results, outputs, and continuations whose final state
materializes later.

A callback may return no state when no migration is needed. If the final
normalized result equals the input state, the engine also treats the callback
chain as a no-op and skips persistence and commit.

## Result state and successors

In the Python SDK, a synchronous migration has this shape (asynchronous
migrations may return the same result from an `Awaitable`):

```python
def migrate(args: StateMigrationArgs) -> StateMigrationResult | None:
    return StateMigrationResult(
        new_state=[...],
        successors={old_urn: new_urn},
    )
```

`args.urn` is the component being migrated. `args.old_state` is the prior
component and all of its transitive descendants in checkpoint format.
Returning `None` means that the state is already in the desired shape.
Otherwise, `new_state` replaces that complete prior subtree and `successors`
maps each omitted prior URN to a URN present in `new_state`.

Every resource in the prior subtree must either remain in the returned state or
name a successor that is present in the returned state. This makes removing
state explicit: accidentally omitting a resource cannot silently forget a
physical object that Pulumi manages.

Successors also tell the engine how to rewrite references. For example, if
child `B` is replaced by child `D`, the callback maps `B` to `D`. References to
`B` in parents, dependencies, provider references, resource references, and
other reference-bearing state fields are rewritten to `D`.

When several callbacks run, a later callback may replace a resource returned
by an earlier callback. The engine uses the complete callback-chain mapping to
normalize the final result subtree, then collapses the chain into a direct
original-to-final mapping for state outside that subtree and for state produced
later in the deployment.

## The prepared transaction

The engine validates and prepares a complete
[`StateMigrationTransaction`](gh-file:pulumi#pkg/resource/deploy/state_migration_model.go#L62)
before mutating shared deployment state. Suppose
`Deployment.prev.Resources` is:

```text
[P, A, B, C]
```

`A` is the component root, `B` is its child, and `C` is outside the migrated
subtree but depends on `B`. The callback returns `[A, D]` and maps `B` to `D`.
The prepared resource values are therefore:

```text
[P, A, D, C′]
```

where `C′` is a prepared copy of `C` whose dependency refers to `D`. The
transaction records both the values to persist and the information needed to
commit them without replacing live resource pointers:

| Field | Value in the example | Purpose |
|---|---|---|
| `PriorSubtree` | `[A, B]` | Live states from `Deployment.prev.Resources` replaced by the migration |
| `ResultSubtree` | `[A, D]` | Normalized replacement subtree |
| `SuccessorURNs` | `{B: D}` | Direct original-to-final reference rewrite |
| `PreparedPriorResources` | `[P, A, D, C′]` | Complete replacement for `Deployment.prev.Resources` |
| `RetainedResourceRewrites` | `{C: C′}` | Prepared values to copy into retained live objects |
| `currentResourceRewrites` | As needed | Prepared values for earlier `Deployment.news` and `Deployment.reads` states |

`Deployment.prev.Resources`, planned steps, and snapshot bookkeeping may all
hold pointers to the same `*pkgresource.State`. Replacing retained resource
`C`'s pointer would leave existing work observing its old references. The
transaction therefore prepares `C′` as a separate value for validation and
persistence, then commit copies its fields into the original `C` object.

The snapshot manager validates and persists `[P, A, D, C′]` without changing
objects already referenced by the deployment, planned steps, or snapshot
bookkeeping. These prepared copies form the transaction's rollback boundary.
After persistence succeeds, commit performs that in-place copy, sets
`Deployment.prev.Resources` to `[P, A, D, C]`, and rebuilds its indexes. The
persisted and in-memory states are value-equivalent, but the latter preserves
`C`'s pointer for work planned before the migration.

## Persist before committing

During a real update, persistence happens before the engine changes
`Deployment.prev` or any other shared resource state. If persistence fails, the
migration returns an error and the live deployment remains unchanged. A preview
performs the same preparation and in-memory commit without durable persistence.

The traditional snapshot manager constructs the complete post-migration
checkpoint from `PreparedPriorResources` and its current operation inventory.
It saves that checkpoint through the ordinary encryption, integrity checking,
repair, and persistence path.

The journal manager starts with an immutable base snapshot and records ordered
operations on top of it. Because a migration can change both that base and
resources produced by earlier journal entries, journal v2 records one state
migration entry containing:

* `RemoveOlds`: indices of prior-subtree resources in the original base
  snapshot;
* `ResultStates`: the complete subtree inserted in their place;
* `BaseStatePatches`: complete replacements for retained base states whose
  references changed; and
* `NewStatePatches`: complete replacements for earlier operation results,
  addressed by operation ID.

These patches are complete resource states, not property-level diffs.

This ordering gives useful crash behavior: a failure before persistence leaves
no migration to commit, while a crash after the journal entry is acknowledged
can recover the exact prepared result by replay.

## Concurrency and state produced later

Planning and step execution overlap while the program runs. A migration
therefore waits for earlier asynchronous planner continuations and takes the
step executor's exclusive lock. This prevents it from overtaking an accepted
serial step chain or observing a replacement chain halfway through.

The transaction prepares every relevant state value that exists at commit time.
Other final values may materialize later from refresh or import continuations,
`Step.Apply`, `RegisterResourceOutputs`, or provider-published view steps. They
may contain predecessor references captured before the migration. After commit,
the deployment retains a small `stateMigrationRewrite` containing successor
URNs and physical identity to normalize those later values.
