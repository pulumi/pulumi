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

- [sdk/go] - Emit `pulumiplugin.json`
  [#8530](https://github.com/pulumi/pulumi/pull/8530)

- [cli] - Always use locking in filestate backends. This feature was
  previously disabled by default and activated by setting the
  `PULUMI_SELF_MANAGED_STATE_LOCKING=1` environment variable.
  [#8565](https://github.com/pulumi/pulumi/pull/8565)

- [sdk/dotnet] - Fixes a rare race condition that sporadically caused
  NullReferenceException to be raised when constructing resources
  [#8495](https://github.com/pulumi/pulumi/pull/8495)

### Bug Fixes

- [codegen/schema] - Error on type token names that are not allowed (schema.Name
  or specified in allowedPackageNames).
  [#8538](https://github.com/pulumi/pulumi/pull/8538)
  [#8558](https://github.com/pulumi/pulumi/pull/8558)

- [codegen/go] - Fix `ElementType` for nested collection input and output types.
  [#8535](https://github.com/pulumi/pulumi/pull/8535)
