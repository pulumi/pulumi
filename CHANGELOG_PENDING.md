### Breaking


### Improvements

- [automation/dotnet] Allow null environment variables
  [#6687](https://github.com/pulumi/pulumi/pull/6687)

### Bug Fixes

- [automation/dotnet] Environment variable value type is now nullable.
  [#6520](https://github.com/pulumi/pulumi/pull/6520)

- [sdk/nodejs] Fix `Construct` to wait for child resources of a multi-lang components to be created.
  [#6452](https://github.com/pulumi/pulumi/pull/6452

- [sdk/python] Fix serialization bug if output contains 'items' property.
  [#6701](https://github.com/pulumi/pulumi/pull/6701)
  
- [sdk/go] Use ioutil.ReadFile to avoid forcing 1.16 upgrade.
  [#6703](https://github.com/pulumi/pulumi/pull/6703)
