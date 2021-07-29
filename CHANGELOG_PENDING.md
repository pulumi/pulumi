### Improvements

### Bug Fixes

- [cli] - Respect provider aliases
  [#7166](https://github.com/pulumi/pulumi/pull/7166)

- [cli] - `pulumi stack ls` now returns all accessible stacks (removing
  earlier cap imposed by the httpstate backend).
  [#3620](https://github.com/pulumi/pulumi/issues/3620)

- [sdk/go] - Fix panics caused by logging from `ApplyT`, affecting
  `pulumi-docker` and potentially other providers
  [#7661](https://github.com/pulumi/pulumi/pull/7661)

- [sdk/python] - Handle unknown results from methods.
  [#7677](https://github.com/pulumi/pulumi/pull/7677)

