### Improvements

### Bug Fixes

- [cli] Fixed some context leaks where shutdown code wasn't correctly called.
  [#9438](https://github.com/pulumi/pulumi/pull/9438)

- [auto/go] Fixed the exit code reported by `runPulumiCommandSync` to be zero if the command runs successfully. Previously it returned -2 which could lead to confusing messages if the exit code was used for other errors, such as in `Stack.Preview`.
  [#9443](https://github.com/pulumi/pulumi/pull/9443)
