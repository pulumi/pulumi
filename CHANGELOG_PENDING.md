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

- [codegen/go] - Remove `ResourcePtr` types from generated SDKs. Besides being
  unnecessary--`Resource` types already accommodate `nil` to indicate the lack of
  a value--the implementation of `Ptr` types for resources was incorrect, making
  these types virtually unusable in practice.
  [#8449](https://github.com/pulumi/pulumi/pull/8449)

### Bug Fixes

- [codegen/typescript] - Respect default values in Pulumi object types.
  [#8400](https://github.com/pulumi/pulumi/pull/8400)

- [sdk/python] - Correctly handle version checking python virtual environments.
  [#8465](https://github.com/pulumi/pulumi/pull/8465)

- [cli] - Catch expected errors in stacks with filestate backends.
  [#8455](https://github.com/pulumi/pulumi/pull/8455)
