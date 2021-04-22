### Breaking Changes

- We documented a breaking change for 3.0 for stack selection behavior (see https://www.pulumi.com/docs/get-started/install/migrating-3.0/#updated-cli-behavior-in-pulumi-30). Unfortunately, the initial release did not include that change. 
  We apologize for any confusion or inconvenience this may have caused and have now correctly added that behavior.
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
