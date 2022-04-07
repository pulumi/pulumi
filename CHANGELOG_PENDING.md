### Improvements

- [cli] - Installing of language specific project dependencies is now managed by the language plugins, not the pulumi cli.
  [#9294](https://github.com/pulumi/pulumi/pull/9294)

- [cli] Warn users when there are pending operations but proceed with deployment
  [#9293](https://github.com/pulumi/pulumi/pull/9293)

- [cli] Display more useful diffs for secrets that are not primitive values
  [#9351](https://github.com/pulumi/pulumi/pull/9351)

- [cli] - Warn when `additionalSecretOutputs` is used to mark the `id` property as secret.
  [#9360](https://github.com/pulumi/pulumi/pull/9360)

- [providers] - gRPC providers can now support an Attach method for debugging. The engine will attach to providers listed in the PULUMI_DEBUG_PROVIDERS environment variable. This should be of the form "providerName:port,otherProvider:port".
  [#8979](https://github.com/pulumi/pulumi/pull/8979)

### Bug Fixes

