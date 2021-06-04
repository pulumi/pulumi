### Improvements

- [dotnet/sdk] Add get value async to output utilities.
  [#7170](https://github.com/pulumi/pulumi/pull/7170)

- [codegen] - Fix Go init.go codegen to be govet compliant.

- [codegen] - Encrypt input args for secret properties.
  [#7128](https://github.com/pulumi/pulumi/pull/7128)

### Bug Fixes

- [CLI] Fix broken venv for Python projects started from templates
  [#6624](https://github.com/pulumi/pulumi/pull/6623)
  
- [cli] - Send plugin install output to stderr, so that it doesn't
  clutter up --json, automation API scenarios, and so on.
  [#7115](https://github.com/pulumi/pulumi/pull/7115)
  
- [cli] Protect against panics when using the wrong resource type with `pulumi import`
  [#7202](https://github.com/pulumi/pulumi/pull/7202)

- [auto/nodejs] - Emit warning instead of breaking on parsing JSON events for automation API.
  [#7162](https://github.com/pulumi/pulumi/pull/7162)

- [sdk/python] Improve performance of `Output.from_input` and `Output.all` on nested objects.
  [#7175](https://github.com/pulumi/pulumi/pull/7175)

- [codegen/dotnet] Fix plain properties
  [#7180](https://github.com/pulumi/pulumi/pull/7180)

- [codegen/nodejs] Properly handle nested modules
  [#7213](https://github.com/pulumi/pulumi/pull/7213)


### Misc
- Update version of go-cloud used by Pulumi to `0.23.0`.
  [#7204](https://github.com/pulumi/pulumi/pull/7204)
