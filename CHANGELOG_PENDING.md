### Improvements

- [cli/plugins] Warn that using GITHUB_REPOSITORY_OWNER is deprecated.
  [#10142](https://github.com/pulumi/pulumi/pull/10142)

- [cli] Allow `pulumi plugin install <type> <pkg> -f <path>` to target a binary
  file or a folder.
  [#10094](https://github.com/pulumi/pulumi/pull/10094)

### Bug Fixes

- [cli] Only log github request headers at log level 11.
  [#10140](https://github.com/pulumi/pulumi/pull/10140)

- [codegen/go] Support program generation, `pulumi convert` for programs that create explicit
  provider resources.
  [#10132](https://github.com/pulumi/pulumi/issues/10132)
