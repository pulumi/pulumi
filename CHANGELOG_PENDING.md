### Improvements

- [codegen/go] - Simplify the application of object defaults in generated SDKs.
  [#8539](https://github.com/pulumi/pulumi/pull/8539)

- [codegen/{python,dotnet}] - Emit `pulumiplugin.json` unconditionally.
  [#8527](https://github.com/pulumi/pulumi/pull/8527)
  [#8532](https://github.com/pulumi/pulumi/pull/8532)

- [sdk/python] - Lookup Pulumi packages by searching for `pulumiplugin.json`.
  Pulumi packages need not be prefixed by `pulumi-` anymore.
  [#8515](https://github.com/pulumi/pulumi/pull/8515)

- [sdk/go] - Lookup packages by searching for `pulumiplugin.json`.
  Pulumi packages need not be prefixed by `github.com/pulumi/pulumi-` anymore.
  [#8516](https://github.com/pulumi/pulumi/pull/8516)

- [sdk/dotnet] - Lookup packages by searching for `pulumiplugin.json`.
  Pulumi packages need not be prefixed by `Pulumi.` anymore.
  [#8517](https://github.com/pulumi/pulumi/pull/8517)

### Bug Fixes

- [codegen/schema] - Error on type token names missing the correct prefix.
  [#8538](https://github.com/pulumi/pulumi/pull/8538)

- [codegen/go] - Fix `ElementType` for nested collection input and output types.
  [#8535](https://github.com/pulumi/pulumi/pull/8535)
