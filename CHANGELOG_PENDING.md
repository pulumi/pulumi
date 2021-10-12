### Improvements

- [sdk/nodejs] - Updates SDK to allow using any newer version of ts-node and typescript
- [automation/python] - Use `rstrip` rather than `strip` for the sake of indentation
  [#8160](https://github.com/pulumi/pulumi/pull/8160)
- [codegen/nodejs] - Add helper function forms `$fnOutput` that accept
  `Input`s, return an `Output`, and wrap the underlying `$fn` call.
  This change addreses
  [#5758](https://github.com/pulumi/pulumi/issues/5758) for NodeJS,
  making it easier to compose functions/datasources with Pulumi
  resources.
  [#8047](https://github.com/pulumi/pulumi/pull/8047)

- [sdk/dotnet] - Update SDK to support the upcoming codegen feature that
  will enable functions to accept Outputs
  ([5758](https://github.com/pulumi/pulumi/issues/5758)). Specifically
  add `Pulumi.DeploymentInstance.Invoke` and remove the now redundant
  `Pulumi.Utilities.CodegenUtilities`.
  [#8142](https://github.com/pulumi/pulumi/pull/8142)

- [cli] - Upgrade CLI to go1.17
  [#8171](https://github.com/pulumi/pulumi/pull/8171)

- [codegen/go] Register input types for schema object types.
  [#7959](https://github.com/pulumi/pulumi/pull/7959)

### Bug Fixes

- [codegen/go] - Use `importBasePath` before `name` if specified for name 
  and path.
  [#8159](https://github.com/pulumi/pulumi/pull/8159)
  [#8187](https://github.com/pulumi/pulumi/pull/8187)

- [auto/go] - Mark entire exported map as secret if key in map is secret.
  [#8179](https://github.com/pulumi/pulumi/pull/8179)
