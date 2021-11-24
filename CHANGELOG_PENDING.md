### Improvements

- [codegen/go] - Remove `ResourcePtr` types from generated SDKs. Besides being
  unnecessary--`Resource` types already accommodate `nil` to indicate the lack of
  a value--the implementation of `Ptr` types for resources was incorrect, making
  these types virtually unusable in practice.
  [#8449](https://github.com/pulumi/pulumi/pull/8449)

### Bug Fixes

- [cli/engine] - Accurately computes the fields changed when diffing with unhelpful providers. This
  allows the `replaceOnChanges` feature to be respected for all providers.
  [#8488](https://github.com/pulumi/pulumi/pull/8488)

- [codegen/go] - Respect default values in Pulumi object types.
  [#8411](https://github.com/pulumi/pulumi/pull/8400)
