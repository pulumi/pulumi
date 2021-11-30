### Improvements

- [cli] - Add support for pulumiplugin.json for .NET and Go
  [#8127](https://github.com/pulumi/pulumi/pull/8127)

- [codegen/go] - Remove `ResourcePtr` types from generated SDKs. Besides being
  unnecessary--`Resource` types already accommodate `nil` to indicate the lack of
  a value--the implementation of `Ptr` types for resources was incorrect, making
  these types virtually unusable in practice.
  [#8449](https://github.com/pulumi/pulumi/pull/8449)

- Allow interpolating plugin custom server URLs.
  [#8507](https://github.com/pulumi/pulumi/pull/8507)

### Bug Fixes

- [codegen/go] - Respect default values in Pulumi object types.
  [#8411](https://github.com/pulumi/pulumi/pull/8400)
