### Improvements

- [cli] - Differentiate in-progress actions by bolding output.
  [#7918](https://github.com/pulumi/pulumi/pull/7918)

- [cli] Add the ability to set `refresh: always` in an options object at a Pulumi.yaml level
  to allow a user to be able to always refresh their derivative stacks by default
  [#8071](https://github.com/pulumi/pulumi/pull/8071)

- [cli] Add `pulumi schema generate-sdk`. This command generates language-specific SDKs from a package schema.
  [#7876](https://github.com/pulumi/pulumi/pull/7876)

### Bug Fixes

- [codegen/go] - Fix generation of cyclic struct types.
  [#8049](https://github.com/pulumi/pulumi/pull/8049)

- [sdk/go] - Fix --target / --replace args
  [#8109](https://github.com/pulumi/pulumi/pull/8109)
