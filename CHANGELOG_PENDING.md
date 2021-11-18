### Improvements

- [codegen/go] - Remove `ResourcePtr` types from generated SDKs. Besides being
  unnecessary--`Resource` types already accommodate `nil` to indicate the lack of
  a value--the implementation of `Ptr` types for resources was incorrect, making
  these types virtually unusable in practice.
  [#8449](https://github.com/pulumi/pulumi/pull/8449)

### Bug Fixes

