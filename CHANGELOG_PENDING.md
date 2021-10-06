### Improvements

- [cli] - Differentiate in-progress actions by bolding output.
  [#7918](https://github.com/pulumi/pulumi/pull/7918)

- [CLI] Adding the ability to set `refresh: always` in an options object at a Pulumi.yaml level
  to allow a user to be able to always refresh their derivative stacks by default
  [#8071](https://github.com/pulumi/pulumi/pull/8071)

- [sdk/dotnet] Update SDK to support the upcoming codegen feature that
  will enable functions to accept Outputs
  ([5758](https://github.com/pulumi/pulumi/issues/5758)). Specifically
  add `Pulumi.DeploymentInstance.Invoke` and remove the now redundant
  `Pulumi.Utilities.CodegenUtilities`.
  [#8142](https://github.com/pulumi/pulumi/pull/8142)

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
