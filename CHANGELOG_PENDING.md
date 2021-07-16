
### Improvements


- [schema] - Support for unmarshaling/marshaling schemas from/to YAML
  [#7509](https://github.com/pulumi/pulumi/pull/7509)

### Bug Fixes

- [sdk/dotnet] - Fix for race conditions in .NET SDK that used to
  manifest as a `KeyNotFoundException` from `WhileRunningAsync`
  [#7529](https://github.com/pulumi/pulumi/pull/7529)
