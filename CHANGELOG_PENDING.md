### Improvements

- [cli/plugins] Warn that using GITHUB_REPOSITORY_OWNER is deprecated.
  [#10142](https://github.com/pulumi/pulumi/pull/10142)

- [cli] Allow `pulumi plugin install <type> <pkg> -f <path>` to target a binary
  file or a folder.
  [#10094](https://github.com/pulumi/pulumi/pull/10094)

- [cli/config] Allow `pulumi config cp --path` between objects.
  [#10147](https://github.com/pulumi/pulumi/pull/10147)

### Bug Fixes

- [cli] Only log github request headers at log level 11.
  [#10140](https://github.com/pulumi/pulumi/pull/10140)

- [sdk/go] `config.Encrypter` and `config.Decrypter` interfaces now
  require explicit `Context`. This is a minor breaking change to the
  SDK. The change fixes parenting of opentracing spans that decorate
  calls to the Pulumi Service crypter.

  [#10037](https://github.com/pulumi/pulumi/pull/10037)

- [codegen/go] Support program generation, `pulumi convert` for programs that create explicit
  provider resources.
  [#10132](https://github.com/pulumi/pulumi/issues/10132)

- [sdk/go] Remove the `AsName` and `AsQName` asserting functions.
  [#10156](https://github.com/pulumi/pulumi/pull/10156)
  
- [python] PULUMI_PYTHON_CMD is checked for deciding what python binary to use in a virtual environment.
  [#10155](https://github.com/pulumi/pulumi/pull/10155)

- [cli] Reduced the noisiness of `pulumi new --help` by replacing the list of available templates to just the number.
  [#10164](https://github.com/pulumi/pulumi/pull/10164)