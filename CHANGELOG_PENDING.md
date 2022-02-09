### Improvements

### Bug Fixes

- [sdk/go] - Normalize merge behavior for `ResourceOptions`, inline
  with other SDKs. See https://github.com/pulumi/pulumi/issues/8796 for more
  details.
  [#8882](https://github.com/pulumi/pulumi/pull/8882)

- [sdk/go] - Correctly parse GoLang version.
  [#8920](https://github.com/pulumi/pulumi/pull/8920)

- [sdk/nodejs] - Fix nodejs function serialization module path to comply with package.json 
  exports if exports is specified.
  [#8893](https://github.com/pulumi/pulumi/pull/8893)
