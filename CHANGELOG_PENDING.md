### Improvements

- [cli] Split invoke request protobufs, as monitors and providers take different arguments.
  [#9323](https://github.com/pulumi/pulumi/pull/9323)

### Bug Fixes

- [cli/plugin] - Dynamic provider binaries will now be found even if pulumi/bin is not on $PATH.
  [#9396](https://github.com/pulumi/pulumi/pull/9396)