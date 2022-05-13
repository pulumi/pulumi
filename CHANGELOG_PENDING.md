### Improvements

- [cli] Add `--stack` to `pulumi about`.
  [#9518](https://github.com/pulumi/pulumi/pull/9518)

- [sdk/dotnet] Bumped several dependency versions to avoid pulling packages with known vulnerabilities.
  [#9591](https://github.com/pulumi/pulumi/pull/9591)

### Bug Fixes

- [cli] The PULUMI_CONFIG_PASSPHRASE environment variables can be empty, this is treated different to being unset.
  [#9568](https://github.com/pulumi/pulumi/pull/9568)
  
- [codegen/python] Fix importing of enum types from other packages.
  [#9579](https://github.com/pulumi/pulumi/pull/9579)