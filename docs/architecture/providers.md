(providers)=
# Providers

The term "provider" can mean different things in different contexts. When we
talk about Pulumi programs, we often talk about *provider resources* such as
that provided by the `aws.Provider` class in the `@pulumi/aws` NodeJS/TypeScript
package. Or, we might simply mean a cloud provider, such as AWS or GCP. In the
context of the wider Pulumi architecture though, a provider (specifically, a
*resource provider*) is a Pulumi [plugin](plugins) that implements [a
standardized gRPC interface](pulumirpc.ResourceProvider) for handling
communication with a third-party service (usually a cloud service, such as AWS,
GCP, or Azure):

* *Configuration* methods are designed to allow a consumer to configure a
  provider instance in some way. The [](pulumirpc.ResourceProvider.Configure)
  call is the most common example of this, allowing a caller to e.g. specify the
  AWS region that a provider should use operate in.
  [](pulumirpc.ResourceProvider.Parameterize) is a similar method that operates
  at a higher level, allowing a caller to influence more deeply how a provider
  works (see [the section on parameterized providers](parameterized-providers)
  for more).
* *Schema* endpoints allow a caller to interrogate the resources and functions
  that a provider exposes. The [](pulumirpc.ResourceProvider.GetSchema) method
  returns a provider's schema, which includes the set of resources and functions
  that the provider supports, as well as the properties and inputs that each
  resource and function expects. This is the primary driver for the various code
  generation processes that Pulumi uses, such as that underpinning SDK
  generation.
* *Lifecycle* methods expose the typical [](pulumirpc.ResourceProvider.Create),
  [](pulumirpc.ResourceProvider.Read), [](pulumirpc.ResourceProvider.Update),
  and [](pulumirpc.ResourceProvider.Delete) (CRUD) operations that allow clients
  to manage provider resources. The [](pulumirpc.ResourceProvider.Check),
  [](pulumirpc.ResourceProvider.Diff), and
  [](pulumirpc.ResourceProvider.Construct) methods also fall into this category,
  as discussed in [resource registration](resource-registration).
* *Functions* can be invoked on a provider through the
  [](pulumirpc.ResourceProvider.Invoke) call, or on specific resources by using
  the [](pulumirpc.ResourceProvider.Call) operation. Functions are typically
  used to perform operations that don't fit into the CRUD model, such as
  retrieving a list of availability zones, or available regions, etc.

While any program which implements the [](pulumirpc.ResourceProvider) interface
can be interfaced with by the Pulumi engine, in practice most Pulumi providers
are built in a handful of ways:

