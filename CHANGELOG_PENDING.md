### Improvements

- [sdk/python] - Support for authoring resource methods in Python.
  [#7555](https://github.com/pulumi/pulumi/pull/7555)

### Bug Fixes

- [sdk/dotnet] - Fix for race conditions in .NET SDK that used to
  manifest as a `KeyNotFoundException` from `WhileRunningAsync`
  [#7529](https://github.com/pulumi/pulumi/pull/7529)
