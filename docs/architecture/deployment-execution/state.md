(state-snapshots)=
# State management and snapshots

*State* is the metadata that Pulumi stores about the infrastructure it manages,
and, among other things, is key to enabling Pulumi to work out when to create,
update, replace and delete resources. A *snapshot* is a view of a Pulumi state
at a particular point in time. State is stored in a [backend](backends) that can
be configured on a per-project basis. For these purposes, state is typically
serialized to a JSON format; this is also the format used by the `stack export`
and `stack import` CLI commands.

(backends)=
## Backends

A *backend* is an API and storage endpoint used by the Pulumi CLI to coordinate
updates, reading and writing stack state whenever appropriate.

(diy)=
### DIY backends

A *DIY* (*do it yourself*) backend is one in which a state JSON file is
persisted to a medium controlled and managed by the Pulumi user. Under the hood,
Pulumi uses the [Go Cloud Development Kit](https://gocloud.dev/) (specifically,
its [`blob` package](https://gocloud.dev/howto/blob/)) to support a number of
storage implementations, from local files to cloud storage services such as AWS
S3, Google Cloud Storage, and Azure Blob Storage.

(httpstate)=
### HTTP state backends

An *HTTP state backend* is one in which the state is managed by API calls to a
remote HTTP service, which is responsible for managing the underlying state.
[Pulumi Cloud](https://www.pulumi.com/product/pulumi-cloud/) is the primary
example of this.

(snapshots)=
## Snapshots

The Pulumi CLI generates a new snapshot at the start and at the end of each
operation, unless the `PULUMI_SKIP_CHECKPOINTS` environment variable is set.

Currently we upload either a full snapshot, or the diff to the previous snapshot
if the HTTP state backend is used, and the snapshot is sufficiently large.

Depending on the step type, at the start of the operation we create a "pending
operation" entry. This is created, so if anything goes wrong during the operation,
e.g. the connection drops while we set up a resource, we have a record of it, and
the user can manually go through them and either delete the entry if the resource
has not been created in the provider, or import the resource into the state. At
the end of each operation, we finalize the entry, and add it to the list of
resouces, while removing the "pending operation" entry.

Note that the engine is currently free to modify the snapshot in any way, and we'll
always upload that internal snapshot, since the snapshot manager internally uses
a pointer to the same snapshot as the engine is using. The engine makes use of this
for example for marking snapshot entries as `Delete=true` or `PendingReplacement=true`,
as well as for changing outputs due to a `RegisterResourceOutputs` call. It's also
used for refreshes, and for default provider updates.

Another important thing to note here is that each of the updates to the snapshot
needs to happen sequentially. This is so the snapshot is always consistent. If we
were to do snapshot updates in parallel, it would be possible that we overwrite
a snapshot with more information, with one that has been generated earlier, that
doesn't include the latest updates yet.

(snapshot-journaling)=
### Snapshot Journaling

To avoid the problem of not being able to send snapshot updates in parallel, and
to save on bandwidth costs, we are implementing a journaling approach. Instead of
sending the full snapshot we can create a journal entry for each step, and replay
them in sequence on top of the current snapshot, to create a new valid snapshot.

This can be done due to the fact that that the engine and the snapshot manager can
start with the same base snapshot, on top of which the entries are applied. And
because the engine never starts a new update before any of the dependents have
completed, it is always safe to apply all current journal entries in order, to
reconstruct the snapshot. In particular this is different from the snapshotting
implementation in that we can't rely on the engine setting fields in the snapshot
anymore, but need to encode all of this information in the journal entries.

In the first pass of the implementation, we still send the whole snapshot to the
backend. In later stages of the implementation, backends that can handle it, can
receive the journal entries directly, and rebuild the snapshot on the backend.

To make sure we always operate on the same snapshot the backend has access to,
and to be able to confidently test the implementation against the lifecycletests,
the journaler only has access to a copy of the snapshot, where all resources are
deep copied. This way we make sure that we don't make use of snapshot entries
being modified by the engine.

The snapshot may be updated once at the beginning, for provider migrations. In
that case we do a "write" operation update the snapshot. It is not valid to do
this after any journal entries have been created, because resources might now be
in different places in the resource list.

#### Journal entry details

There's various types of Journal entries, all with slightly different semantics.
This section describes them. All journal entries are associated with increasing
sequence IDs. IDs start at 1. These sequence IDs are used for:
- Ordering the journal entries on the backend side. We could do this by
  timestamps, but since we have IDs, those are definitely unambiguous, and
  easier to deal with.
- Removing any journal entries that came up again later. The `DeleteNew` field
  in particular contains the ID of previous journal entries that are no longer
  relevant as they were superseeded.

Journal entries associated with Pulumi Operations additionally have an Operation
ID assigned to them. This is used mainly for correlating begin and end entries,
so we can remove pending operations from the list.

All journal entries sent to the service also have a `Version` field embedded in them.
This field allows for an evolution of the journal entry format in the future.

Note that journal entries can arrive out of order at the backend. However the
engine guarantees a partial order, as operations for dependents of a resource
will never start being processed before the dependency has finished its
operation. It's always safe to replay all the journal entries that have arrived
at the backend, in increasing order of their IDs, even if some IDs are missing.

Spelled out as a number of rules, this looks like:
- The client must not start an operation until its "begin" entry has been
  persisted on the backend.
- The client must not consider an operation complete until its "end" entry has
  been persisted on the backend.
- The client must not send journal entries for operations that depend on
  previous operations, until those previous operations are complete.
- The client may send journal entries for independent operations concurrently.

##### JournalEntryBegin

This journal entry type is emitted at the start of each step. It's optionally
associated with a `resource.Operation`, that should be recorded as a "pending
operation" in the snapshot, until we have a corresponding journal entry
finalizing the operation. The begin journal entries are also used to mark
resources as `Delete=true` if necessary, by setting the `DeleteOld` field to the
index of the resource that should be marked for deletion.

##### JournalEntrySuccess

Whenever we successfully run a resource step, we emit a success journal
entry. This Journal Entry contains the following information:
- `RemoveOld`: If not nil, the index in the resource list in the old snapshot for an entry
  that should be removed.
- `RemoveNew`: If not nil, the operation ID of a resource that's to be deleted, e.g. if
  a previous operation created a resource, but it's no longer needed/replaced.
- `PendingReplacement`: If not nil, the index of a resource in the old snapshot that
  should be marked as pending replacement.
- `Delete`: If not nil, the index of a resource in the old snapshot that should be
  marked as `Delete`.
- `State`: The newly created resource state of the journal entry, if any.
- `ElideWrite`: True if the write can be elided. This is only used in the local
  implementation. If true, we don't need to send a new snapshot. The journal
  entry still needs to be sent to the backend.
- `IsRefresh`: True if the journal entry is part of a refresh operation. If there
  are any refresh operations, we need to rebuild the base state. Refreshes can
  delete resources, without updating their dependants. So we need to rebuild the
  base state, removing no longer existing resources from dependency lists. This
  rebuild happens at the end of building the snapshot.

##### JournalEntryFailure

This is emitted if a step fails. In this case no resource has been changed by the
step, so we just remove any pending operation that we created when we emitted the
"begin" entry with the same operation ID.

##### JournalEntryRefreshSuccess

This is a special journal entry for refreshes that the engine does not mark as
`Persisted()`. For refreshes, the engine would traditionally just update the base
snapshot, without involving the snapshot code. This however does not work with
the journaling code, as we operate on a copy of the snapshot. The difference
between a refresh success and a success journal entry, is that for refresh
successes, we need to replace the resource in the snapshot at the same index as
it was before. E.g. if `DeleteOld` is 1, we need to put the new state at index 1,
while if the same thing was true for a regular success entry, we would delete the
old resource, and add the new one to the end of the resource list.

##### JournalEntryOutputs

This journal event is emitted when the outputs of a resource have
changed. Similar to the "refresh success" journal entry, we replace the old state
with the new one here.

##### JournalEntryWrite

This special journal entry is only allowed once, at the very beginning of the
whole sequence of events, to write a new snapshot, in case of provider migrations
as mentioned above. The only field set here is `NewSnapshot`, which contains the
new snapshot, which will be deep copied to make sure we don't change any
pointers.

##### JournalEntryRebuiltBaseState

This is another special journal entry, that is only allowed when we either have
no new resources yet, or if it is emitted as the last journal entry. It's an
indicator that the engine has internally called `rebuildBaseState`, and some
resources might be gone from the base snapshot. At that point we also rebuild the
deployment within the journaler, so resources that have been deleted by refreshes
are removed from the state, and we're indexing it the right way.

#### Pseudocode

Following is the pseudocode for constructing the snapshot from journal entries:

```

# Apply snapshot writes
snapshot = find_write_journal_entry_or_use_base(base, journal)

# Track changes
deletes, snapshot_deletes, mark_deleted, mark_pending = set(), set(), set(), set()
operation_id_to_resource_index = {}

# Process operations
incomplete_ops = {}
has_refresh = false

index = 0
for entry in journal:
    match entry.type:
        case BEGIN:
            incomplete_ops[entry.op_id] = entry

        case SUCCESS:
            del incomplete_ops[entry.op_id]

            if entry.state and entry.op_id:
                resources.append(entry.state)
				operation_id_to_resource_index.add(entry.op_id, index)
				index++
            if entry.remove_old:
                snapshot_deletes.add(entry.remove_old)
			if entry.remove_new:
				deletes[remove_new] = true
			if entry.pending_replacement:
				mark_pending(entry.pending_replacement)
			if entry.delete:
			    mark_deleted(entry.delete)
            has_refresh |= entry.is_refresh

        case REFRESH_SUCCESS:
            del incomplete_ops[entry.op_id]
            has_refresh = true
            if entry.remove_old:
                if entry.state:
                    snapshot_replacements[entry.remove_old] = entry.state
                else:
                    snapshot_deletes.add(entry.remove_old)
	        if entry.remove_new:
			    if entry.state:
				    deletes[entry.remove_new] = true
				else:
				    resources.replace(operation_id_to_resource_index(entry.remove_new), entry.state)
        case [REFRESH_SUCCESS, OUTPUTS]:
		    resources.replace(operation_id_to_resource_index(entry.remove_new), entry.state)
        case FAILURE:
            del incomplete_ops[entry.op_id]

        case OUTPUTS:
            if entry.state and entry.remove_old:
                snapshot_replacements[entry.remove_old] = entry.state
			if entry.state and entry.remove_new:
			    resources.replace(operation_id_to_resource_index(entry.remove_new), entry.state)

deletes = deletes.map(|i| => operation_id_to_resource_index[i])

# Remove new resources that should be removed
for i, res in resources:
    if i in deletes:
	    remove_from_resources(resources, i)

# Merge snapshot resources
for i, res in enumerate(snapshot.resources):
    if i not in snapshot_deletes:
        if i in snapshot_replacements:
            resources.append(snapshot_replacements[i])
        else:
            if i in mark_deleted:
                res.delete = true
			if i in mark_pending:
			    res.pending_replacement = true
            resources.append(res)

# Collect pending operations
pending_ops = [op.operation for op in incomplete_ops.values() if op.operation]
pending_ops.extend([op for op in snapshot.pending_ops if op.type == CREATE])

# Rebuild and return
if has_refresh:
    rebuild_dependencies(resources)
```

As mentioned above, Journal Entries need to be stored before the
BeginOperation/EndOperation calls finish, so the engine only starts the next step
after the journal entry for the dependencies is saved before we work on any of the
dependents. That's the main ordering requirement.

On the service side this is easiest to reproduce by replaying all journal entries
we have (the CLI will wait to start with dependencies until we got a reply from
the server), in order of their operation IDs.

### REST API

The service will get a new api `journalentries`, that will get the serialized
journal entries and persist them for later replay. The request can contain a list
of one or more journal entries. Once the service responds with a success response,
the entry is expected to be persisted and the engine allowed to continue. The
service is expected to reconstruct the snapshot itself after the operation finished,
and the API for getting the snapshot stays the same as it is currently.

(snapshot-integrity)=
## Snapshot integrity

*Integrity* is a property of a snapshot that ensures that the snapshot is
consistent and can be safely operated upon. The
[`Snapshot.VerifyIntegrity`](gh-file:pulumi#pkg/resource/deploy/snapshot.go)
method is responsible for performing these checks. When a snapshot has an
integrity error, the Pulumi CLI will refuse to operate on it.[^sie-p1] Note that the
Pulumi CLI will *not refuse to write a snapshot with integrity errors*, since
snapshots are often the only way of recording what actions the engine has
already taken (and e.g. which of those succeeded and which failed), and that
record is vital should the user need to recover from a failure.

If you find yourself debugging a snapshot integrity issue, or if you are keen to
avoid introducing one when writing new code, the following guidelines and
general principles may be useful:

* *Reproduce or simulate potential issues with one or more [lifecycle
  tests](lifecycle-tests).* Snapshot integrity issues are the result of the
  deployment engine mismanaging state. While bugs may manifest due to unexpected
  behaviour in resource providers or language hosts, for example, it is the
  engine's job to handle these cases correctly and preserve the integrity of its
  resource state. Lifecycle tests allow mocking providers and specifying
  programs directly without an intermediate language host, and provide the best
  means to consistently reproduce an issue or specify a desired behaviour.
  The lifecycle test suite's [fuzzing](lifecycle-fuzzing) capabilities may help
  when tracking down hard-to-find issues.

* *Avoid realising [deletions](step-generation-deletions) until the end of an
  operation.* Many snapshot integrity issues arise from resources ending up in
  state with missing dependencies, or with dependencies that appear later than
  they do in the snapshot (snapshots are expected to be [topologically
  sorted](https://en.wikipedia.org/wiki/Topological_sorting)). Deleting a
  resource from the state mid-deployment is almost guaranteed to result in these
  issues at some point. This is especially likely if a later operation fails and
  causes the deployment to terminate early, leaving later resources that you may
  have intended to update following the deletion in a broken state. Instead of
  outright removing a resource from the state, consider marking it as pending or
  needing deletion later on (this is how
  [`deleteBeforeReplace`](step-generation-dependent-replacements) works, for
  example). That way, you can remove the resource at the end of the operation
  when you know that all of its dependencies have been processed (in the case of
  `deleteBeforeReplace`, it is the final `CreateReplacementStep` that actually
  removes the old resource from the state, for instance).

* *Consider all forms of dependencies.* [Providers](providers), parents,
  dependencies, property dependencies, and deleted-with relationships are all
  forms of resource dependency that must be respected by any code being written
  or examined. If a resource is moved, renamed or deleted, and its dependencies
  are not updated, for instance, an integrity error is likely to occur.

* *Think about how code behaves when only specific resources are targeted.*
  Targeted operations can violate many assumptions that are otherwise safe to
  make, such as having processed a resource's dependencies before the resource
  itself is visited. When debugging, ascertaining whether a snapshot integrity
  issue has been triggered by a targeted operation is often an excellent first
  step, since it can massively narrow down the code paths that need to be
  examined.

* *Many operations are non-atomic and nearly all of them can fail.* Don't assume
  that processing a resource will always proceed smoothly. If the snapshot is to
  be modified before or after making a provider call, consider that the provider
  call could fail. Does the code account for this and work correctly even if it
  is resumed following a failure?

* *The program may change between operations.* If you are debugging or
  attempting to reproduce an issue, consider that it may take multiple
  operations to trigger the issue and that the program being run may change
  between these operations. For instance, a resource may be removed from the
  program -- in these cases, there will be an operation where the resource is in
  the state but the engine does not receive a registration (this may behave even
  more interestingly if that resource is or is not targeted in a targeted
  operation -- see [](gh-issue:pulumi#17117) for an example of these kinds of
  interactions).

The following are examples of fixes for snapshot integrity issues that may serve
as examples of applying the above principles and tracking down issues:

* [Fix snapshot integrity on pending replacement](gh-issue:pulumi#17146)
* [Propagate deleted parents of untargeted resources](gh-issue:pulumi#17117)
* [Better handle property dependencies and `deletedWith`](gh-issue:pulumi#16088)
* [Rewrite `DeletedWith` properties when renaming stacks](gh-issue:pulumi#16216)

(secrets-encryption)=
## Secrets & encryption

Pulumi has first-class support for secrets within state which are encrypted and
decrypted via pluggable secret backends. Secrets are identified with the
signature property ("4dabf18193072939515e22adb298388d") set to the value
`"1b47061264138c4ac30d75fd1eb44270"`. A secret will then have either a
`ciphertext` or `plaintext` property alongside the signature. These are either
an encrypted or unencrypted JSON serialized string, respectively, of the property
within the secret. Any nested secrets will only have a `plaintext` field as the
outer secret's encryption already covers it.

The process of encrypting and decrypting secrets is managed through crypters
(`config.Encrypter` & `config.Decrypter`) (interface defined in
`sdk/go/common/resource/config/crypt.go`). The most important crypters to be
aware of are:

* `serviceCrypter` in `pkg/secrets/service/manager.go` - performs all encryption
   operations within Pulumi's cloud service.
* `symmetricCrypter` - used for all passphrase & cloud backends (including AWS,
   Azure, GCP & HashiVault). Created with key bytes to perform encryption tasks
   locally.
* `blindingCrypter` - returns `[secret]` for all encryption operations e.g. for
   use in default CLI display
* `nopCrypter` - returns the ciphertext as the plaintext - useful for operations
   where we might not have access to the encryption key but can work without
   modifying or fully deserializing the secrets.

Crypters for use on stack state are typically created from a `secret.Manager`
except in special circumstances where a blinding or noop crypter might suffice.
There are three production secret managers:

1. `serviceSecretsManager` for providing `serviceCrypter` instances for
   interacting with Pulumi's cloud service.
2. "Cloud Secrets Manager" which uses `gocloud.dev/secrets` to construct
   `symmetricCrypter` instances from popular cloud platforms.
3. `localSecretsManager` (in the `passphrase` module) to construct
   `symmetricCrypter` instances from a simple configured passphrase.

Secret managers are constructed from a `secrets.Provider` using the `OfType`
method (as defined in `pkg/secrets/provider.go`). This is almost always the
`DefaultSecretsProvider` except for when moving state, when we use the
`NamedStackSecretsProvider`. The `OfType` method is only called at the point we
first deserialize the stack state, then the returned secrets manager is included
within the `deploy.Snapshot` object. Both implementations of the
`secrets.Provider` type wrap the underlying secrets manager implementation in a
`BatchingSecretsManager`. The `batchingCachingSecretsManager` implementation
also maintains a cache of encrypted secrets to prevent work duplication across
a single operation.

[^sie-p1]:
    Snapshot integrity issues are generally "P1" issues, meaning that they are
    picked up as soon as possible in the development process.
