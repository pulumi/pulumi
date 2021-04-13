### Breaking


### Improvements

- [sdk/nodejs] Add program side caching for dynamic provider serialization behind env var
  [#6673](https://github.com/pulumi/pulumi/pull/6673)

- [automation/dotnet] Allow null environment variables
  [#6687](https://github.com/pulumi/pulumi/pull/6687)

- [sdk/python] Python support for authoring resource providers for
  multi-language components.
  [#6715](https://github.com/pulumi/pulumi/pull/6715)

### Bug Fixes

- [automation/dotnet] Environment variable value type is now nullable.
  [#6520](https://github.com/pulumi/pulumi/pull/6520)

- [sdk/nodejs] Fix `Construct` to wait for child resources of a multi-lang components to be created.
  [#6452](https://github.com/pulumi/pulumi/pull/6452

- [sdk/python] Fix serialization bug if output contains 'items' property.
  [#6701](https://github.com/pulumi/pulumi/pull/6701)

- [sdk/go] Use ioutil.ReadFile to avoid forcing 1.16 upgrade.
  [#6703](https://github.com/pulumi/pulumi/pull/6703)

- [automation/dotnet] Fix GetConfigValueAsync failing to deserialize
  [#6698](https://github.com/pulumi/pulumi/pull/6698)
