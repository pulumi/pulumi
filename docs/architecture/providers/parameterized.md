(parameterized-providers)=
# Parameterized Providers

*Parameterized providers* are a provider feature where a user can generate and
use a custom SDK based on some custom input.

Key rules:

* Each provider parameterization is run in its own provider instance.
* Parameterized package names must be unique within the program where its added.
* Parameterize is always called before Configure.
* Parameterize is always called before GetSchema.

For a provider to support parameterization it must implement the
[](pulumirpc.ResourceProvider.Parameterize) call. The
[](pulumirpc.ParameterizeRequest) will either contain the args passed via the
CLI from `pulumi package add` or `pulumi package gen-sdk`; or will be passed a
previous parameterization result (a binary blob). Parameterize will always be
called before either Configure or GetSchema are called.
[](pulumirpc.ParameterizeResponse) must return a response with the *package*
name and version, which must match the generated SDK (from the schema). The name
of the parameterized package must be unique and will be used as the first part of
all resource tokens and used to route RegisterResourceRequests back to the
Parameterized provider instance. The version of the parameterized package can be
anything the provider would like and does not have to align to the version of the
base provider. The version does not have any specific purpose right now - it's
purely informational, but is required to be set.

Once [](pulumirpc.ResourceProvider.Parameterize) has been called (in either mode)
the engine must be able to call GetSchema and Configure to start to use the
provider or generate the SDK. For replacement parameterization, once Parameterize
has been called, the provider will only be used in its parameterized form.

Once parameterized, GetSchema can be called by the engine. The GetSchema request
will contain the same name and version information as was returned from
[](pulumirpc.ResourceProvider.Parameterize) in the SubpackageName and
SubpackageVersion fields, respectively. The generated schema *must* have the same
name and version as the SubpackageName and SubpackageVersion. GetSchema must be
callable after either the "args" or "value" mode of parameterize was called. It's
expected that GetSchema doesn't have to be fast and that the engine will cache
schemas as required.

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

Currently, the only kind of parameterization is *replacement parameterization*.
In this mode, once the provider has been parameterized, it will only be used for
that parameter only; and not without the parameter or with another parameter.

In the future we might creation the option for a parameterization to be an
"extension" of the original schema. Details are yet to be worked out here, but we
expect the following differences from the replacement parameterization:

* The provider for the extension SDK might have to be the same as the base provider
* Tokens in the extension schema must have the same provider name as the base
  schema & SDKs.
* The same instance of the provider will be used from both the base SDK and the
  extension SDKs.
* GetSchema might be called multiple times - once for each of the extensions.
* Parameterize will be called once for each extension.
