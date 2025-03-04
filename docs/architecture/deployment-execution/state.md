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
`1b47061264138c4ac30d75fd1eb44270`. A secret will then have either a
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
`CachingSecretsManager`.

[^sie-p1]:
    Snapshot integrity issues are generally "P1" issues, meaning that they are
    picked up as soon as possible in the development process.
