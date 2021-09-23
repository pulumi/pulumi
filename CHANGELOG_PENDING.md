### Improvements

- [codegen/nodejs] - Add helper function forms `$fnOutput` that accept
  `Input`s, return an `Output`, and wrap the underlying `$fn` call.
  This change addreses
  [#5758](https://github.com/pulumi/pulumi/issues/) for Node JS,
  making it easier to compose functions/datasources with Pulumi
  resources.
  [#8047](https://github.com/pulumi/pulumi/pull/8047)


### Bug Fixes

- [automation/python] Fix a bug in printing `Stack` if no program is provided.
  [#8032](https://github.com/pulumi/pulumi/pull/8032)

- [codegen/schema] Revert #7938
  [#8035](https://github.com/pulumi/pulumi/pull/8035)

- [codegen/nodejs] Correctly determine imports for functions.
  [#8038](https://github.com/pulumi/pulumi/pull/8038)
