(state-migrations)=
# Component state migrations

Component state migrations let a component reshape its persisted resource state
before the engine compares it with the component's current registrations. A
migration can rename, split, merge, or otherwise reorganize the component and
its descendants without treating the change as unrelated deletes and creates.

Migrations change Pulumi's state, they do not call providers. A managed custom
resource and its successor must retain the same physical ID, provider,
extension, ownership, and provider lifecycle state.

For example, suppose component `A` previously contained child `B`, but its new
implementation replaces that child with `D`. A migration returns the updated
subtree `[A, D]` and identifies `D` as `B`'s successor. The engine can then
rewrite references from `B` to `D` without deleting and recreating the physical
resource.

## How migrations run

Migrations use the engine's
[callback system](/docs/architecture/deployment-execution/callbacks.md). The
callbacks run in the language SDK process, and a resource registration can
carry an ordered list of them. While planning that registration, the step
generator:

1. Resolves the prior subtree root using the registration URN and its aliases.
2. Captures that root and its transitive descendants from
   `Deployment.prev.Resources` in snapshot order.
3. Converts those `pkgresource.State` values to `ResourceV3` checkpoint JSON and
   invokes each callback in order.
4. Converts the result back to resource state and validates the returned
   subtree and successor mappings.
5. Prepares, persists, and commits one `StateMigrationTransaction`.
6. Publishes a successor rewrite for state produced after the commit.
7. Resumes planning the registration against the migrated state.

## Callback contract

In simplified form, the Python SDK callback types and a synchronous migration
have this shape:

```python
class StateMigrationArgs:
    urn: str
    old_state: list[dict[str, Any]]

class StateMigrationResult:
    new_state: list[dict[str, Any]]
    successors: dict[str, str] | None

def migrate(args: StateMigrationArgs) -> StateMigrationResult | None:
    return StateMigrationResult(
        new_state=[...],
        successors={old_urn: new_urn},
    )
```

An asynchronous migration may return the same result from an `Awaitable`.

`args.urn` is the URN of the current registration. `args.old_state` contains the
prior root and all of its descendants in checkpoint format.[^aliased-root]

Returning `None` means that the state is already in the desired shape.
Otherwise, `new_state` replaces that complete prior subtree and `successors`
maps each omitted prior URN to a URN present in `new_state`. The engine also
treats a result that represents the same state as the input as a no-op. No-op
migrations skip persistence and commit.

Migration callbacks remain attached to the resource after a stack has migrated,
so they must be idempotent. A callback should recognize state in its final shape
and return `None`, so later updates do not repeat the state change.

A callback must only transform the supplied checkpoint data. It must not perform
Pulumi runtime operations[^monitor-reentrancy] or wait for unresolved `Output`
values. The resource monitor waits for the callback to return, so work that
calls back into the monitor would deadlock.

Any secrets in the supplied checkpoint data are decrypted before the callback
runs. The callback must treat those values as sensitive.

[^aliased-root]:
    When the engine resolves the prior root through an alias, `args.urn`
    identifies the current registration while `args.old_state[0]["urn"]` keeps
    the matched prior resource's URN. The two URNs may therefore differ.

[^monitor-reentrancy]:
    Where possible, language SDKs should detect and reject attempts to call back
    into the resource monitor from a migration callback, producing a useful
    error instead of allowing the program to deadlock.

## Results and successors

Every resource in the prior subtree must either remain in the result or map to a
successor in the result. This makes removal explicit: accidentally omitting a
resource cannot silently forget a physical object that Pulumi manages.

Successors also tell the engine how to rewrite references. For example, if
child `B` is replaced by child `D`, the callback maps `B` to `D`. References to
`B` in parents, dependencies, typed resource references, output dependencies,
and other reference-bearing state fields are rewritten to `D`. Plain strings
that happen to contain a URN are not interpreted as references.

Successor mappings compose across callbacks. For example, if one callback maps
`B` to `C` and the next maps `C` to `D`, the engine rewrites references from
`B` directly to `D`.

## Safety validation

Before preparing a transaction, the engine validates the callback result as a
complete replacement subtree:

* It must contain exactly one resource at the top of the replacement subtree:
  either the current registration (`args.urn`) or its matched prior resource
  (`args.old_state[0]["urn"]`), but not both.
* That resource must keep its prior parent, and every other returned resource
  must be its descendant.
* Returned URNs must remain in the registration's stack and project, and all
  structural references must resolve in either the returned subtree or the
  retained prior state.
* Provider resource states must be returned unchanged. A checkpoint rewrite
  cannot update the engine's live provider registry.
* Every returned custom resource must have a retained or explicitly mapped
  custom predecessor. Migrations cannot import or fabricate managed resource
  state.
