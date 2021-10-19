### Improvements

- [codegen/dotnet] - Add helper function forms `$fn.Invoke` that
  accept `Input`s, return an `Output`, and wrap the underlying
  `$fn.InvokeAsync` call. This change addreses
  [#5758](https://github.com/pulumi/pulumi/issues/) for .NET, making
  it easier to compose functions/datasources with Pulumi resources.
  NOTE for resource providers: the generated code requires Pulumi .NET
  SDK 3.15 or higher.

  [#7899](https://github.com/pulumi/pulumi/pull/7899)
  
- [auto/dotnet] - Add `pulumi state delete` and `pulumi state unprotect` functionality
  [#8202](https://github.com/pulumi/pulumi/pull/8202)

### Bug Fixes

- [codegen/go] - Fix interaction between plain types with default values.
  [#8254](https://github.com/pulumi/pulumi/pull/8254)
