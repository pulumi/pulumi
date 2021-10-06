### Improvements

- [codegen/dotnet] - Add helper function forms `$fn.Invoke` that
  accept `Input`s, return an `Output`, and wrap the underlying
  `$fn.InvokeAsync` call. This change addreses
  [#5758](https://github.com/pulumi/pulumi/issues/) for .NET, making
  it easier to compose functions/datasources with Pulumi resources.
  NOTE for resource providers: the generated code requires Pulumi .NET
  SDK 3.13 or higher.

  [#7899](https://github.com/pulumi/pulumi/pull/7899)

- [cli] - Differentiate in-progress actions by bolding output.
  [#7918](https://github.com/pulumi/pulumi/pull/7918)

- [CLI] Adding the ability to set `refresh: always` in an options object at a Pulumi.yaml level
  to allow a user to be able to always refresh their derivative stacks by default
  [#8071](https://github.com/pulumi/pulumi/pull/8071)

### Bug Fixes

- [codegen/go] - Fix generation of cyclic struct types.
  [#8049](https://github.com/pulumi/pulumi/pull/8049)

- [codegen/nodejs] - Fix type literal generation by adding
  disambiguating parens; previously nested types such as arrays of
  unions and optionals generated type literals that were incorrectly
  parsed by TypeScript precedence rules.

  NOTE for providers: using updated codegen may result in API changes
  that break existing working programs built against the older
  (incorrect) API declarations.

  [#8116](https://github.com/pulumi/pulumi/pull/8116)

- [auto/go] - Fix --target / --replace args
  [#8109](https://github.com/pulumi/pulumi/pull/8109)
  
- [sdk/python] - Fix deprecation warning when using python 3.10
  [#8129](https://github.com/pulumi/pulumi/pull/8129)
