### Improvements

- [cli] - Installing of language specific project dependencies is now managed by the language plugins, not the pulumi cli.
  [#9294](https://github.com/pulumi/pulumi/pull/9294)

- [cli] Warn users when there are pending operations but proceed with deployment
  [#9293](https://github.com/pulumi/pulumi/pull/9293)

- [cli] Display more useful diffs for secrets that are not primitive values
  [#9351](https://github.com/pulumi/pulumi/pull/9351)

- [cli] - Warn when `additionalSecretOutputs` is used to mark the `id` property as secret.
  [#9360](https://github.com/pulumi/pulumi/pull/9360)

- [cli] Display richer diffs for texutal property values.
  [#9376](https://github.com/pulumi/pulumi/pull/9376)

### Bug Fixes

- [codegen/node] - Fix an issue with escaping deprecation messages.
  [#9371](https://github.com/pulumi/pulumi/pull/9371)

- [cli] - StackReferences will now correctly use the service bulk decryption end point.
  [#9373](https://github.com/pulumi/pulumi/pull/9373)

- [cli/plugin] - Dynamic provider binaries will now be found even if pulumi/bin is not on $PATH.
  [#9396](https://github.com/pulumi/pulumi/pull/9396)