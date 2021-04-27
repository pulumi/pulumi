### Breaking Changes



### Enhancements

- [auto/go] - Provide GetPermalink for all results
  [#6875](https://github.com/pulumi/pulumi/pull/6875)

- [automation/*] Add support for getting stack outputs using Workspace
  [#6859](https://github.com/pulumi/pulumi/pull/6859)

### Bug Fixes

- [cli] Return an appropriate error when a user has not set `PULUMI_CONFIG_PASSPHRASE` nor `PULUMI_CONFIG_PASSPHRASE_FILE`
  when trying to access the Passphrase Secrets Manager
  [#6893](https://github.com/pulumi/pulumi/pull/6893)

- [sdk/python] - Fix bug in MockResourceArgs.
  [#6863](https://github.com/pulumi/pulumi/pull/6863)

- [automation/dotnet] Fix EventLogWatcher failing to read events after an exception was thrown
  [#6821](https://github.com/pulumi/pulumi/pull/6821)
  
- [automation/dotnet] Use stackName in ImportStack
  [#6858](https://github.com/pulumi/pulumi/pull/6858)

### Misc.

- [auto/*] - Bump minimum version to v3.1.0.
  [#6852](https://github.com/pulumi/pulumi/pull/6852)
