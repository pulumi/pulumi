### Improvements

- [auto/*] Add `--save-plan` and `--plan` options to automation API.
  [#9391](https://github.com/pulumi/pulumi/pull/9391)

- [cli] "down" is now treated as an alias of "destroy".
  [#9458](https://github.com/pulumi/pulumi/pull/9458)

- [go] Add `Composite` resource option allowing several options to be encapsulated into a "single" option.
  [#9459](https://github.com/pulumi/pulumi/pull/9459)

### Bug Fixes

- [codegen] - Ensure that plain properties are always plain.
  [#9430](https://github.com/pulumi/pulumi/pull/9430)

- [cli] Fixed some context leaks where shutdown code wasn't correctly called.
  [#9438](https://github.com/pulumi/pulumi/pull/9438)

- [cli] Do not render array diffs for unchanged elements without recorded values.
  [#9448](https://github.com/pulumi/pulumi/pull/9448)
  
- [auto/go] Fixed the exit code reported by `runPulumiCommandSync` to be zero if the command runs successfully. Previously it returned -2 which could lead to confusing messages if the exit code was used for other errors, such as in `Stack.Preview`.
  [#9443](https://github.com/pulumi/pulumi/pull/9443)
