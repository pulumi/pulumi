### Improvements

- [CLI] Adding the ability to use `pulumi org set [name]` to set a default org
  to use when creating a stacks in the Pulumi Service backend or Self -hosted Service
  [#8352](https://github.com/pulumi/pulumi/pull/8352)

- [schema] Add IsOverlay option to disable codegen for particular types
  [#8338](https://github.com/pulumi/pulumi/pull/8338)

- [sdk/dotnet] - Marshal output values.
  [#8316](https://github.com/pulumi/pulumi/pull/8316)

- [sdk/nodejs] - Always use `"module": "commonjs"` TypeScript option when compiling `pulumi.ts`
  []()

### Bug Fixes

- [engine] - Compute dependents correctly during targeted deletes.
  [#8360](https://github.com/pulumi/pulumi/pull/8360)
