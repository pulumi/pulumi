### Improvements

- [dotnet/sdk] Add create unknown to output utilities.
  [#7173](https://github.com/pulumi/pulumi/pull/7173)

- [dotnet] Fix Resharper code issues.
  [#7178](https://github.com/pulumi/pulumi/pull/7178)

- [codegen] - Include properties with an underlying type of string on Go provider instances.

### Bug Fixes

- [sdk/python] - Fix regression in behaviour for `Output.from_input({})`

- [sdk/python] - Fix hanging deployments and improve error messages
  for programs with incorrect typings for output values
  [#7049](https://github.com/pulumi/pulumi/pull/7049)

- [codegen/python] - Rename conflicting ResourceArgs classes
  [#7171](https://github.com/pulumi/pulumi/pull/7171)
