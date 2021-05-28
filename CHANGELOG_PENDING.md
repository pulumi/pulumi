### Improvements


  
- [codegen] - Encrypt input args for secret properties.
  [#7128](https://github.com/pulumi/pulumi/pull/7128)

### Bug Fixes

- [cli] - Send plugin install output to stderr, so that it doesn't
  clutter up --json, automation API scenarios, and so on.
  [#7115](https://github.com/pulumi/pulumi/pull/7115)
- [backend] Add gzip compression to filestate backend.
  Compression can be enabled via `PULUMI_SELF_MANAGED_STATE_GZIP=true`.
