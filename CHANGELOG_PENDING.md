### Improvements

- [cli/plugins] Warn that using GITHUB_REPOSITORY_OWNER is deprecated.
  [#10142](https://github.com/pulumi/pulumi/pull/10142)

- [dotnet/codegen] code generation for csharp Pulumi programs now targets .NET 6
  [#10143](https://github.com/pulumi/pulumi/pull/10143)

- [cli] Allow `pulumi plugin install <type> <pkg> -f <path>` to target a binary
  file or a folder.
  [#10094](https://github.com/pulumi/pulumi/pull/10094)

- [cli/config] Allow `pulumi config cp --path` between objects.
  [#10147](https://github.com/pulumi/pulumi/pull/10147)

### Bug Fixes

- [cli] Only log github request headers at log level 11.
  [#10140](https://github.com/pulumi/pulumi/pull/10140)

- [codegen/go] Support program generation, `pulumi convert` for programs that create explicit
  provider resources.
  [#10132](https://github.com/pulumi/pulumi/issues/10132)
