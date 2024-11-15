(resource-registration)=
# Resource registration

As a Pulumi program is executed by the language host, it will make
[](pulumirpc.ResourceMonitor.RegisterResource) calls to the engine in order to
declare resources and their desired state. Each
[](pulumirpc.RegisterResourceRequest) contains:

* The resource's type and name.
* The resource's [parent](https://www.pulumi.com/docs/concepts/options/parent/),
  if it has one.
* A reference to the
  [provider](https://www.pulumi.com/docs/concepts/resources/providers/) that
  manages the resource, if an explicit one has been specified. If no reference
  has been specified, Pulumi will use a [default provider](#default-providers)
  instance for the resource's package and version.
* Values for the resource's input properties.
* Any [resource options](https://www.pulumi.com/docs/concepts/options/) that
  have been specified for the resource.

In order to determine what actions to take in order to arrive at the desired,
the engine *diffs* the desired state of the resource against the current state
as recorded in the [state snapshot](state-snapshots):

* If there is no current state, the engine will attempt to create a new
  resource.
* If there is a current state, the engine will [diff](step-generation-diff) the
  current state with the desired state to determine whether the resource is
  unchanged, requires updating, or must be replaced in some manner.

When the appropriate actions have been determined, the engine will invoke the
relevant provider methods to carry them out. After the actions complete, the
engine returns the new state of the resource to the program.

Although all of the above happens "in the engine", in practice these concerns
are separated into a number of subsystems: the [resource
monitor](resource-monitor), the [step generator](step-generation), and the [step
executor](step-execution).

(resource-monitor)=
## Resource monitor

The *resource monitor* (largely represented by `resmon` in the codebase; see
<gh-file:pulumi#pkg/resource/deploy/source_eval.go>) implements the
[](pulumirpc.ResourceMonitor) interface, which is the primary communication
channel between language hosts and the engine. There is a single resource
monitor per deployment. Aside from being a marshalling and unmarshalling layer
between the engine and its gRPC boundary, the resource monitor is also
responsible for resolving [default providers](default-providers) and [component
providers](component-providers), responding to
[](pulumirpc.RegisterResourceRequest)s as follows:

* The request is unmarshalled from the gRPC wire format into an engine-internal
  representation.
* If the request lacks a `provider` reference, the resource monitor will resolve
  a [default provider](default-providers) for the resource's package and
  version.
* If the request registers a [remote component](component-providers), the
  resource monitor will dispatch an appropriate
  [](pulumirpc.ResourceProvider.Construct) call to the component provider and
  await the result.
* If the request does *not* register a remote component (a so-called *custom
  resource*, although this is in reality the default type of resource), the
  resource monitor will emit a `RegisterResourceEvent` and await a response.
* When a result is received (either in response to a
  [](pulumirpc.ResourceProvider.Construct) call or a `RegisterResourceEvent`),
  the resource monitor will marshal the result back into the gRPC wire format
  and return it to the language host.

(step-generation)=
## Step generation

The *step generator* (<gh-file:pulumi#pkg/resource/deploy/step_generator.go>) is
responsible for processing `RegisterResourceEvent`s emitted by the resource
monitor. When an event is received, the step generator proceeds as follows:

1. A URN is generated for the resource, using the type, name and parent fields
   from the event.
2. The URN is used to look up the resource's existing state, if it has any. If
   the event contains aliases, state under those aliases will also be looked up.
   It is an error if multiple pieces of existing state are found due to
   aliasing.
3. Input properties are pre-processed to implement the `ignoreChanges` resource
   option, by resetting the values of any properties which should be ignored to
   their previous values.
4. If the event indicates that the resource should be imported, the step
   generator will emit an `ImportStep` and return.
5. If the resource is not being imported, the step generator will continue by
   calling the provider's [](pulumirpc.ResourceProvider.Check) method with both
   the event's input properties and the resource's existing
   inputs.[^existing-inputs] `Check` will return a validated bag of input values
   that may be used in later calls to [](pulumirpc.ResourceProvider.Diff),
   [](pulumirpc.ResourceProvider.Create), and
   [](pulumirpc.ResourceProvider.Update).
6. At this point, the step generator will invoke any *analyzers* that have been
   configured in the stack to perform additional validation on the resource's
   input properties.
7. If the resource has no existing state, it must be created. Issue a
   `CreateStep` and return.
8. [Diff](step-generation-diff) the resource in order to determine whether it
   must be updated, replaced, or left as-is.
9. If there are no changes, issue a `SameStep` and return.
10. If the resource is not being replaced, issue an `UpdateStep` and return.
11. If the resource is being replaced, call the resource provider's
    [](pulumirpc.ResourceProvider.Check) method again, this time sending no
    existing inputs. This call ensures that the input properties used to create
    the replacement will not reuse generated defaults that should be unique to
    the existing resource.
12. If the replacement should be created before the original is deleted (a
    normal replacement, also "create-replace" or "create-before-replace"), issue
    an appropriate `CreateStep` and `DeleteStep` pair and return.
13. If the replacement should be created after the original has been deleted
    ("delete-replace", "delete-before-replace", "DBR"), calculate the list of
    resources that will have to be deleted in response to the deletion of the
    original (see the sections on [deletions](step-generation-deletions) and
    [dependent replacements](step-generation-dependent-replacements) later on
    for more details). Then, issue a `DeleteStep` and `CreateStep` pair for the
    replacement and return.

[^existing-inputs]:
    Existing inputs inputs may be used to repopulate default values for input
    properties that are automatically generated when the resource is created but
    that are not changed on subsequent updates (e.g. automatically generated
    names).

:::{note}
Presently, step generation is a *serial* process (that is, steps are processed
one at a time, in turn). This means that step generation is on the critical path
for a deployment, so any significant blocking operations could slow down
deployments considerably. In the case of an update, step generator latency is
generally insignificant compared to the time spend performing provider
operations (e.g. cloud updates), but in the case of a large preview operation,
or an update where most resources are unchanged, the step generator could become
a bottleneck.
:::

Step generation is a fire-and-forget process. Once a step has been generated,
the step generator immediately moves on to the next `RegisterResourceEvent`. It
is the responsibility of the [step executor](step-execution) to communicate the
results of each step back to the [resource monitor](resource-monitor).

(step-generation-diff)=
### Diffing

While in most cases diffing boils down to calling a provider's
[](pulumirpc.ResourceProvider.Diff) method, there are a number of cases where
this might not happen. The full algorithm that the engine currently implements
is as follows:

1. If the resource has been marked for replacement out-of-band (e.g. by the use
   of the `--target-replace` command-line option), the resource must be
   replaced.
2. If the resource's provider has changed, the resource must be replaced.
   [Default providers](default-providers) are allowed to change without
   triggering a replacement if and only if the provider's configuration allows
   the new default provider to continue to manage existing resources. This is
   intended to allow default providers to be upgraded without causing all
   resources they manage to be replaced.
3. If the engine is configured to use legacy (pre-1.0) diffs, the engine will
   compare old and new inputs itself (without consulting the provider). If these
   differ, the resource must be updated.
4. In all other cases, the engine will call the provider's
   [](pulumirpc.ResourceProvider.Diff) method to determine whether the resource
   must be updated, replaced, or left as-is.

(step-generation-deletions)=
### Deletions

Once the Pulumi program has exited, the step generator determines the set of
resources that must be deleted by computing the difference between the set of
registered resources and the set of resources that were present in the previous
state snapshot. This set is sorted topologically by reverse dependencies (that
is, resources that depend on other resources are deleted first). This sorted
list is then decomposed into a list of lists where the sublists must be
processed serially but the contents of each sublist can be processed in
parallel.

(step-generation-dependent-replacements)=
### Dependent replacements

By default, Pulumi will replace resources by first creating the replacement and
then deleting the original (create-before-replace). There are cases however
where Pulumi will delete the original first (a delete-before-replace):

* If the resource's provider specifies that the resource must be deleted before
  it can be replaced as using a [](pulumirpc.DiffResponse)'s
  `deleteBeforeReplace` field.
* If the program specifies the [`deleteBeforeReplace` resource
  option](https://www.pulumi.com/docs/concepts/options/deletebeforereplace/) for
  the resource.

In such cases, it may be necessary to first delete resources that depend on that
being replaced, since there will be a moment between the delete and create steps
where no version of the resource exists (and thus dependent resources will have
broken dependencies). The step generator does this as follows:

1. Compute the full set of resources that transitively depend on the resource
   being replaced.
2. Remove from this set any resources that would *not themselves be replaced* by
   changes to their dependencies. This is determined by substituting
   [unknown](output-unknowns) values for any inputs that stem from a dependency
   and calling the provider's [](pulumirpc.ResourceProvider.Diff) method.
3. Process the replacements in reverse topological order.

To better illustrate this, consider the following example (written in
pseudo-TypeScript):

```typescript
const a = new Resource("a", {})
const b = new Resource("b", {}, { dependsOn: a })
const c = new Resource("c", { input: a.output })
const d = new Resource("d", { input: b.output })
```

The dependency graph for this program is as follows:

```mermaid
flowchart TD
    b --> a
    c --> a
    d --> b
```

We see that the transitive set of resources that depend on `a` is `{b, c, d}`.
In the event that `a` is subject to a delete-before-replace, then each of `b`,
`c`, and `d` must also be considered. Since `b`'s relationship is only due to
[`dependsOn`](https://www.pulumi.com/docs/concepts/options/dependson/), its
inputs will not be affected by the deletion of `a`, so it does not need to be
replaced. `c`'s inputs are affected by the deletion of `a`, so we must call
`Diff` to see whether it needs to be replaced or not. `d`'s dependency on `a` is
through `b`, which we have established does not need to be replaced, so `d` does
not need to be replaced either.

(step-execution)=
## Step execution

The *step executor* is responsible for executing steps yielded by the [step
generator](step-generation). Steps are processed in sequences called *chains*.
While the steps within a chain must be processed serially, chains may be
processed in parallel. The step executor uses a pool of workers to execute
steps. Once a step completes, the executor communicates its results to the
[resource monitor](resource-monitor). If a step fails, the executor notes its
failure and cancels the deployment. Once the Pulumi program has exited and the
step generator has issued all required deletions, the step executor waits for
all outstanding steps to complete and then returns.

(resource-registration-examples)=
## Examples

The following subsections give some example sequence diagrams for the processes
described in this document. Use your mouse to zoom in/out and move around as
necessary.

### Creating a resource

```mermaid
:zoom:

sequenceDiagram
    participant LH as Language host
    box Engine
        participant RM as Resource monitor
        participant SG as Step generator
        participant SE as Step executor
    end
    participant P as Provider

    LH->>+RM: RegisterResourceRequest(type, name, inputs, options)
    RM->>+SG: RegisterResourceEvent(type, name, inputs, options)
    SG->>+P: CheckRequest(type, inputs)
    P->>-SG: CheckResponse(inputs', failures)
    SG->>+SE: CreateStep(inputs', options)
    SE->>+P: CreateRequest(type, inputs')
    P->>-SE: CreateResponse(new state)
    SE->>-RM: Done(new state)
    RM->>-LH: RegisterResourceResponse(URN, ID, new state)
```

### Updating a resource

```mermaid
:zoom:

sequenceDiagram
    participant LH as Language host
    box Engine
        participant RM as Resource monitor
        participant SG as Step generator
        participant SE as Step executor
    end
    participant P as Provider

    LH->>+RM: RegisterResourceRequest(type, name, inputs, options)
    RM->>+SG: RegisterResourceEvent(type, name, inputs, options)
    SG->>+P: CheckRequest(type, inputs, old inputs)
    P->>-SG: CheckResponse(inputs', failures)
    SG->>+P: DiffRequest(type, inputs', old state, options)
    P->>-SG: DiffResponse(diff)
    SG->>+SE: UpdateStep(inputs', old state, options)
    SE->>+P: UpdateRequest(type, inputs', old state)
    P->>-SE: UpdateResponse(new state)
    SE->>-RM: Done(new state)
    RM->>-LH: RegisterResourceResponse(URN, ID, new state)
```

### Replacing a resource (create-before-replace)

```mermaid
:zoom:

sequenceDiagram
    participant LH as Language host
    box Engine
        participant RM as Resource monitor
        participant SG as Step generator
        participant SE as Step executor
    end
    participant P as Provider

    LH->>+RM: RegisterResourceRequest(type, name, inputs, options)
    RM->>+SG: RegisterResourceEvent(type, name, inputs, options)
    SG->>+P: CheckRequest(type, inputs, old inputs)
    P->>-SG: CheckResponse(inputs', failures)
    SG->>+P: DiffRequest(type, inputs', old state, options)
    P->>-SG: DiffResponse(diff)
    SG->>+SE: CreateStep(inputs', old state, options)
    SE->>+P: CreateRequest(type, inputs', old state)
    P->>-SE: CreateResponse(new state)
    SE->>-RM: Done(new state)
    RM->>-LH: RegisterResourceResponse(URN, ID, new state)

    Note over SG: Pulumi program exits

    SG->>SG: Generate delete steps
    SG->>+SE: DeleteStep(old state)
    SE->>+P: DeleteRequest(type, old state)
    P->>-SE: DeleteResponse()
```

### Replacing a resource (delete-before-replace)

```mermaid
:zoom:

sequenceDiagram
    participant LH as Language host
    box Engine
        participant RM as Resource monitor
        participant SG as Step generator
        participant SE as Step executor
    end
    participant P as Provider

    LH->>+RM: RegisterResourceRequest(type, name, inputs, options)
    RM->>+SG: RegisterResourceEvent(type, name, inputs, options)
    SG->>+P: CheckRequest(type, inputs, old inputs)
    P->>-SG: CheckResponse(inputs', failures)
    SG->>+P: DiffRequest(type, inputs', old state, options)
    P->>-SG: DiffResponse(diff)
    SG->>+SE: DeleteStep(old state), CreateStep(inputs', options)
    SE->>+P: DeleteRequest(type, old state)
    P->>-SE: DeleteResponse()
    SE->>+P: CreateRequest(type, inputs', old state)
    P->>-SE: CreateResponse(new state)
    SE->>-RM: Done(new state)
    RM->>-LH: RegisterResourceResponse(URN, ID, new state)
```

### Importing a resource

```mermaid
:zoom:

sequenceDiagram
    participant LH as Language host
    box Engine
        participant RM as Resource monitor
        participant SG as Step generator
        participant SE as Step executor
    end
    participant P as Provider

    LH->>+RM: RegisterResourceRequest(type, name, inputs, options)
    RM->>+SG: RegisterResourceEvent(type, name, inputs, options)
    SG->>+SE: ImportStep(inputs, options)
    SE->>+P: ReadRequest(type, id)
    P->>-SE: ReadResponse(current inputs, current state)
    SE->>+P: CheckRequest(type, inputs, current inputs)
    P->>-SE: CheckResponse(inputs', failures)
    SE->>+P: DiffRequest(type, inputs', current state, options)
    P->>-SE: DiffResponse(diff)
    SE->>-RM: Done(current state)
    RM->>-LH: RegisterResourceResponse(URN, ID, current state)
```

### Leaving a resource unchanged

```mermaid
:zoom:

sequenceDiagram
    participant LH as Language host
    box Engine
        participant RM as Resource monitor
        participant SG as Step generator
        participant SE as Step executor
    end
    participant P as Provider

    LH->>+RM: RegisterResourceRequest(type, name, inputs, options)
    RM->>+SG: RegisterResourceEvent(type, name, inputs, options)
    SG->>+P: CheckRequest(type, inputs, old inputs)
    P->>-SG: CheckResponse(inputs', failures)
    SG->>+P: DiffRequest(type, inputs', old state, options)
    P->>-SG: DiffResponse(diff)
    SG->>+SE: SameStep(inputs', old state, options)
    SE->>-RM: Done(old state)
    RM->>-LH: RegisterResourceResponse(URN, ID, old state)
```
