### Improvements
- [cli] - Implemented `state rename` command [#9098](https://github.com/pulumi/pulumi/pull/9098)

- [cli/plugins] `pulumi plugin install` can now look up the latest version of plugins on GitHub releases.
  [#9012](https://github.com/pulumi/pulumi/pull/9012)

- [cli/backend] - `pulumi cancel` is now supported for the file state backend.
  [#9100](https://github.com/pulumi/pulumi/pull/9100)

- [language/dotnet] - updated Pulumi dotnet packages to use grpc-dotnet instead of grpc [#9149](https://github.com/pulumi/pulumi/pull/9149)

### Bug Fixes

- [sdk/python] - Fix build warnings. See
  [#9011](https://github.com/pulumi/pulumi/issues/9011) for more details.
  [#9139](https://github.com/pulumi/pulumi/pull/9139)

- [cli/backend] - Fixed an issue with non-atomicity when saving file state stacks.
  [#9122](https://github.com/pulumi/pulumi/pull/9122)
