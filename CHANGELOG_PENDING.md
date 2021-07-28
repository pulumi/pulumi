### Improvements

- [sdk/go] - Add stack output helpers for numeric types.
  [#7410](https://github.com/pulumi/pulumi/pull/7410)

- [sdk/python] - Permit `Input[Resource]` values in `depends_on`.
  [#7559](https://github.com/pulumi/pulumi/pull/7559)

- [backend/filestate] - Allow pulumi stack ls to see all stacks regardless of passphrase.
  [#7660](https://github.com/pulumi/pulumi/pull/7660)


### Bug Fixes

- [sdk/{go,python,nodejs}] - Rehydrate provider resources in `Construct`.
  [#7624](https://github.com/pulumi/pulumi/pull/7624)
  
- [engine] - Include children when targeting components.
  [#7605](https://github.com/pulumi/pulumi/pull/7605)

- [cli] - Restore passing log options to providers when `--logflow` is specified
  https://github.com/pulumi/pulumi/pull/7640

- [sdk/nodejs] - Fix `pulumi up --logflow` causing Node multi-lang components to hang
  [#7644](https://github.com/pulumi/pulumi/pull/)

- [sdk/{dotnet,python,nodejs}] - Set the package on DependencyProviderResource.
  [#7630](https://github.com/pulumi/pulumi/pull/7630)
