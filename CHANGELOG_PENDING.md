### Improvements

- [cli] Split invoke request protobufs, as monitors and providers take different arguments.
  [#9323](https://github.com/pulumi/pulumi/pull/9323)

### Bug Fixes

- [cli/plugin] - Dynamic provider binaries will now be found even if pulumi/bin is not on $PATH.
  [#9396](https://github.com/pulumi/pulumi/pull/9396)

- [sdk/go] - Fail appropriatly for `config.Try*` and `config.Require*` where the
  key is present but of the wrong type.
  [#9407](https://github.com/pulumi/pulumi/pull/9407)
