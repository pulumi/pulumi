### Improvements
  
- [sdk/dotnet] - Fix async await warnings.
  [#7537](https://github.com/pulumi/pulumi/pull/7537)

- [codegen/dotnet] - Emit dynamic config-getters.
  [#7549](https://github.com/pulumi/pulumi/pull/7549)

- [sdk/python] - Support for authoring resource methods in Python.
  [#7555](https://github.com/pulumi/pulumi/pull/7555)

- [sdk/{go,dotnet}] - Admit non-asset/archive values when unmarshalling into assets and archives.
  [#7579](https://github.com/pulumi/pulumi/pull/7579)

### Bug Fixes

- [sdk/dotnet] - Fix for race conditions in .NET SDK that used to
  manifest as a `KeyNotFoundException` from `WhileRunningAsync`.
  [#7529](https://github.com/pulumi/pulumi/pull/7529)

- [sdk/go] - Fix target and replace options for the Automation API.
  [#7426](https://github.com/pulumi/pulumi/pull/7426)

- [cli] - Don't escape special characters when printing JSON.
  [#7593](https://github.com/pulumi/pulumi/pull/7593)

- [sdk/go] - Fix panic when marshaling `self` in a method.
  [#7604](https://github.com/pulumi/pulumi/pull/7604)
