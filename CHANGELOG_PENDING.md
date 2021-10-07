### Improvements

- [codegen/nodejs] - Add helper function forms `$fnOutput` that accept
  `Input`s, return an `Output`, and wrap the underlying `$fn` call.
  This change addreses
  [#5758](https://github.com/pulumi/pulumi/issues/) for Node JS,
  making it easier to compose functions/datasources with Pulumi
  resources.
  [#8047](https://github.com/pulumi/pulumi/pull/8047)

- [sdk/dotnet] Update SDK to support the upcoming codegen feature that
  will enable functions to accept Outputs
  ([5758](https://github.com/pulumi/pulumi/issues/5758)). Specifically
  add `Pulumi.DeploymentInstance.Invoke` and remove the now redundant
  `Pulumi.Utilities.CodegenUtilities`.
  [#8142](https://github.com/pulumi/pulumi/pull/8142)

### Bug Fixes

- [codegen/go] - Use `importBasePath` before `name` if specified
  [#8159](https://github.com/pulumi/pulumi/pull/8159)
