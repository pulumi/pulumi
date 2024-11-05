(parameterized-providers)=
# Parameterized providers

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
