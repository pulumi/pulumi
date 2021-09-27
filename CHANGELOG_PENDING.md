### Improvements

- [codegen/dotnet] - Add helper function forms `$fn.Invoke` that
  accept `Input`s, return an `Output`, and wrap the underlying
  `$fn.InvokeAsync` call. This change addreses
  [#5758](https://github.com/pulumi/pulumi/issues/) for .NET, making
  it easier to compose functions/datasources with Pulumi resources.
  NOTE for resource providers: the generated code requires Pulumi .NET
  SDK 3.13 or higher.

  [#7899](https://github.com/pulumi/pulumi/pull/7899)

- [CLI] Enable output values in the engine by default.
  [#8014](https://github.com/pulumi/pulumi/pull/8014)

### Bug Fixes

- [automation/python] Fix a bug in printing `Stack` if no program is provided.
  [#8032](https://github.com/pulumi/pulumi/pull/8032)

- [codegen/schema] Revert #7938
  [#8035](https://github.com/pulumi/pulumi/pull/8035)

- [codegen/nodejs] Correctly determine imports for functions.
  [#8038](https://github.com/pulumi/pulumi/pull/8038)

- [codegen/go] - Fix resolution of enum naming collisions.
  [#7985](https://github.com/pulumi/pulumi/pull/7985)

- [sdk/{nodejs,python}] - Fix errors when testing remote components with mocks.
  [#8053](https://github.com/pulumi/pulumi/pull/8053)

- [codegen/nodejs] - Fix generation of provider enum with environment variables.
  [#8051](https://github.com/pulumi/pulumi/pull/8051)
