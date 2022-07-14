### Improvements

- [cli/plugins] Warn that using GITHUB_REPOSITORY_OWNER is deprecated.
  [#10142](https://github.com/pulumi/pulumi/pull/10142)

### Bug Fixes

- [cli] Only log github request headers at log level 11.
  [#10140](https://github.com/pulumi/pulumi/pull/10140)

- [sdk/go] `config.Encrypter` and `config.Decrypter` interfaces now
  require explicit `Context`. This is a minor breaking change to the
  SDK. The change fixes parenting of opentracing spans that decorate
  calls to the Pulumi Service crypter.

  [#10037](https://github.com/pulumi/pulumi/pull/10037)
