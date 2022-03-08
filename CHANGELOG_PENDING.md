### Improvements
 - [area/cli] - Implemented `state rename` command [#9098](https://github.com/pulumi/pulumi/pull/9098)

- [cli/plugins] `pulumi plugin install` can now look up the latest version of plugins on GitHub releases.
  [#9012](https://github.com/pulumi/pulumi/pull/9012)

- [cli/backend] - `pulumi cancel` is now supported for the file state backend.
  [#9100](https://github.com/pulumi/pulumi/pull/9100)

- [cli/import] - Code generation in `pulumi import` can now be disabled with the `--generate-code=false` flag.
  [#9141](https://github.com/pulumi/pulumi/pull/9141)

### Bug Fixes

- [sdk/python] - Fix build warnings. See
  [#9011](https://github.com/pulumi/pulumi/issues/9011) for more details.
  [#9139](https://github.com/pulumi/pulumi/pull/9139)
