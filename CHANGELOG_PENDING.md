### Improvements

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
