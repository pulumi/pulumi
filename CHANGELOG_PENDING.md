### Improvements

- [codegen/docs] Edit docs codegen to document `$fnOutput` function
  invoke forms in API documentation.
  [#8287](https://github.com/pulumi/pulumi/pull/8287)


### Bug Fixes

- [automation/python] - Fix deserialization of events.
  [#8375](https://github.com/pulumi/pulumi/pull/8375)

- [sdk/dotnet] - Fixes failing preview for programs that call data
  sources (`F.Invoke`) with unknown outputs
  [#8339](https://github.com/pulumi/pulumi/pull/8339)

- [programgen/go] - Don't change imported resource names.
  [#8353](https://github.com/pulumi/pulumi/pull/8353)

- [codegen/typescript] - Respect default values in Pulumi object types.
  [#8400](https://github.com/pulumi/pulumi/pull/8400)
