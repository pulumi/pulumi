### Improvements
- [cli] - Implemented `state rename` command [#9098](https://github.com/pulumi/pulumi/pull/9098)

- [cli/plugins] `pulumi plugin install` can now look up the latest version of plugins on GitHub releases.
  [#9012](https://github.com/pulumi/pulumi/pull/9012)

- [cli/backend] - `pulumi cancel` is now supported for the file state backend.
  [#9100](https://github.com/pulumi/pulumi/pull/9100)

### Bug Fixes

- [cli/backend] - Fixed an issue with non-atomicity when saving file state stacks.
  [#9122](https://github.com/pulumi/pulumi/pull/9122)