### Improvements

- [sdk/python] - Add support for custom naming of dynamic provider resource.
  [#7633](https://github.com/pulumi/pulumi/pull/7633)

- [codegen/go] - Add helper function forms `$fnOutput` that accept
  `Input`s, return an `Output`, and wrap the underlying `$fn` call.
  This change addreses
  [#5758](https://github.com/pulumi/pulumi/issues/) for Go, making it
  easier to compose functions/datasources with Pulumi resources.
  [#7784](https://github.com/pulumi/pulumi/pull/7784)

### Bug Fixes

- [sdk/dotnet] - Fix an exception when passing an unknown `Output` to
  the `DependsOn` resource option.
  [#7762](https://github.com/pulumi/pulumi/pull/7762)
