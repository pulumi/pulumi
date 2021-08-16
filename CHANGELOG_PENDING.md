### Improvements

- [cli] Stop printing secret value on `pulumi config set` if it looks like a secret.
  [#7327](https://github.com/pulumi/pulumi/pull/7327)

- [sdk/nodejs] Prevent Pulumi from overriding tsconfig.json options.
  [#7068](https://github.com/pulumi/pulumi/pull/7068)

- [sdk/go] - Permit declaring explicit resource dependencies via
  `ResourceInput` values.
  [#7584](https://github.com/pulumi/pulumi/pull/7584)

### Bug Fixes

- [sdk/python] - Fix program hangs when monitor becomes unavailable.
  [#7734](https://github.com/pulumi/pulumi/pull/7734)

- [sdk/python] Allow Python dynamic provider resources to be constructed outside of `__main__`. 
  [#7755](https://github.com/pulumi/pulumi/pull/7755)
