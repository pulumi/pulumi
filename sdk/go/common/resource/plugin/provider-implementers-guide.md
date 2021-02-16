# Resource Provider Implementer's Guide

## Provider Programming Model

### Resources

The core functionality of a resource provider is the management of custom resources and
construction of component resources within the scope of a Pulumi stack. Custom resources
have a well-defined lifecycle built around the differences between their acutal state and
the desired state described by their inputs and implemented using create, read, update,
and delete (CRUD) operations defined by the provider. Component resources have no
associated lifecycle, and are constructed by registering child custom or component
resources with the Pulumi engine.

Each resource registered with the Pulumi engine is logically identified by its 
uniform resource name (URN). A resource's URN is derived from the its type, parent type,
and user-supplied name. Within the scope of a resource-related provider method
([`Check`](#check), [`Diff`](#diff), [`Create`](#create), [`Read`](#read),
[`Update`](#update), [`Delete`](#delete), and [`Construct`](#construct)), the type of
the resource can be extracted from the provided URN.

#### Custom Resources

In addition to its URN, each custom resource has an associated ID. This ID is opaque to
the Pulumi engine, and is only meaningful to the provider as a means to identify a
physical resource. The ID must be a string. The empty ID indicates that a resource's ID
is not known because it has not yet been created. Critically, a custom resource has a
[well-defined lifecycle](#custom-resource-lifecycle) within the scope of a Pulumi stack.

#### Component Resources

A component resource is a logical conatiner for other resources. Besides its URN, a
component resource has a set of inputs, a set of outputs, and a tree of children. Its
only lifecycle semantics are those of its children; its inputs and outputs are not
related in the same way a [custom resource's](#custom-resources) inputs and state are
related. The engine can call a resource provider's [`Construct`](#construct) method to
request that the provider create a component resource of a particular type.

### Functions

A provider function is a function implemented by a provider, and has access to any of the
provider's state. Each function has a unique token, optionally accepts an input object,
and optionally produces an output object. The data passed to and returned from a function
must not be [unknown](#unknowns) or [secret](#secrets), and must not
[refer to resources](#resource-references). Note that an exception to these rules is made
for component resource methods, which may accept values of any type, and are provided
with a connection to the Pulumi engine.

### Data Exchange Types

The values exchanged between Pulumi resource providers and the Pulumi engine are a 
superset of the values expressible in JSON.

Pulumi supports the following data types:
- `Null`, which represents the lack of a value
- `Bool`, which represents a boolean value
- `Number`, which represents an IEEE-754 double-precision number
- `String`, which represents a sequence of UTF-8 encoded unicode code points
- `Array`, which represents a numbered sequence of values
- `Object`, which represents an unordered map from strings to values
- [`Asset`](#assets-and-archives), which represents a blob
- [`Archive`](#assets-and-archives), which represents a map from strings to `Asset`s or
  `Archive`s
- [`ResourceReference`](#resource-references), which represents a reference to a [Pulumi 
  resource](#resources)
- [`Unknown`](#unknowns), which represents a value whose type and concrete value are not 
  known
- [`Secret`](#secrets), which demarcates a value whose contents are sensitive

#### Assets and Archives

An `Asset` or `Archive` may contain either literal data or a reference to a file or URL.
In the former case, the literal data is a textual string or a map from strings to `Asset`s
or `Archive`s, respectively. In the latter case, the referenced file or URL is an opaque
blob or a TAR, gzipped TAR, or ZIP archive, respectively.

Each `Asset` or `Archive` also carries the SHA-256 hash of its contents. This hash can be
used to uniquely identify the asset (e.g. for locally caching `Asset` or `Archive`
contents).

#### Resource References

A `ResourceReference` represents a reference to a [Pulumi resource](#Resources). Although
all that is necessary to uniquely identify a resource is its URN, a `ResourceReference`
also carries the resource's ID (if it is a [custom resource](#custom-resources)) and the
version of the provider that manages the resource. If the contents of the referenced
resource must be inspected, the reference must be resolved by invoking the `getResource`
function of the engine's builtin provider. Note that this is only possible if there is a 
connection to the engine's resource monitor, e.g. within the scope of a call to `Construct`.
This implies that resource references may not be resolved within calls to other 
provider methods. Therefore, configuration vales, custom resources and provider functions
should not rely on the ability to resolve resource references, and should instead treat
resource references  as either their ID (if present) or URN. If the ID is present and
empty, it should be treated as an [`Unknown`](#unknowns).

#### Unknowns

An `Unknown` represents a value whose type and concrete value are not known. Resources
typically produce these values during [previews](#preview) for properties with values
that cannot be determined until the resource is actually created or updated.
[Functions](#functions) must not accept or return unknown values.

#### Secrets

A `Secret` represents a value whose contents are sensitive. Values of this type are 
merely wrappers around the sensitive value. A provider should take care not to leak a
secret value. and should wrap any resource output values that are always sensitive in a
`Secret`. [Functions](#functions) must not accept or return secret values.

## Schema

TODO: document the Pulumi schema model.

- configuration
- types
- resources
- functions

## Provider Lifecycle

Clients of a provider (e.g. the Pulumi CLI) must obey the provider lifecycle. This
lifecycle guarantees that a provider is configured before any resource operations are
performed or provider functions are invoked. The lifecycle of a provider instance is
described in brief below.

1. The user [looks up](#lookup) the factory for a particular `(package, semver)` tuple
   and uses the factory to create a provider instance.
2. The user [configures](#configuration) the provider instance with a particular
   configuration object.
3. The user performs resource operations and/or calls provider functions with the
   provider instance.
4. The user [shuts down](#shutdown) the provider instance.

Within the scope of a Pulumi stack, each provider instance has a corresponding provider
resource. Provider resources are custom resources that are managed by the Pulumi engine,
and obey the usual [custom resource lifecycle](#custom-resource-lifecycle). The `Check`
and `Diff` methods for a provider resource are implemented using the
[`CheckConfig`](#checkconfig) and [`DiffConfig`](#diffconfig) methods of the resource's
provider instance. The latter is criticially important to the user experience: if
[`DiffConfig`](#diffconfig) indicates that the provider resource must be replaced, all of
the custom resources managed by the provider resource will _also_ be replaced. Thus,
`DiffConfig` should only indicate that replacement is required if the provider's
new configuration prevents it from managing resources associated with its old
configuration.

### Lookup

Before a provider can be used, it must be instantiated. Instatiating a provider requires
a `(package, semver)` tuple, which is used to find an appropriate provider factory. The
lookup process proceeds as follows:

- Let the best available factory `B` be empty
- For each available provider factory `F` with package name `package`:
	- If the `F`'s version is compatible with `semver`:
		- If `B` is empty or if `F`'s version is newer than `B`'s version, set `B` to `F`
- If `B` is empty, no compatible factory is available, and lookup fails

Within the context of the Pulumi CLI, the list of available factories is the list of
installed resource plugins plus the builtin `pulumi` provider. The list of installed
resource plugins can be viewed by running `pulumi plugin ls`.

Once an appropriate factory has been found, it is used to construct a provider instance.

### Configuration

A provider may accept a set of configuration variables. After a provider is instantiated,
the instance must be configured before it may be used, even if its set of configuration
variables is empty. Configuration variables may be of [any type](#data-exchange-types).
Because it has no connection to the Pulumi engine during configuration, a provider's
configuration variables should not rely on the ability to resolve
[resource references](#resource-references).

In general, a provider's configuration variables define the set of resources it is able
to manage: for example, the `aws` provider accepts the AWS region to use as a
configuration variable, which prevents a particular instance of the provider from
managing AWS resources in other regions. As noted in the [overview](#provider-lifecycle),
changes to a provider's configuration that prevent the provider from managing resources
that were created with its old configuration should require that those resources are
destroyed and recreated.

Provider configuration is performed in at most three steps:

1. [`CheckConfig`](#checkconfig), which validates configuration values and applies
   defaults computed by the provider. This step is only required when configuring a
   provider using user-supplied values, and can be skipped when using values that were
   previously processed by `CheckConfig`.
2. [`DiffConfig`](#diffconfig), which indicates whether or not the new configuration can
   be used to manage resources created with the old configuration. Note that this step is
   only applicable within contexts where new and old configuration exist (e.g. during a
   [preview](#preview) or [update](#update) of a Pulumi stack).
3. [`Configure`](#configure), which applies the inputs validated by `CheckConfig`.

#### CheckConfig

`CheckConfig` implements the semantics of a custom resource's [`Check`](#check) method,
with provider configuration in the place of resource inputs. Each call to `CheckConfig` is
provided with the provider's prior checked configuration (if any) and the configuration
supplied by the user. The provider may reject configuration values that do not conform to
the provider's schema, and may apply default values that are not statically computable.
The type of a computed default value for a property should agree with the property's
schema.

#### DiffConfig

`DiffConfig` implements the semantics of a custom resource's [`Diff`](#diff) method,
with provider configuration in the place of resource inputs and state. Each call to
`DiffConfig` is provided with the provider's prior and current configuration. If there
are any changes to the provider's configuration, those changes should be reflected in the
result of `DiffConfig`. If there are changes to the configuration that make the provider
unable to manage resources created using the prior configuration (e.g. changing an AWS
provider instance's region), `DiffConfig` should indicate that the provider must be
replaced. Because replacing a provider will require that all of the resources with
which it is associated are _also_ replaced, replacement semantics should be reserved
for changes to configuration properties that are guaranteed to make old resources
unmanagable (e.g. a change to an AWS access key should not require replacement, as the
set of resources accesible via an access key is easily knowable).

#### Configure

`Configure` applies a set of checked configuration values to a provider instance. Within
a call to `Configure`, a provider instance should use its configuration values to create
appropriate SDK instances, check connectivity, etc. If configuration fails, the provider
should return an error.

### Shutdown

Once a client has finished using a resource provider, it must shut the provider down.
A client requests that a provider shut down gracefully by calling its `SignalCancellation`
method. In response to this method, a provider should cancel all outstanding resource
operations and funtion calls. After calling `SignalCancellation`, the client calls
`Close` to inform the provider that it should release any resources it holds.

`SignalCancellation` is advisory and non-blocking; it is up to the client to decide how
long to wait after calling `SignalCancellation` to call `Close`.

## Custom Resource Lifecycle

A custom resource has a well-defined lifecycle within the scope of a Pulumi stack. When a
custom resource is registered by a Pulumi program, the Pulumi engine first determines
whether the resource is being read, imported, or managed. Each of these operations
involves a different interaction with the resource's provider.

If the resource is being read, the engine calls the resource's provider's [`Read`](#`read`) method
to fetch the resource's current state. This call to [`Read`](#`read`) includes the resource's ID and
any state provided by the user that may be necessary to read the resource.

If the resource is being imported, the engine first calls the provider's [`Read`](#`read`) method
to fetch the resource's current state and inputs. This call to [`Read`](#`read`) only inclues the
ID of the resource to import; that is, _any importable resource must be identifiable using
its ID alone_. If the [`Read`](#`read`) succeeds, the engine calls the provider's [`Check`](#`check`) method with
the inputs returned by [`Read`](#`read`) and the inputs supplied by the user. If any of the inputs
are invalid, the import fails. Finally, the engine calls the provider's [`Diff`](#`diff`) method with
the inputs returned by [`Check`](#`check`) and the state returned by [`Read`](#`read`). If the call to [`Diff`](#`diff`)
indicates that there is no difference between the desired state described by the inputs
and the actual state, the import succeeds. Otherwise, the import fails.

If the resource is being managed, the engine first looks up the last registered inputs and
last refreshed state for the resource's URN. The engine then calls the resource's
provider's [`Check`](#`check`) method with the last registered inputs (if any) and the inputs supplied
by the user. If any of the inputs are invalid, the registration fails. Otherwise, the
engine decides which operations to perform on the resource based on the difference between
the desired state described by its inputs and its actual state. If the resource does not
exist (i.e. there is no last refereshed state for its URN), the engine calls the
provider's [`Create`](#`create`) method, which returns the ID and state of the created resource. If the
resource does exist, the action taken depends on the differences (if any) between the
desired and actual state of the resource.

If the resource does exist, the engine calls the provider's [`Diff`](#`diff`) method with the
inputs returned from [`Check`](#`check`), the resource's ID, and the resource's last refreshed state.
If the result of the call indicates that there is no difference between the desired and
actual state, no operation is necessary. Otherwise, the resource is either updated (if
[`Diff`](#`diff`) does not indicate that the resource must be replaced) or replaced (if [`Diff`](#`diff`) does
indicate that the resource must be replaced).

To update a resource, the engine calls the provider's [`Update`](#`update`) method with the inputs
returned from [`Check`](#`check`), the resource's ID, and its last refreshed state. [`Update`](#`update`) returns
the new state of the resource. The resource's ID may not be changed by a call to [`Update`](#`update`).

To replace a resource, the engine first calls [`Check`](#`check`) with an empty set of prior inputs
and the inputs supplied with the resource's registration. If [`Check`](#`check`) fails, the resource
is not replaced. Otherwise, the inputs returned by this call to [`Check`](#`check`) will be used to
create the replacement resource. Next, the engine inspects the resource options supplied
with the resource's registration and result of the call to [`Diff`](#`diff`) to determine whether
the replacement can be created before the original resource is deleted. This order of
operations is preferred when possible to avoid downtime due to the lag between the
deletion of the current resource and creation of its replacement. If the replacement may
be created before the original is deleted, the engine calls the provider's [`Create`](#`create`) method
with the re-checked inputs, then later calls [`Delete`](#`delete`) with the resource's ID and original
state. If the resource must be deleted before its replacement can be created, the engine
first deletes the transitive closure of resource that depend on the resource being
replaced. Once these deletes have completed, the engine deletes the original resource by
calling the provider's [`Delete`](#`delete`) method with the resource's ID and original state. Finally,
the engine creates the replacement resource by calling [`Create`](#`create`) with the re-checked
inputs.

If a managed resource registered by a Pulumi program is not re-registered by the next
successful execution of a Pulumi progam in the resource's stack, the engine deletes the
resource by calling the resource's provider's [`Delete`](#`delete`) method with the resource's ID and
last refereshed state.

The diagram below summarizes the custom resource lifecycle. Detailed descriptions of each

![Resource Lifeycle Diagram](./resource_lifecycle.svg)

### Check

- validate inputs
- apply provider-side defaults

### Diff

- determine differences between requested config and last state
- decide whether or not diffs require replacement
- detailed diff

### Create

- create resource
- partial failures

### Update

- update resource in-place
- partial failures

### Read

- read live state given an ID

### Delete

- delete resource

## Component Resource Lifecycle

### Construct

- user-level programming model

## Provider Functions

### Invoke

### StreamInvoke

## CLI Scenarios

- preview
- update
- import
- refresh
- destroy

### Preview

- check
- diff
- create/update preview, read operation

### Update

- check
- diff
- create/update/read/delete operation

### Import

- read operation

### Refresh

- read operations

### Destroy

- delete operation

## Appendix

### Out-of-Process Plugin Lifecycle

### gRPC Interface

- feature negotiation
- data representation
