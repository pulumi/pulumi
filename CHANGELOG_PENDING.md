### Improvements

- [auto/*] Add `--save-plan` and `--plan` options to automation API.
  [#9391](https://github.com/pulumi/pulumi/pull/9391)

- [cli] "down" is now treated as an alias of "destroy".
  [#9458](https://github.com/pulumi/pulumi/pull/9458)

- [go] Add `Composite` resource option allowing several options to be encapsulated into a "single" option.
  [#9459](https://github.com/pulumi/pulumi/pull/9459)

- [codegen] Support all [Asset and Archive](https://www.pulumi.com/docs/intro/concepts/assets-archives/) types.
  [#9463](https://github.com/pulumi/pulumi/pull/9463)

- [cli] The engine will now default resource parent to the root stack if it exists.
  [#9481](https://github.com/pulumi/pulumi/pull/9481)

### Bug Fixes

- [codegen] Ensure that plain properties are always plain.
  [#9430](https://github.com/pulumi/pulumi/pull/9430)

- [cli] Fixed some context leaks where shutdown code wasn't correctly called.
  [#9438](https://github.com/pulumi/pulumi/pull/9438)

- [cli] Do not render array diffs for unchanged elements without recorded values.
  [#9448](https://github.com/pulumi/pulumi/pull/9448)

- [auto/go] Fixed the exit code reported by `runPulumiCommandSync` to be zero if the command runs successfully. Previously it returned -2 which could lead to confusing messages if the exit code was used for other errors, such as in `Stack.Preview`.
  [#9443](https://github.com/pulumi/pulumi/pull/9443)

- [auto/go] Fixed a race condition that could cause `Preview` to fail with "failed to get preview summary".
  [#9467](https://github.com/pulumi/pulumi/pull/9467)

- [backend/filestate] - Fix a bug creating `stack.json.bak` files.
  [#9476](https://github.com/pulumi/pulumi/pull/9476)