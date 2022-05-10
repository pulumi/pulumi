### Improvements
- [cli] Updated gocloud.dev to 0.24.0, which adds support for using AWS SDK v2. It enables users to pass an AWS profile to the `awskms` secrets provider url (i.e. `awskms://alias/pulumi?awssdk=v2&region=eu-west-1&profile=aws-prod`)
  [#9536](https://github.com/pulumi/pulumi/pull/9536)

- [cli] Add `--stack` to `pulumi about`.
  [#9518](https://github.com/pulumi/pulumi/pull/9518)

### Bug Fixes
