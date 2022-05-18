### Improvements

- [cli] Add `--stack` to `pulumi about`.
  [#9518](https://github.com/pulumi/pulumi/pull/9518)

- [sdk/dotnet] Bumped several dependency versions to avoid pulling packages with known vulnerabilities.
  [#9591](https://github.com/pulumi/pulumi/pull/9591)
  
- [cli] Updated gocloud.dev to 0.24.0, which adds support for using AWS SDK v2. It enables users to pass an AWS profile to the `awskms` secrets provider url (i.e. `awskms://alias/pulumi?awssdk=v2&region=eu-west-1&profile=aws-prod`)
  [#9590](https://github.com/pulumi/pulumi/pull/9590)

### Bug Fixes

- [cli] The PULUMI_CONFIG_PASSPHRASE environment variables can be empty, this is treated different to being unset.
  [#9568](https://github.com/pulumi/pulumi/pull/9568)

- [codegen/python] Fix importing of enum types from other packages.
  [#9579](https://github.com/pulumi/pulumi/pull/9579)

- [cli] Fix panic in `pulumi console` when no stack is selected.
  [#9594](https://github.com/pulumi/pulumi/pull/9594)


- [auto/python] - Fix text color argument being ignored during stack     operations.
  [#9615](https://github.com/pulumi/pulumi/pull/9615)