### Improvements

- [cli] - When running `pulumi new https://github.com/name/repo`, check 
  for branch `main` if branch `master` doesn't exist.
  [#8463](https://github.com/pulumi/pulumi/pull/8463)

- [codegen/python] - Program generator now uses `fn_output` forms where
  appropriate, simplifying auto-generated examples.
  [#8433](https://github.com/pulumi/pulumi/pull/8433)

- [codegen/go] - Program generator now uses fnOutput forms where
  appropriate, simplifying auto-generated examples.
  [#8431](https://github.com/pulumi/pulumi/pull/8431)

- [codegen/dotnet] - Program generator now uses `Invoke` forms where
  appropriate, simplifying auto-generated examples.
  [#8432](https://github.com/pulumi/pulumi/pull/8432)

### Bug Fixes

- [cli/nodejs] - Allow specifying the tsconfig file used in Pulumi.yaml.
  [#8452](https://github.com/pulumi/pulumi/pull/8452)

- [codegen/nodejs] - Respect default values in Pulumi object types.
  [#8400](https://github.com/pulumi/pulumi/pull/8400)

- [sdk/python] - Correctly handle version checking python virtual environments.
  [#8465](https://github.com/pulumi/pulumi/pull/8465)

- [cli] - Catch expected errors in stacks with filestate backends.
  [#8455](https://github.com/pulumi/pulumi/pull/8455)
