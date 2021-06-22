**Please Note:** Release v3.5.0 failed in our build pipeline so will be rebuilt with a new tag of v3.5.1

### Improvements

- [cli] Stops printing secret value on `pulumi config set` if it looks like a secret.
  [#7327](https://github.com/pulumi/pulumi/pull/7327)

- [dotnet/sdk] Support microsoft logging extensions with inline programs
  [#7117](https://github.com/pulumi/pulumi/pull/7117)

- [cli] - Added support for passing custom paths that need
  to be watched by the `pulumi watch` command.
  [#7115](https://github.com/pulumi/pulumi/pull/7247)


### Bug Fixes

- [sdk/dotnet] - Fix swallowed nested exceptions with inline program so they correctly bubble to consumer
  [#7323](https://github.com/pulumi/pulumi/pull/7323)
  
- [sdk/go] - Specify known when creating outputs for construct.
  [#7343](https://github.com/pulumi/pulumi/pull/7343)
