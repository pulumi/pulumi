(state-migrations)=
# Component state migrations

Component state migrations let a component reshape its persisted resource state
before the engine compares it with the component's current registrations. A
migration can rename, split, merge, or otherwise reorganize the component and
its descendants without treating the change as unrelated deletes and creates.

Migrations change Pulumi's state, they do not call providers. A managed custom
resource and its successor must retain the same physical ID, provider,
extension, ownership, and provider lifecycle state.

For example, suppose component `Root` previously contained `OldChild`, but its
new implementation replaces that child with `NewChild`. A migration returns the
updated subtree `[Root, NewChild]` and identifies `NewChild` as `OldChild`'s
successor. The engine can then rewrite references from `OldChild` to `NewChild`
without deleting and recreating the physical resource.

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
5. Prepares the complete post-migration prior-resource list, topologically sorts
   it, persists it, and commits one `StateMigrationTransaction`.
6. Retains the successor rewrite for references the program may send after the
   commit.
7. Normalizes the registration goal using the saved rewrites.
8. Resumes planning the registration against the migrated state.

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
`OldChild` is replaced by `NewChild`, the callback maps `OldChild` to
`NewChild`. References to `OldChild` in parents, dependencies, typed resource
references, output dependencies, and other reference-bearing state fields are
rewritten to `NewChild`. Plain strings that happen to contain a URN are not
interpreted as references.

Successor mappings compose across callbacks. For example, if one callback maps
`OldChild` to `IntermediateChild` and the next maps `IntermediateChild` to
`NewChild`, the engine rewrites references from `OldChild` directly to
`NewChild`.

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
[Provider, Root, OldChild, Consumer, Tail]
```

`Root` is the component root, and `OldChild` and `Tail` are its children.
`Consumer` is outside the migrated subtree but is interleaved with its
resources: `Consumer` depends on `OldChild`, and `Tail` depends on `Consumer`.
The callback returns `[Root, NewChild, Tail]` and maps `OldChild` to `NewChild`.

The replacement subtree cannot be inserted as one contiguous block. Placing it
at the old subtree's last position would put `Consumer` before its new
dependency `NewChild`, while placing it at the first position would put `Tail`
before its dependency `Consumer`. The engine rewrites `Consumer`, builds the
complete candidate snapshot, and topologically sorts all of its resources. The
prepared resource values are:

```text
[Provider, Root, NewChild, Consumer′, Tail]
```

where `Consumer′` is a prepared copy of `Consumer` whose dependency refers to
`NewChild`. If this resource graph cannot be sorted, preparation fails without
persisting or committing the migration. Otherwise, the transaction records
both the values to persist and the information needed to commit them without
replacing live resource pointers:

| Field | Value in the example | Purpose |
|---|---|---|
| `PriorSubtree` | `[Root, OldChild, Tail]` | Live states from `Deployment.prev.Resources` replaced by the migration |
| `ResultSubtree` | `[Root, NewChild, Tail]` | Replacement subtree |
| `SuccessorURNs` | `{OldChild: NewChild}` | Maps removed resources to their successors |
| `PreparedPriorResources` | `[Provider, Root, NewChild, Consumer′, Tail]` | Topologically sorted replacement for `Deployment.prev.Resources` |
| `RetainedResourceRewrites` | `{Consumer: Consumer′}` | Prepared values to copy into retained live objects |
| `currentResourceRewrites` | As needed | Prepared values for earlier `Deployment.news` and `Deployment.reads` states |

`Deployment.prev.Resources` and other deployment bookkeeping may refer to the
same `*pkgresource.State` object. If the engine replaced `Consumer` in
`Deployment.prev.Resources` with the new `Consumer′` object, other structures
could still refer to the original `Consumer` and see its old references. The
engine therefore validates and persists `Consumer′`, then copies its fields
into the original `Consumer`. It sets `Deployment.prev.Resources` to
`[Provider, Root, NewChild, Consumer, Tail]` and rebuilds the deployment
indexes. The persisted and in-memory states have the same values while shared
pointer identity is preserved.

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

* `Layout`: the complete post-migration base snapshot order. Each item refers
  either to a retained resource by its index in the original base snapshot or
  to an inserted resource by its index in `States`. Base resources omitted from
  the layout are removed.
* `States`: the complete replacement subtree. `stateIndex` items in `Layout`
  refer to resources in this list.
* `BaseStatePatches`: complete replacements for retained base states whose
  references changed, addressed by their original base-snapshot index.
* `NewStatePatches`: complete replacements for earlier operation results,
  addressed by operation ID.

These patches are complete resource states, not property-level diffs.

For the example above, the immutable base snapshot assigns indices `0` through
`4` to `[Provider, Root, OldChild, Consumer, Tail]`. The migration entry has
this shape:

```json
{
  "kind": "state-migration",
  "states": ["Root", "NewChild", "Tail"],
  "layout": [
    {"baseIndex": 0},
    {"stateIndex": 0},
    {"stateIndex": 1},
    {"baseIndex": 3},
    {"stateIndex": 2}
  ],
  "baseStatePatches": [
    {"index": 3, "state": "Consumer′"}
  ]
}
```

The strings stand in for complete serialized resource states. Replay applies
the patches and walks `Layout` to reconstruct
`[Provider, Root, NewChild, Consumer′, Tail]` exactly; the omitted base indices
for `Root`, `OldChild`, and `Tail` remove the prior subtree.

If the journal entry is acknowledged before a crash, replay recovers the exact
prepared result. If persistence fails, there is no migration to commit.

## References sent after commit

Planning and step execution overlap while the program runs. Before a migration,
the engine stops accepting source events, lets earlier planning and step chains
finish, and blocks new execution while the migration commits. The transaction
rewrites all engine-held state before work resumes, so engine-generated
continuations do not need separate normalization.

The program may still hold a reference created before the migration and send it
to the engine afterward. The deployment keeps a small `stateMigrationRewrite`
with the successor URNs and physical identity needed to normalize such values.
`ReadResource` and `RegisterResourceOutputs` events are normalized when they
enter the engine. Registration goals are normalized later, after applying any
migrations carried by that registration, because its own migration may create
the required successor mapping.

URNs used for identity lookup are handled separately. A registration's parent
is rewritten before generating its URN, and its aliases are rewritten before
looking up prior state. The predecessor URN cannot be reused as the primary URN
of a new registration or read during the same deployment. References to that
URN now mean the successor, so reusing it would be ambiguous.

Providers and analyzers receive normalized inputs and old state and are expected
to derive resource references from those values.
