### Improvements

- [cli] - Differentiate in-progress actions by bolding output.
  [#7918](https://github.com/pulumi/pulumi/pull/7918)

- [CLI] Adding the ability to set `refresh: always` in an options object at a Pulumi.yaml level
  to allow a user to be able to always refresh their derivative stacks by default
  [#8071](https://github.com/pulumi/pulumi/pull/8071)

### Bug Fixes

- [codegen/go] - Fix generation of cyclic struct types.
  [#8049](https://github.com/pulumi/pulumi/pull/8049)
