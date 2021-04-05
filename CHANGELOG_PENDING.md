### Breaking


### Improvements

- [sdk/nodejs] Add program side caching for dynamic provider serialization behind env var
  [#6673](https://github.com/pulumi/pulumi/pull/6673)

- [automation/dotnet] Allow null environment variables
  [#6687](https://github.com/pulumi/pulumi/pull/6687)

### Bug Fixes

- [automation/dotnet] Environment variable value type is now nullable.
  [#6520](https://github.com/pulumi/pulumi/pull/6520)

- [sdk/nodejs] Fix `Construct` to wait for child resources of a multi-lang components to be created.
  [#6452](https://github.com/pulumi/pulumi/pull/6452

- [sdk/python] Fix serialization bug if output contains 'items' property.
  [#6701](https://github.com/pulumi/pulumi/pull/6701)