* *Bridged providers* wrap a Terraform provider using the [Pulumi Terraform
  bridge](https://github.com/pulumi/pulumi-terraform-bridge). The majority of
  Pulumi providers are built in this way.
* The [`pulumi-go-provider`](https://github.com/pulumi/pulumi-go-provider)
  library provides a high-level API for writing a provider in Go, without having
  to worry about low-level gRPC details.
* *Dynamic providers* can be written as part of a Pulumi program. They are
  discussed in more detail [later in this document](dynamic-providers).
* Each language SDK provides a `provider` package that offers low-level
  primitives for writing a provider in that language (e.g.
  <gh-file:pulumi#sdk/nodejs/provider> for NodeJS,
  <gh-file:pulumi#sdk/python/lib/pulumi/provider> for Python, and
  <gh-file:pulumi#sdk/go/pulumi/provider> for Go). These packages are in varying
  states of completeness and neatness and are generally only used for building
  [component providers](component-providers).

A provider binary is typically named `pulumi-resource-<provider-name>`;
`pulumi-resource-aws` is one example.

(default-providers)=
## Default providers

A *default provider* for a package and version is the provider instance that
Pulumi will use to manage resources that do not have a provider explicitly
specified (either directly as a resource option or indirectly via a parent, for
instance). Consider for example the following TypeScript program that creates
an S3 bucket in AWS:

```typescript
import * as aws from "@pulumi/aws"

new aws.s3.Bucket("my-bucket")
```

The `Bucket` constructor will yield a [](pulumirpc.RegisterResourceRequest) such
as the following:

```
RegisterResourceRequest{
  type: "aws:s3/bucket:Bucket",
  name: "my-bucket",
  parent: "urn:pulumi:dev::project::pulumi:pulumi:Stack::project",
  custom: true,
  object: {},
  version: "4.16.0",
}
```

The absence of a `provider` field in this request will cause the engine to use a
default provider for the `aws` package at version 4.16.0. The engine's
[](pulumirpc.ResourceMonitor) implementation ensures that only a single default
provider instance exists for each package version, and only creates default
provider instances on demand (that is, when a resource that requires one is
registered). Default provider instances are created by synthesizing appropriate
`RegisterResourceEvent`s with inputs sourced from the stack's configuration
values for the relevant provider package. In the example above, the default AWS
provider would be configured using any stack configuration values whose keys
begin with `aws:` (e.g. `aws:region`).

Changing the above example to use an explicit provider will prevent a default
provider from being used:

```typescript
import * as aws from "@pulumi/aws"

const usWest2 = new aws.Provider("us-west-2", { region: "us-west-2" })

new aws.s3.Bucket("my-bucket", {}, { provider: usWest2 })
```

This will yield a `RegisterResourceRequest` whose `provider` field references
the explicitly constructed entity:

```
RegisterResourceRequest{
  type: "aws:s3/bucket:Bucket",
  name: "my-bucket",
  parent: "urn:pulumi:dev::project::pulumi:pulumi:Stack::project",
  custom: true,
  object: {},
  provider: "urn:pulumi:dev::project::pulumi:providers:aws::us-west-2::308b79ee-8249-40fb-a203-de190cb8faa8",
  version: "4.16.0",
}
```

Note that the explicit provider *itself* is registered as a resource, and its
constructor will emit its own `RegisterResourceRequest` with the appropriate
name, type, parent, and so on.

(mlcs)=
(component-providers)=
## Component providers

Authors of Pulumi programs can use [component resources](component-resources) to
logically group related resources together. For instance, a TypeScript program
might specify a component that combines AWS and PostgreSQL providers to abstract
the management of an RDS database and logical databases within it:

```typescript
import * as aws from "@pulumi/aws"
import * as postgresql from "@pulumi/postgresql"

class Database extends pulumi.ComponentResource {
  constructor(name: string, args: DatabaseArgs, opts?: pulumi.ComponentResourceOptions) {
    super("my:database:Database", name, args, opts)

    const rds = new aws.rds.Instance("my-rds", { ... }, { parent: this })
    const pg = new postgresql.Database("my-db", { ... }, { parent: this })

    ...
  }
}
```

This component can then be used just like any other Pulumi resource:

```typescript
const db = new Database("my-db", { ... })
```

...if the program is written in the same language as the component (in this
case, TypeScript). In some cases however it would be great if components could
be reused in multiple languages, since components provide a natural means to
abstract and reuse infrastructure.

Enter *component providers* (also known as *multi-language components*, or
MLCs). Component providers allow components to be written in one language and
used in another (or rather, any other). Typically we refer to such components as
*remote*, in contrast with *local* components written directly in and alongside
the user's program as above.

Under the hood, component providers expose remote components by implementing the
[](pulumirpc.ResourceProvider.Construct) method. The engine automatically calls
`Construct` when it sees a request to create a remote
component.[^engine-construct] Indeed, since providers and gRPC calls are the key
to making custom resources consumable in any language, exposing components
through the same interface is a natural extension of the Pulumi model.

[^engine-construct]:
    See [resource registration](resource-monitor) for more information.

Just as the body of a component resource is largely concerned with instantiating
other resources, so is the implementation of `Construct` for a component
provider. Whereas a custom resource's [](pulumirpc.ResourceProvider.Create)
method can be expected to make a "raw" call to some underlying cloud provider
API (for instance), [](pulumirpc.ResourceProvider.Construct) is generally only
concerned with registering child resources and their desired state. For this
reason, [](pulumirpc.ConstructRequest) includes a `monitorEndpoint` so that the
component provider can itself make
[](pulumirpc.ResourceMonitor.RegisterResource) calls *back* to the [deployment's
resource monitor](resource-monitor) to register these child resources. Child
resources registered by `Construct` consequently end up in the calling program's
state just like any other resource, and proceed through [step
generation](step-generation), etc. in exactly the same way. That is to say, once
`Construct` has been called, the engine does not really care whether or not a
resource registration came from the program or a remote component.

:::{note}
"Ordinary" resource providers and component providers are not mutually exclusive
-- it is perfectly sensible for a provider to implement both the
[](pulumirpc.ResourceProvider.Construct) and
[](pulumirpc.ResourceProvider.Create)/[](pulumirpc.ResourceProvider.Read)/...
methods.
:::

(dynamic-providers)=
## Dynamic providers

[*Dynamic
providers*](https://www.pulumi.com/docs/concepts/resources/dynamic-providers/)
are a Pulumi feature that allows the core logic of a provider to be defined and
managed within the context of a Pulumi program. This is in contrast to a normal
("real", sometimes "side-by-side") provider, whose logic is encapsulated as a
separate [plugin](plugins) for use in any program. Dynamic providers are
presently only supported in NodeJS/TypeScript and Python. They work as follows:

* The SDK defines two types:
  * That of *dynamic providers* -- objects with methods for the lifecycle
    methods that a gRPC provider would normally offer (CRUD, diff, etc.).
  * That of *dynamic resources* -- those that are managed by a dynamic provider.
    This type specialises (e.g. by subclassing in NodeJS and Python) the SDK's
    core resource type so that all dynamic resources *have the same Pulumi
    package* -- `pulumi-nodejs` for NodeJS and `pulumi-python` for Python.

  These are located in <gh-file:pulumi#sdk/nodejs/dynamic/index.ts> in
  NodeJS/TypeScript and
  <gh-file:pulumi#sdk/python/lib/pulumi/dynamic/dynamic.py> in Python.
* The SDK also defines a "real" provider that implements the gRPC interface and
  manages the lifecycle of dynamic resources. This provider is named according
  to the single package name used for all dynamic resources. See
  <gh-file:pulumi#sdk/nodejs/cmd/dynamic-provider/index.ts> for NodeJS and
  <gh-file:pulumi#sdk/python/lib/pulumi/dynamic/__main__.py> for Python.

* A user extends the types defined by the SDK in order to implement one or more
  dynamic providers and resources that belong to those providers. They use these
  resources in their program like any other.
* When a dynamic resource class is instantiated, it *captures the provider
  instance that manages it* and *serializes this provider instance* as part of
  the resource's properties.
  * In NodeJS, serialization is performed by capturing and mangling the source
    code of the provider and any dependencies by (ab)using v8 primitives -- see
    <gh-file:pulumi#sdk/nodejs/runtime/closure> for the gory details.
  * In Python, serialization is performed by pickling the dynamic provider
    instance -- see <gh-file:pulumi#sdk/python/lib/pulumi/dynamic/dynamic.py>'s
    use of `dill` for more on this.
* The serialized provider state is then stored as a property on the dynamic
  resource. It is consequently sent to the engine as part of lifecycle calls
  (check, diff, create, etc.) like any other property.
* When the engine receives requests pertaining to dynamic resources, the fixed
  package (`pulumi-nodejs` or `pulumi-python`) will cause it to make provider
  calls against the "real" provider defined in the SDK.
* The provider proxies these calls to the code the user wrote by deserializing
  and hydrating the provider instance from the resource's properties and
  invoking the appropriate code.

These implementation choices impose a number of limitations:

* Serialized/pickled code is brittle and simply doesn't work in all cases. Some
  features are supported and some aren't, depending on the language and
  surrounding context. Dependency management (both within the user's program and
  as it relates to third-party packages such as those from NPM or PyPi) is
  challenging.
* Even when code works once, or in one context, it might not work later on. If
  e.g. absolute paths specific to one machine form part of the provider's code
  (or the code of its dependencies), the fact that these are serialized into the
  Pulumi state means that on later hydration, a program that worked before might
  not work again.
* Related to the problem of state serialization is the fact that dynamic
  provider state is only updated *when the program runs*. It is therefore not
  possible in general to e.g. change the code of a dynamic provider and expect
  an operation like `destroy` (which does not run the program) to pick up the
  changes.

(parameterized-providers)=
## Parameterized providers

*Parameterized providers* are a feature of Pulumi that allows a caller to change
a provider's behaviour at runtime in response to a
[](pulumirpc.ResourceProvider.Parameterize) call. Where a
[](pulumirpc.ResourceProvider.Configure) call allows a caller to influence
provider behaviour at a high level (e.g. by specifying the region in which an
AWS provider should operate), a [](pulumirpc.ResourceProvider.Parameterize) call
may change the set of resources and functions that a provider offers (that is,
its schema). A couple of examples of where this is useful are:

* Dynamically bridging Terraform providers. The
  [`pulumi-terraform-bridge`](https://github.com/pulumi/pulumi-terraform-bridge)
  can be used to build a Pulumi provider that wraps a Terraform provider. This
  is an "offline" or "static" process -- provider authors write a Go program
  that imports the bridge library and uses it to wrap a specific Terraform
  provider. The resulting provider can then be published as a Pulumi plugin and
  its [](pulumirpc.ResourceProvider.GetSchema) method used to generate
  language-specific SDKs which are also published. Generally, the Go program
  that authors write is the same (at least in structure) for many if not all
  providers.
  [`pulumi-terraform-provider`](https://github.com/pulumi/pulumi-terraform-provider)
  is a parameterized provider that exploits this to implement a provider that
  can bridge an arbitrary Terraform provider *at runtime*.
  `pulumi-terraform-provider` accepts the name of the Terraform provider to
  bridge and uses the existing `pulumi-terraform-bridge` machinery to perform
  the bridging and schema loading *in response to the `Parameterize` call*.
  Subsequent calls to `GetSchema` and other lifecycle methods will then behave
  as if the provider had been statically bridged.

* Managing Kubernetes clusters with [custom resource definitions
  (CRDs)](https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/custom-resources/).
  Kubernetes allows users to define their own resource types outside the
  standard set of APIs (`Pod`, `Service`, and so on). By default, the Pulumi
  Kubernetes provider does not know about these resources, and so cannot expose
  them in its schema and by extension offer SDK/code completion for interacting
  with them. Parameterization offers the possibility for the provider to accept
  a parameter describing a set of CRDs, enabling it to then extend its schema to
  expose them to programs and SDK generation.

As hinted at by the above examples, [](pulumirpc.ResourceProvider.Parameterize)
encodes a provider-specific *parameter* that is used to influence the provider's
behaviour. The parameter passed in the [](pulumirpc.ParameterizeRequest) can
take two forms, corresponding to the two contexts in which parameterization
typically occurs:

* When generating an SDK (e.g. using a `pulumi package add` command), we need to
  boot up a provider and parameterize it using only information from the
  command-line invocation. In this case, the parameter is a string array
  representing the command-line arguments (`args`).
* When interacting with a provider as part of program execution, the parameter
  is *embedded in the SDK*, so as to free the program author from having to know
  whether a provider is parameterized or not. In this case, the parameter is a
  provider-specific bytestring (`value`). This is intended to allow a provider
  to store arbitrary data that may be more efficient or practical at program
  execution time, after SDK generation has taken place. This value is
  base-64-encoded when embedded in the SDK.

:::{warning}
In the absence of parameterized providers, it is generally safe to assume that a
resource's package name matches exactly the name of the provider
[plugin](plugins) that provides that package. For example, an `aws:s3:Bucket`
resource could be expected to be managed by the `aws` provider plugin, which in
turn would live in a binary named `pulumi-resource-aws`. In the presence of
parameterized providers, this is *not* necessarily the case. Dynamic Terraform
providers are a great example of this -- if a user were to dynamically bridge an
AWS Terraform provider, the same `aws:s3:Bucket` resource might be provided by
the `terraform` provider plugin (with a parameter of `aws:<version>` or similar,
for example).
:::

(replacement-extension-providers)=
## Replacement and extension parameterization
