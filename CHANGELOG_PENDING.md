### Improvements

- [sdk/{nodejs,go,python}] - Add `PluginDownloadURL` as a resource option. When provided by
  the schema, `PluginDownloadURL` will be baked into `new Resource` and `Invoke`
  requests in generated SDKs. 
  [#8698](https://github.com/pulumi/pulumi/pull/8698)
  [#8690](https://github.com/pulumi/pulumi/pull/8690)
  [#8692](https://github.com/pulumi/pulumi/pull/8692)

### Bug Fixes

- [auto/python] - Fixes an issue with exception isolation in a
  sequence of inline programs that caused all inline programs to fail
  after the first one failed
  [#8693](https://github.com/pulumi/pulumi/pull/#8693)

- [codegen/nodejs] - Generate an install script that runs `pulumi plugin install` with 
  the `--server` flag when necessary.
  [#8730](https://github.com/pulumi/pulumi/pull/8730)
