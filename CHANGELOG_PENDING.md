### Improvements

- [sdk/dotnet] - Add `PluginDownloadURL` as a resource option. When provided by
  the schema, `PluginDownloadURL` will be baked into `new Resource` and `Invoke`
  requests in generated SDKs. 
  [#8739](https://github.com/pulumi/pulumi/pull/8739)

- [sdk] - Allow property paths to accept `[*]` as sugar for `["*"]`.
  [#8743](https://github.com/pulumi/pulumi/pull/8743)

- [sdk/dotnet] Add `Union.Bimap` function for converting both sides of a union at once.
  [#8733](https://github.com/pulumi/pulumi/pull/8733)

### Bug Fixes

- [sdk/dotnet] Allow `Output<Union>` to be converted to `InputUnion`.
  [#8733](https://github.com/pulumi/pulumi/pull/8733)