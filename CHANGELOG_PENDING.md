### Breaking Changes



### Enhancements

- [auto/go] - Provide GetPermalink for all results
  [#6875](https://github.com/pulumi/pulumi/pull/6875)

- [automation/*] Add support for getting stack outputs using Workspace
  [#6859](https://github.com/pulumi/pulumi/pull/6859)

- [automation/*] Optionally skip Automation API version check
  [#6882](https://github.com/pulumi/pulumi/pull/6882)

  The version check can be skipped by passing a non-empty value to the `PULUMI_AUTOMATION_API_SKIP_VERSION_CHECK` environment variable.

- [codegen/python] Lazy module import to improve CLI startup performance
  [#6827](https://github.com/pulumi/pulumi/pull/6827)

- [auto/go,nodejs] Add UserAgent to update/pre/refresh/destroy options.
  [#6935](https://github.com/pulumi/pulumi/pull/6935)

### Bug Fixes

- [cli] Return an appropriate error when a user has not set `PULUMI_CONFIG_PASSPHRASE` nor `PULUMI_CONFIG_PASSPHRASE_FILE`
  when trying to access the Passphrase Secrets Manager
  [#6893](https://github.com/pulumi/pulumi/pull/6893)
  
- [cli] Prevent against panic when using a ResourceReference as a program output
  [#6962](https://github.com/pulumi/pulumi/pull/6962)

- [sdk/python] - Fix bug in MockResourceArgs.
  [#6863](https://github.com/pulumi/pulumi/pull/6863)

- [automation/dotnet] Fix EventLogWatcher failing to read events after an exception was thrown
  [#6821](https://github.com/pulumi/pulumi/pull/6821)

- [automation/dotnet] Use stackName in ImportStack
  [#6858](https://github.com/pulumi/pulumi/pull/6858)
  
- [automation/go] Improve autoError message formatting
  [#6924](https://github.com/pulumi/pulumi/pull/6924)

- [sdk/python] Address issues when using resource subclasses.
  [#6890](https://github.com/pulumi/pulumi/pull/6890)

- [sdk/python] Fix type-related regression on Python 3.6.
  [#6942](https://github.com/pulumi/pulumi/pull/6942)

- [sdk/python] Don't error when a dict input value has a mismatched type annotation.
  [#6949](https://github.com/pulumi/pulumi/pull/6949)

### Misc.

- [auto/dotnet] Bump YamlDotNet to 11.1.1
  [#6915](https://github.com/pulumi/pulumi/pull/6915)

- [sdk/dotnet] Enable deterministic builds
  [#6917](https://github.com/pulumi/pulumi/pull/6917)

- [auto/*] - Bump minimum version to v3.1.0.
  [#6852](https://github.com/pulumi/pulumi/pull/6852)
