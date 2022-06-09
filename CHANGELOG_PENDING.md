### Improvements


- [codegen/go] Always chose the correct version when `respectSchemaVersion` is set.
  [#9798](https://github.com/pulumi/pulumi/pull/9798)

### Bug Fixes

- [sdk/python] Better explain the keyword arguments to create(etc)_stack.
  [#9794](https://github.com/pulumi/pulumi/pull/9794)

- [cli] Revert changes causing a panic in `pulumi destroy` that tried to operate without config files.
  [#9821](https://github.com/pulumi/pulumi/pull/9821)

- [cli] Revert to statically linked binaries on Windows and Linux,
  fixing a regression introduced in 3.34.0
  [#9816](https://github.com/pulumi/pulumi/issues/9816)
