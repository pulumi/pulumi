### Improvements


- [sdk/dotnet] - Update SDK to support the upcoming codegen feature that
  will enable functions to accept Outputs
  ([5758](https://github.com/pulumi/pulumi/issues/5758)). Specifically
  add `Pulumi.DeploymentInstance.Invoke` and remove the now redundant
  `Pulumi.Utilities.CodegenUtilities`.
  [#8142](https://github.com/pulumi/pulumi/pull/8142)

- [cli] - Upgrade CLI to go1.17
  [#8171](https://github.com/pulumi/pulumi/pull/8171)

### Bug Fixes
