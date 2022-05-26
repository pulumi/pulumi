### Improvements

- [cli] `pulumi logout` now prints a confirmation message that it logged out.
  [#9641](https://github.com/pulumi/pulumi/pull/9641)

- [cli/backend] Add gzip compression to filestate backend. Compression can be enabled via `PULUMI_SELF_MANAGED_STATE_GZIP=true`. Special thanks to @awoimbee for the initial PR.
  [#9610](https://github.com/pulumi/pulumi/pull/9610)

- [sdk/nodejs] Lazy load inflight context to remove module import side-effect.
  [#9375](https://github.com/pulumi/pulumi/issues/9375)

### Bug Fixes

- [sdk/python] Fix spurious diffs causing an "update" on resources created by dynamic providers.
  [#9656](https://github.com/pulumi/pulumi/pull/9656)

- [sdk/python] Do not depend on the children of remote component resources.
  [#9665](https://github.com/pulumi/pulumi/pull/9665)

- [codegen/nodejs] Emit the "package.json".pulumi.server as "server" correctly. Previously, "pluginDownloadURL" was emitted but never read.
  [#9662](https://github.com/pulumi/pulumi/pull/9662)

- [cli] Fix panic in `pulumi console` when no stack is selected.
  [#9594](https://github.com/pulumi/pulumi/pull/9594)

- [cli] Engine now correctly tracks that resource reads have unique URNs.
  [#9516](https://github.com/pulumi/pulumi/pull/9516)

- [sdk/python] Fixed bug in automation API that invoked Pulumi with malformed arguments.
  [#9607](https://github.com/pulumi/pulumi/pull/9607)

- [cli/backend] Fix a panic in the filestate backend when renaming history files.
  [#9673](https://github.com/pulumi/pulumi/pull/9673)

- [sdk/proto] Inline dockerfile used to generate protobuf code.
  [#9700](https://github.com/pulumi/pulumi/pull/9700)