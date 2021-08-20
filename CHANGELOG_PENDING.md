### Improvements


- [codegen/{go,python}] - Add helper function forms `$fnOutput` that
  accept `Input`s, return an `Output`, and wrap the underlying `$fn`
  call. This change addreses
  [#5758](https://github.com/pulumi/pulumi/issues/) for Go and Python,
  making it easier to compose functions/datasources with Pulumi
  resources. [#7784](https://github.com/pulumi/pulumi/pull/7784)

### Bug Fixes
