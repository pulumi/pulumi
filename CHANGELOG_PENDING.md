### Improvements

- [sdk/python] - Permit `Input[Resource]` values in `depends_on`.
  [#7559](https://github.com/pulumi/pulumi/pull/7559)

### Bug Fixes

- [sdk/{go,python,nodejs}] - Rehydrate provider resources in `Construct`.
  [#7624](https://github.com/pulumi/pulumi/pull/7624)

- [sdk/nodejs] - Fix `pulumi up --logflow` causing Node multi-lang components to hang
  [#7644](https://github.com/pulumi/pulumi/pull/)

- [sdk/{dotnet,python,nodejs}] - Set the package on DependencyProviderResource.
  [#7630](https://github.com/pulumi/pulumi/pull/7630)
