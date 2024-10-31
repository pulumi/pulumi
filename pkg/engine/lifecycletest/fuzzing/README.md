(lifecycle-fuzzing)=
## Fuzzing

[Snapshot integrity errors](snapshot-integrity) are very problematic when they
occur and can be hard to spot and prevent. To this end, a subset of the
lifecycle test suite uses a combination of
[fuzzing](https://en.wikipedia.org/wiki/Fuzzing) and [property-based
testing](https://en.wikipedia.org/wiki/Property_testing) via the
[Rapid](https://pkg.go.dev/pgregory.net/rapid) Go library to randomly generate
snapshots and programs to see whether or not it is possible to trigger a
snapshot integrity error.

While snapshot integrity issues often happen as part of a chain of snapshot
operations (e.g. the execution of multiple steps in a deployment), the precursor
to any error state will always be a valid snapshot. Thus, rather than having to
generate random chains of operations, we can instead simplify the problem to
generating valid starting snapshots and then executing a single random operation
on them. The strategy we employ is thus as follows:

* Generate a [snapshot](state-snapshots)
  ([snapshot.go](gh-file:pulumi#pkg/engine/lifecycletest/fuzzing/snapshot.go))
  consisting of a random set of resources
  ([resource.go](gh-file:pulumi#pkg/engine/lifecycletest/fuzzing/resource.go)),
  including appropriate [providers](providers).
  Resources may randomly depend on each other, and may have random properties,
  such as whether they are [custom resources or components](custom-resources),
  [pending replacement](step-generation-dependent-replacements), and so on.

* Generate a program
  ([program.go](gh-file:pulumi#pkg/engine/lifecycletest/fuzzing/program.go))
  from the previously generated snapshot. The program may choose to
  [register](resource-registration) any subset (including none) of the
  resources in the snapshot, as well as any set of new resources before, in
  between and after those specified in the snapshot. Resources from the snapshot
  that are registered may be copied as-is or registered with different
  properties.

* Generate a set of provider implementations for the program
  ([provider.go](gh-file:pulumi#pkg/engine/lifecycletest/fuzzing/provider.go)).
  Provider operations such as [](pulumirpc.ResourceProvider.Create),
  [](pulumirpc.ResourceProvider.Diff), etc. may be configured to fail randomly,
  or return one of a set of random results (e.g. an update vs a replace for
  `Diff`), on a per-resource basis.

* Generate an operation (one of `preview`, `up`, `refresh` and `destroy`) and
  associated configuration (such as a list of `--target`s), known in the test
  suite as a *plan* to execute
  ([plan.go](gh-file:pulumi#pkg/engine/lifecycletest/fuzzing/plan.go)).

* Combine the snapshot, program, providers and plan to form a *fixture*
  ([fixture.go](gh-file:pulumi#pkg/engine/lifecycletest/fuzzing/fixture.go)) and
  execute it. If the operation yields a valid snapshot, the test passes, whether
  the operation completes successfully or not. If an invalid snapshot is
  produced, the test fails and the reproducing combination of snapshot, program,
  providers and plan is returned for debugging.
