### Improvements

- [automation] - Add force flag for RemoveStack in workspace
  [#7523](https://github.com/pulumi/pulumi/pull/7523)
### Bug Fixes

- [cli] - Properly parse Git remotes with periods or hyphens.
  [#7386](https://github.com/pulumi/pulumi/pull/7386)

- [codegen/python] - Recover good IDE completion experience over
  module imports that was compromised when introducing the lazy import
  optimization.
  [#7487](https://github.com/pulumi/pulumi/pull/7487)

- [sdk/python] - Use `Sequence[T]` instead of `List[T]` for several `Resource`
  parameters.
  [#7698](https://github.com/pulumi/pulumi/pull/7698)

- [auto/nodejs] - Fix a case where inline programs could exit with outstanding async work.
  [#7704](https://github.com/pulumi/pulumi/pull/7704)

- [auto/nodejs] - Use ESlint instead of TSlint
  [#7719](https://github.com/pulumi/pulumi/pull/7719)