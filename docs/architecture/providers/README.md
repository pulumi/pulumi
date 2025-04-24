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
  discussed in more detail [in their own section of this
  documentation](dynamic-providers).
* Each language SDK provides a `provider` package that offers low-level
  primitives for writing a provider in that language (e.g.
  <gh-file:pulumi#sdk/nodejs/provider> for NodeJS,
  <gh-file:pulumi#sdk/python/lib/pulumi/provider> for Python, and
  <gh-file:pulumi#sdk/go/pulumi/provider> for Go). These packages are in varying
  states of completeness and neatness and are generally only used for building
  [component providers](component-providers).

A provider binary is typically named `pulumi-resource-<provider-name>`;
`pulumi-resource-aws` is one example. The following pages provide more
information on the various types of providers, their modes of operation, and
their implementation:

:::{toctree}
:maxdepth: 1
:titlesonly:

/docs/architecture/providers/built-in
/docs/architecture/providers/default
/docs/architecture/providers/components
/docs/architecture/providers/dynamic
/docs/architecture/providers/parameterized
:::