* A custom resource and its successor must preserve physical ID, provider and
  extension identity, `External` ownership, `PendingReplacement`, and `Taint`.
  `Protect` and `RetainOnDelete` are inherited conservatively: a many-to-one
  successor carries either flag if any predecessor had it.

Two custom resources can share a provider and physical ID while representing
different parts of the same object. The engine cannot tell whether such a
cross-type merge or rename is correct; the callback must choose the right
successor.

## When migrations can run

Migrations are considered during updates and update previews when prior state
exists. Migration callbacks do not run during refresh or destroy, even with
`--run-program`.[^non-update-operations] The selected persistence mode must also
support state migrations; full-checkpoint snapshot persistence and journal v2
do, while older journal formats do not.

A state-changing result is rejected when the surrounding update cannot safely
rewrite the complete checkpoint. This includes:

* updates constrained by `--target`, `--replace`, `--exclude`, or
  `--target-snippet`
* generating or applying a saved update plan
* snapshots with pending operations, or migrated subtrees with resources
  pending deletion from an earlier update
* migrations involving resource views
* persisted snippets whose references would need to be rewritten

These restrictions apply only when a callback changes state. A permanently
attached callback can return `None` without blocking partial updates, plan use,
or recovery of pending state.

[^non-update-operations]:
    Migrations reshape prior state before update planning. Refresh instead
    reconciles that state with providers, while destroy removes it.
    `--run-program` evaluates the program but does not turn either operation
    into an update.

## Preparing the transaction

The engine validates and prepares a
[`StateMigrationTransaction`](https://github.com/pulumi/pulumi/blob/7551227c26a1a506f44dc19ef5dad91acd15a1c9/pkg/resource/deploy/state_migration_models.go#L60)
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
| `ResultSubtree` | `[A, D]` | Replacement subtree |
| `SuccessorURNs` | `{B: D}` | Maps removed resources to their successors |
| `PreparedPriorResources` | `[P, A, D, C′]` | Complete replacement for `Deployment.prev.Resources` |
| `RetainedResourceRewrites` | `{C: C′}` | Prepared values to copy into retained live objects |
| `currentResourceRewrites` | As needed | Prepared values for earlier `Deployment.news` and `Deployment.reads` states |

`Deployment.prev.Resources`, planned steps, and snapshot bookkeeping may all
refer to the same `*pkgresource.State` object. If the engine replaced `C` in
`Deployment.prev.Resources` with the new `C′` object, planned work would still
refer to the original `C` and see its old references. The engine therefore
validates and persists `C′`, then copies its fields into the original `C`. It
sets `Deployment.prev.Resources` to `[P, A, D, C]` and rebuilds the deployment
indexes. The persisted and in-memory states have the same values, while existing
work continues to refer to the original `C` object.

## Persistence

During a real update, persistence happens before the engine changes
`Deployment.prev` or any other shared resource state. If persistence fails, the
migration returns an error and the live deployment remains unchanged. A preview
performs the same preparation and in-memory commit without durable persistence.

Once a migration is persisted, it is not rolled back if a later part of the
update fails. The next update starts from the migrated state, so migration
callbacks must be idempotent.

The full-checkpoint snapshot manager combines `PreparedPriorResources` with its
current operation state, then saves the result through the usual persistence
path.

The journal manager starts with an immutable base snapshot and records ordered
operations on top of it. A migration may change both the base and resources
produced by earlier journal entries, so journal v2 records one migration entry
containing:

* `RemoveOlds`: indices of prior-subtree resources in the original base
  snapshot
* `States`: the complete subtree inserted in their place
* `BaseStatePatches`: complete replacements for retained base states whose
  references changed
* `NewStatePatches`: complete replacements for earlier operation results,
  addressed by operation ID

These patches are complete resource states, not property-level diffs.

If the journal entry is acknowledged before a crash, replay recovers the exact
prepared result. If persistence fails, there is no migration to commit.

## State produced after commit

Planning and step execution overlap while the program runs. Before a migration,
the engine lets previously accepted planning and step chains finish, then pauses
new work while the migration commits. This prevents the migration from
overtaking or observing a partially completed step chain.

Some resource state may be produced after the commit[^late-state] and still
contain predecessor references captured before the migration. The deployment
keeps a small `stateMigrationRewrite` with the successor URNs and physical
identity needed to normalize that state.

The rewrite also lets a later registration use a predecessor as an alias and
find its successor. The predecessor URN cannot be reused as the primary URN of a
new registration or read during the same deployment. References to that URN now
mean the successor, so reusing it would be ambiguous.

[^late-state]:
    This includes state produced by asynchronous refresh or import work,
    `Step.Apply`, `RegisterResourceOutputs`, and provider-published view steps.
