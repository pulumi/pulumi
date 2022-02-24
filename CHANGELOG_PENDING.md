### Improvements

- [codegen/go] - Add GenerateProgramWithOpts function to enable configurable codegen options.
  [#8997](https://github.com/pulumi/pulumi/pull/8997)

- [cli] -  Enabled dot spinner for non-interactive mode
  [#8996](https://github.com/pulumi/pulumi/pull/8996)

- [sdk] - Add `RetainOnDelete` as a resource option.
  [#8746](https://github.com/pulumi/pulumi/pull/8746)

- [cli] - Adding `completion` as an alias to `gen-completion`
  [#9006](https://github.com/pulumi/pulumi/pull/9006)

- [cli/plugins] Add support for downloading plugin from private GitHub releases.
  [#8944](https://github.com/pulumi/pulumi/pull/8944)

### Bug Fixes

- [sdk/go] - Normalize merge behavior for `ResourceOptions`, inline
  with other SDKs. See https://github.com/pulumi/pulumi/issues/8796 for more
  details.
  [#8882](https://github.com/pulumi/pulumi/pull/8882)

- [sdk/go] - Correctly parse GoLang version.
  [#8920](https://github.com/pulumi/pulumi/pull/8920)

- [sdk/go] - Fix git initialization in git_test.go
  [#8924](https://github.com/pulumi/pulumi/pull/8924)

- [cli/go] - Fix git initialization in util_test.go
  [#8924](https://github.com/pulumi/pulumi/pull/8924)

- [sdk/nodejs] - Fix nodejs function serialization module path to comply with package.json
  exports if exports is specified.
  [#8893](https://github.com/pulumi/pulumi/pull/8893)

- [cli/python] - Parse a larger subset of PEP440 when guessing Pulumi package versions.
  [#8958](https://github.com/pulumi/pulumi/pull/8958)

- [sdk/nodejs] - Allow disabling TypeScript typechecking
  [#8981](https://github.com/pulumi/pulumi/pull/8981)

- [cli/backend] - Revert a change to file state locking that was causing stacks to stay locked.
  [#8995](https://github.com/pulumi/pulumi/pull/8995)

- [cli] - Fix passphrase secrets provider prompting.
  [#8986](https://github.com/pulumi/pulumi/pull/8986)

- [cli] - Fix an assert when replacing protected resources.
  [#9004](https://github.com/pulumi/pulumi/pull/9004)

- [sdk/nodejs] - Fix Node `fs.rmdir` DeprecationWarning for Node JS 15.X+
  [#9044](https://github.com/pulumi/pulumi/pull/9044)
