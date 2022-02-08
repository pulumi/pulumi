### Improvements

- [codegen/go] - Implement go type conversions for optional string, boolean, int, and float32
  arguments, and changes our behavior for optional spilling from variable declaration hoisting to
  instead rewrite as calls to these functions. Fixes #8821.
  [#8839](https://github.com/pulumi/pulumi/pull/8839)

- [sdk/go] Added new conversion functions for Read methods on resources exported at the top level
  of the Pulumi sdk. They are `StringRef`, `BoolRef`, `IntRef`, and `Float64Ref`. They are used for
  creating a pointer to the type they name, e.g.: StringRef takes `string` and returns `*string`.
  Data source methods which take optional strings, bools, ints, and float64 values can be set to
  the return value of these functions. These functions will appear in generated programs as well as
  future docs updates.

- [sdk/nodejs] - Fix resource plugins advertising a `pluginDownloadURL` not being downloaded. This
  should allow resource plugins published via boilerplates to find and consume plugins published
  outside the registry. See: https://github.com/pulumi/pulumi/issues/8890 for the tracking issue to
  document this feature.

- [cli] Experimental support for update plans. Only enabled when PULUMI_EXPERIMENTAL is
  set. This enables preview to save a plan of what the engine expects to happen in a file
  with --save-plan. That plan can then be read in by up with --plan and is used to ensure
  only the expected operations happen.
  [#8448](https://github.com/pulumi/pulumi/pull/8448)

- [cli] - Support wildcards for `pulumi up --target <urn>` and similar commands.
  [#8883](https://github.com/pulumi/pulumi/pull/8883).

- [cli/import] - The import command now takes an extra argument --properties to instruct the engine which
  properties to use for the import. This can be used to import resources which the engine couldn't automaticly
  infer the correct property set for.
  [#8846](https://github.com/pulumi/pulumi/pull/8846)

## Bug Fixes

- [codegen] - Correctly handle third party resource imports.
  [#8861](https://github.com/pulumi/pulumi/pull/8861)

- [sdk/dotnet] - Normalize merge behavior for ComponentResourceOptions, inline
  with other SDKs. See https://github.com/pulumi/pulumi/issues/8796 for more
  details.
  [#8838](https://github.com/pulumi/pulumi/pull/8838)

- [codegen/nodejs] - Respect compat modes when referencing external types.
  [#8850](https://github.com/pulumi/pulumi/pull/8850)

- [cli] The engine will allow a resource to be replaced if either it's old or new state
  (or both) is not protected.
  [#8873](https://github.com/pulumi/pulumi/pull/8873)
