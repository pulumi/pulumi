### Improvements

- [cli/plugins] Warn that using GITHUB_REPOSITORY_OWNER is deprecated.
  [#10142](https://github.com/pulumi/pulumi/pull/10142)

- [config/cp] Allow `pulumi config cp --path` between objects.
  [#10147](https://github.com/pulumi/pulumi/pull/10147)

### Bug Fixes

- [cli] Only log github request headers at log level 11.
  [#10140](https://github.com/pulumi/pulumi/pull/10140)

- [codegen/go] Support program generation, `pulumi convert` for programs that create explicit
  provider resources.
  [#10132](https://github.com/pulumi/pulumi/issues/10132)
