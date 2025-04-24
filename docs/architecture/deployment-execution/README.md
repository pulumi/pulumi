(deployments)=
# Deployment execution

The Pulumi engine is responsible for orchestrating *deployments*. A deployment
typically comprises a single *operation*, such as an update or destroy, though
it may conceptually execute more than one (if `--refresh` is passed to an
`up`, for example). Internally, a deployment is an event-driven process whose
lifecycle looks roughly as follows:

* The engine starts and collects necessary configuration values to construct a
  deployment (the stack to operate on, whether it should continue when it
  encounters errors, and so on).
* As part of this process, the engine decides upon a *source* that will provide
  events to drive the deployment. Events represent things like resources that
  need to be managed ("I want to declare an S3 bucket"), or functions that need
  to be called ("I want to look up the EC2 instance with this ID").
* Once the deployment has been instantiated and configured with a source, the
  engine *iterates* over the source and the events it provides in order to drive
  the deployment.
* [Resource events](resource-registration) result in [steps](step-generation), which
  correspond to actions that must be taken in order to arrive at the desired
  state for a given resource -- e.g. by calling a [provider](providers)
  [](pulumirpc.ResourceProvider.Create) method in order to create a new
  resource.
* When all steps have been executed, the (hopefully desired)
  [state](state-snapshots) of the infrastructure is persisted to the configured
  backend and the deployment is completed.

The choice of source depends on the operation.

## Operations that do not always execute the program

`refresh` and `destroy` operations historically did not result in program execution.
Whether or not this is desirable/useful arguably depends on the context, but as
far as the implementation is concerned at present, these operations are driven
as follows:

* `destroy` uses a *null source*
  (<gh-file:pulumi#pkg/resource/deploy/source_null.go>), which simply returns no
  events. This effectively simulates an empty program, which is exactly what
  we'd write if we wanted to destroy all our resources.
* `refresh` uses an *error source*
  (<gh-file:pulumi#pkg/resource/deploy/source_error.go>), which returns an error
  event if it is iterated. This is to capture the invariant that a `refresh`
  operation should not consult its source, since it is only concerned with
  changes in the various providers, and not the program.
  As such it doesn't call into the step generator, instead it just creates a RefreshStep for
  each resource in state, sends them all in parallel to the step executor and then re-writes the
  state snapshot to handle any deletions.

Both these operations now take a `--run-program` option, which currently defaults to false. When set to true
these operations will run the step generator in either `destroy` or `refresh` mode. This changes how the step
generator generates steps for custom resources. Note that providers and component resources behave similarly
the same in `destroy`, `refresh`, and `up`.

In `refresh` mode the step generator generates a `RefreshStep` or `SkippedCreateStep` for each custom
resource. `SkippedCreateStep` is generated for custom resources that aren't currently in state, as we're doing
a refresh we can't actually create them so we treat them similarly to when running with `--target`. Component
resources and providers are created and updated as if `up` had been run. Note that while we could have
`refresh` also delete removed providers and components, currently it just leaves them in state as is. The
means things like stack outputs get fully updated by the operation.

In `destroy` mode the step generator initially generates a `SameStep` or `SkippedCreateStep` for each custom
resource. This is so the resource registration is returned to the program with the current state of the
resource. It wouldn't be useful for the program to get the post-delete state of nothing. Component resources
and providers are created and updated as if `up` had been run. However at the end of the program the
step generator generates `DeleteStep`s for everything even though we've already done other steps for them.
This differs from a normal run of the step generator where it would only generate `DeleteStep`s for resources
that had not yet had any other step generated for them.

## Operations that execute the program

For `preview` and `update` operations, the Pulumi program specifies the desired
state of a user's infrastructure in a manner idiomatic to the language in which
the program is written, e.g. `new Resource(...)` in TypeScript, or
`Resource(...)` in Python. When performing these operations, the engine
instantiates an *evaluation source*
(<gh-file:pulumi#pkg/resource/deploy/source_eval.go>) that boots up a language
host to run the program. The classes, function calls, and so on exposed by the
particular language SDK are implemented using gRPC calls under the hood that
enable communication between the language host and the engine. Specifically:

* [](pulumirpc.ResourceMonitor) is the primary interface used to manage
  resources. A resource monitor supports [registering
  resources](pulumirpc.ResourceMonitor.RegisterResource), [reading
  resources](pulumirpc.ResourceMonitor.ReadResource), [invoking
  functions](pulumirpc.ResourceMonitor.Invoke), and many more operations key to
  the operation of a Pulumi program. An evaluation source contains a
  `ResourceMonitor` instance that enqueues events to be returned when it is
  iterated over, so the deployment ends up being driven by the program
  evaluation.
* [](codegen.Loader) provides methods for loading schemata.
* [](pulumirpc.Engine) exposes auxilliary operations that are not specific to a
  particular resource, such as [logging](pulumirpc.Engine.Log) and [state
  management](pulumirpc.Engine.SetRootResource).
* [](pulumirpc.Callbacks) allows callers to [execute
  functions](pulumirpc.Callbacks.Invoke) in the language host, such as
  [transforms](https://www.pulumi.com/docs/concepts/options/transforms/), which
  are specified as part of the program but whose execution is driven by the
  engine.

The engine hosts services implementing `ResourceMonitor`, `Loader`, and
`Engine`, while language hosts implement `Callbacks`. All of these come together
in a number of processes to execute a Pulumi program, as elucidated in the
following pages.

:::{toctree}
:maxdepth: 1
:titlesonly:

/docs/architecture/deployment-execution/resource-registration
/docs/architecture/deployment-execution/state
/docs/architecture/deployment-execution/import
:::
