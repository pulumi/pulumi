### Improvements

- [codegen/python] - Emit `pulumiplugin.json` unconditionally. 
  [#8527](https://github.com/pulumi/pulumi/pull/8527)

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

