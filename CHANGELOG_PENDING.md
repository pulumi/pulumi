### Improvements


- [codegen] - Include properties with an underlying type of string on Go provider instances.

### Bug Fixes

- [sdk/python] - Fix regression in behaviour for `Output.from_input({})`

- [sdk/python] - Fix hanging deployments and improve error messages
  for programs with incorrect typings for output values
  [#7049](https://github.com/pulumi/pulumi/pull/7049)
