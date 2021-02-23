### Breaking Changes

- [CLI] Standardize the `--stack` flag to *not* set the stack as current (i.e. setStack=false) across CLI commands.
  [#6840](https://github.com/pulumi/pulumi/pull/6840)

- [Automation/*] All operations use `--stack` to specify the stack instead of running `select stack` before the operation.
  [#6840](https://github.com/pulumi/pulumi/pull/6840)

### Enhancements

- [sdk/nodejs] Handle providers for RegisterResourceRequest
  [#6795](https://github.com/pulumi/pulumi/pull/6795)

- [automation/dotnet] Remove dependency on Gprc.Tools for F# / Paket compatibility
  [#6793](https://github.com/pulumi/pulumi/pull/6793)

### Bug Fixes


- [codegen] Fix codegen for types that are used by both resources and functions.
  [#6811](https://github.com/pulumi/pulumi/pull/6811)

- [sdk/python] Fix bug in `get_resource_module` affecting resource hydration.
  [#6833](https://github.com/pulumi/pulumi/pull/6833)
