**Please Note:** Release v3.5.0 failed in our build pipeline so will be rebuilt with a new tag of v3.5.1

### Improvements

- [cli] - Added support for passing custom paths that need
  to be watched by the `pulumi watch` command.
  [#7115](https://github.com/pulumi/pulumi/pull/7247)

- [auto/nodejs] - Fail early when multiple versions of `@pulumi/pulumi` are detected in nodejs inline programs.'
  [#7349](https://github.com/pulumi/pulumi/pull/7349)

- [sdk/go] - Introduce versions of `ResourceOption` accepting `ResourceInput` instead of `Resource`.
  [#7350](https://github.com/pulumi/pulumi/pull/7350/)

  An example scenario enabled by this change is setting a parent to a
  resource that is only known in the `Output` layer. Such scenarios
  commonly occur with multi-lang components.

  func example(ctx *pulumi.Context, parentOutput ResourceOutput) {
     s3.NewBucketObject(ctx, "my-object", &s3.BucketObjectArgs{}, pulumi.ParentInput(parentOutput))
  }

  List of all new types and functions:

  type ResourceInput
  type ProviderResourceInput
  type ProviderResourceOutput

  func DependsOnInputs(o []ResourceInput) ResourceOption
  func ParentInput(r ResourceInput) ResourceOrInvokeOption
  func ProviderInput(pri ProviderResourceInput) ResourceOrInvokeOption
  func ProviderInputMap(inputMap map[string]ProviderResourceInput) ResourceOption
  func ProviderInputs(o ...ProviderResourceInput) ResourceOption

  Automatic promotion of `Resource` to `ResourceInput` is not
  currently supported. Use the following helpers instead:

  func NewResourceInput(resource Resource) ResourceInput
  func NewProviderResourceInput(resource ProviderResource) ProviderResourceInput


### Bug Fixes

- [sdk/dotnet] - Fix swallowed nested exceptions with inline program so they correctly bubble to consumer
  [#7323](https://github.com/pulumi/pulumi/pull/7323)

- [sdk/go] - Specify known when creating outputs for construct.
  [#7343](https://github.com/pulumi/pulumi/pull/7343)

- [cli] - Fix passphrase rotation.
  [#7347](https://github.com/pulumi/pulumi/pull/7347)

- [multilang/python] - Fix nested module generation.
  [#7353](https://github.com/pulumi/pulumi/pull/7353)

- [multilang/nodejs] - Fix a hang when an error is thrown within an apply in a remote component.
  [#7365](https://github.com/pulumi/pulumi/pull/7365)
