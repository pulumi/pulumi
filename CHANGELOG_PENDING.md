### Improvements
  
- [codegen/dotnet] - Emit dynamic config-getters.
  [#7549](https://github.com/pulumi/pulumi/pull/7549)

- [sdk/python] - Support for authoring resource methods in Python.
  [#7555](https://github.com/pulumi/pulumi/pull/7555)

### Bug Fixes

- [sdk/dotnet] - Fix for race conditions in .NET SDK that used to
  manifest as a `KeyNotFoundException` from `WhileRunningAsync`
  [#7529](https://github.com/pulumi/pulumi/pull/7529)

- [sdk/go] - Fix target and replace options for the Automation API
  [#7426](https://github.com/pulumi/pulumi/pull/7426)

