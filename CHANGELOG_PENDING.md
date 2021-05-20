### Bug Fixes

- [cli] Fix a regression caused by [#6893](https://github.com/pulumi/pulumi/pull/6893) that stopped stacks created
  with empty passphrases from completing successful pulumi commands when loading the passphrase secrets provider.
  [#6976](https://github.com/pulumi/pulumi/pull/6976)
- [backend] Add gzip compression to filestate backend.
  Compression can be disabled via `PULUMI_SELF_MANAGED_STATE_NO_GZIP`.
  [7086](https://github.com/pulumi/pulumi/pull/7086)
