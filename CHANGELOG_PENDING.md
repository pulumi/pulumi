### Breaking Changes

- [sdk/nodejs] Enable nodejs dynamic provider caching by default on program side.
  [#6704](https://github.com/pulumi/pulumi/pull/6704)

- [CLI] Standardize the `--stack` flag to *not* set the stack as current (i.e. setStack=false) across CLI commands.
  [#6300](https://github.com/pulumi/pulumi/pull/6300)

- [sdk/cli] Bump version of Pulumi CLI and SDK to v3
  [#6554](https://github.com/pulumi/pulumi/pull/6554)

- [sdk/go] Simplify `Apply` method options to reduce binary size
  [#6607](https://github.com/pulumi/pulumi/pull/6607)

- [Automation/*] All operations use `--stack` to specify the stack instead of running `select stack` before the operation.
  [#6300](https://github.com/pulumi/pulumi/pull/6300)

- [Automation/go] Moving go automation API package from sdk/v2/go/x/auto -> sdk/v2/go/auto
  [#6518](https://github.com/pulumi/pulumi/pull/6518)

- [Automation/nodejs] Moving NodeJS automation API package from sdk/nodejs/x/automation -> sdk/nodejs/automation
  [#6518](https://github.com/pulumi/pulumi/pull/6518)

- [Automation/python] Moving Python automation API package from pulumi.x.automation -> pulumi.automation
  [#6518](https://github.com/pulumi/pulumi/pull/6518)

- [Automation/go] Moving go automation API package from sdk/v2/go/x/auto -> sdk/v2/go/auto
  [#6518](https://github.com/pulumi/pulumi/pull/6518)

- [sdk/python] Improved dict key translation support (3.0-based providers will opt-in to the improved behavior)
  [#6695](https://github.com/pulumi/pulumi/pull/6695)

- [CLI] Remove `pulumi history` command. This was previously deprecated and replaced by `pulumi stack history`
  [#6724](https://github.com/pulumi/pulumi/pull/6724)

### Enhancements


### Bug Fixes