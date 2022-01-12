### Improvements

- [sdk/all] - Add `PluginDownloadURL` as a resource option. When provided by
  the schema, `PluginDownloadURL` will be baked into `new Resource` and `Invoke`
  requests in generated SDKs.
  [#8698](https://github.com/pulumi/pulumi/pull/8698)
  [#8690](https://github.com/pulumi/pulumi/pull/8690)
  [#8692](https://github.com/pulumi/pulumi/pull/8692)
  [#8702](https://github.com/pulumi/pulumi/pull/8702)

- [codegen/dotnet] - Allow namespacing `Pulumi` to replace the .NET global
  namespace in generated programs.
  [#8735](https://github.com/pulumi/pulumi/pull/8735)

### Bug Fixes

- [auto/python] - Fixes an issue with exception isolation in a
  sequence of inline programs that caused all inline programs to fail
  after the first one failed
  [#8693](https://github.com/pulumi/pulumi/pull/8693)
