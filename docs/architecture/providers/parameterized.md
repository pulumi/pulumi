(parameterized-providers)=
# Parameterized Providers

*Parameterized providers* are a Pulumi feature that allows a user to change a
provider's behaviour at runtime. The user generates an SDK by running
`pulumi package add` or `pulumi package gen-sdk` with additional command-line
arguments (`args`). The provider is passed the arguments and generates an SDK
for the user, which also includes custom provider metadata. When the SDK is used
from a Pulumi program, the provider is "parameterized" with the metadata from
within the generated SDK so it's ready to use.

Therefore, when starting a provider to be parameterized, the parameterization
call can take two forms:

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

(provider-implementation)=
## Provider implementation

Requirements and conditions for "replacement" parameterization:

* Each provider parameterization is run in its own provider instance.
* Parameterized package names must be unique within the program where its added.
* Parameterization is either via CLI args *or* embedded metadata.
* [](pulumirpc.ResourceProvider.Parameterize) is always called before [](pulumirpc.ResourceProvider.Configure).
* [](pulumirpc.ResourceProvider.Parameterize) is always called before [](pulumirpc.ResourceProvider.GetSchema).

:::{note}
Currently, we only support "replacement" parameterization where generated SDKs
don't reference the provider's original SDK. See
[Replacement and extension parameterization](#replacement-extension-parameterization)
for discussion of what alternative implementations might be possible in the future.
:::

For a provider to support parameterization it must implement the
[](pulumirpc.ResourceProvider.Parameterize) call. The
[](pulumirpc.ParameterizeRequest) will either contain the args passed via the
CLI from `pulumi package add` or `pulumi package gen-sdk`; or will be passed the
previous generated metadata binary blob. [](pulumirpc.ResourceProvider.Parameterize)
will always be called before either [](pulumirpc.ResourceProvider.Configure)
or [](pulumirpc.ResourceProvider.GetSchema) are called.
[](pulumirpc.ParameterizeResponse) must return a response with the *parameterized
package* name and version. The name of the parameterized package must be unique
and will be used as the first part of all resource tokens and used to route
RegisterResourceRequests back to the parameterized provider instance. The
version of the parameterized package can be anything the provider would like and
does not have to align to the version of the base provider. The version does not
have any specific purpose right now - it's purely informational, but is
required to be set and to match the parameterized schema too.

Once [](pulumirpc.ResourceProvider.Parameterize) has been called (with either CLI
args or the binary metadata) that provider instance will only be used with that
specific parameterization. It will not be re-parameterized or un-parameterized.

Once parameterized, GetSchema can be called by the engine. The GetSchema request
will contain the same name and version information as was returned from
[](pulumirpc.ResourceProvider.Parameterize) in the SubpackageName and
SubpackageVersion fields, respectively. The generated schema *must* have the same
name and version as the SubpackageName and SubpackageVersion. GetSchema must be
callable after either the "args" or "value" mode of parameterize was called. It's
expected that GetSchema doesn't have to be fast and that the engine will cache
schemas as required.

Once parameterized, the engine may then resume the usual provider lifecycle
operations such as [](pulumirpc.ResourceProvider.Configure) and start interacting
with resources.

## Example uses of parameterization

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

:::{warning}
In the absence of parameterized providers, it is generally safe to assume that a
resource's package name exactly matches exactly the name of the provider
[plugin](plugins) that provides that package. For example, an `aws:s3:Bucket`
resource could be expected to be managed by the `aws` provider plugin, which in
turn would live in a binary named `pulumi-resource-aws`. In the presence of
parameterized providers, this is *not* necessarily the case. Dynamic Terraform
providers are a great example of this -- if a user were to dynamically bridge an
AWS Terraform provider, the same `aws:s3:Bucket` resource might be provided by
the `terraform` provider plugin (with a parameter of `aws:<version>` or similar,
for example).
:::

(replacement-extension-parameterization)=
## Replacement and extension parameterization

Currently, the only kind of parameterization is *replacement parameterization*.
In this mode, once the provider has been parameterized, it will be used for that
parameter only; and not without the parameter or with another parameter.

Future enhancements may include the option for a parameterization to be an
"extension" of the original schema. Details are yet to be worked out here, but we
expect a number of semantic differences from the replacement parameterization, but
this mode would be explicitly opted into by providers, so there are no concerns
around backward compatibility.
