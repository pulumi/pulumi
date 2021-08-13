### Improvements

- [cli] Stop printing secret value on `pulumi config set` if it looks like a secret.
  [#7327](https://github.com/pulumi/pulumi/pull/7327)

- [sdk/nodejs] Prevent Pulumi from overriding tsconfig.json options.
  [#7068](https://github.com/pulumi/pulumi/pull/7068)

### Bug Fixes

- [sdk/python] Allow Python dynamic provider resources to be constructed outside of `__main__`. 
  [#7755](https://github.com/pulumi/pulumi/pull/7755)