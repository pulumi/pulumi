### Improvements

- [build] - make lint returns an accurate status code
  [#7844](https://github.com/pulumi/pulumi/pull/7844)

- [codegen/python] - Add helper function forms `$fn_output` that
  accept `Input`s, return an `Output`, and wrap the underlying `$fn`
  call. This change addreses
  [#5758](https://github.com/pulumi/pulumi/issues/) for Python,
  making it easier to compose functions/datasources with Pulumi
  resources. [#7784](https://github.com/pulumi/pulumi/pull/7784)

- [codegen/schema] Add a `pulumi schema check` command to validate package schemas.
  [#7865](https://github.com/pulumi/pulumi/pull/7865)

### Bug Fixes
