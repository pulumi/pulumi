### Improvements

- [cli] `pulumi logout` now prints a confirmation message that it logged out.
  [#9641](https://github.com/pulumi/pulumi/pull/9641)

- [cli/backend] Add gzip compression to filestate backend. Compression can be enabled via `PULUMI_SELF_MANAGED_STATE_GZIP=true`. Special thanks to @awoimbee for the initial PR.
  [#9610](https://github.com/pulumi/pulumi/pull/9610)

- [sdk/nodejs] Lazy load inflight context to remove module import side-effect.
  [#9375](https://github.com/pulumi/pulumi/issues/9375)

- [cli] The engine will now default resource parent to the root stack if it exists.
  [#9481](https://github.com/pulumi/pulumi/pull/9481)

### Bug Fixes

- [sdk/python] Fix spurious diffs causing an "update" on resources created by dynamic providers.
  [#9656](https://github.com/pulumi/pulumi/pull/9656)

- [cli] Engine now correctly tracks that resource reads have unique URNs.
  [#9516](https://github.com/pulumi/pulumi/pull/9516)
