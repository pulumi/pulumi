### Improvements
* Adds CI detector for Buildkite [#7933](https://github.com/pulumi/pulumi/pull/7933)

- [cli] - Add `--exclude-protected` flag to `pulumi destroy`.
  [#8359](https://github.com/pulumi/pulumi/pull/8359)

- [cli] Adding the ability to use `pulumi org set [name]` to set a default org
  to use when creating a stacks in the Pulumi Service backend or self-hosted Service
  [#8352](https://github.com/pulumi/pulumi/pull/8352)

- [schema] Add IsOverlay option to disable codegen for particular types
  [#8338](https://github.com/pulumi/pulumi/pull/8338)
  [#8425](https://github.com/pulumi/pulumi/pull/8425)

- [sdk/dotnet] - Marshal output values.
  [#8316](https://github.com/pulumi/pulumi/pull/8316)

- [sdk/python] - Unmarshal output values in component provider.
  [#8212](https://github.com/pulumi/pulumi/pull/8212)

- [sdk/nodejs] - Unmarshal output values in component provider.
  [#8205](https://github.com/pulumi/pulumi/pull/8205)

### Bug Fixes

- [engine] - Compute dependents correctly during targeted deletes.
  [#8360](https://github.com/pulumi/pulumi/pull/8360)
