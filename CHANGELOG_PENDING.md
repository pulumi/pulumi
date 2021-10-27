### Improvements

- [cli] Reformat error message string in `sdk/go/common/diag/errors.go`
  [#8284](https://github.com/pulumi/pulumi/pull/8284)

### Bug Fixes

- [sdk/go] - Respect implicit parents in alias resolution
  [#8288](https://github.com/pulumi/pulumi/pull/8288)

- [sdk/dotnet] - Fix a race condition when detecting exceptions in stack creation
  [#8294](https://github.com/pulumi/pulumi/pull/8294)
 - Clarify error message string in `sdk/go/common/diag/errors.go`
   [#8284](https://github.com/pulumi/pulumi/pull/8284)

- [cli] Add `--json` flag to `up`, `destroy` and `refresh`.

  Passing the `--json` flag to `up`, `destroy` and `refresh` will stream JSON events from the engine to stdout.
  For `preview`, the existing functionality of outputting a JSON object at the end of preview is maintained.
  However, the streaming output can be extended to `preview` by using the `PULUMI_ENABLE_STREAMING_JSON_PREVIEW` environment variable.

  [#8275](https://github.com/pulumi/pulumi/pull/8275)

- [codegen/go] - Interaction between the `plain` and `default` tags of a type. 
  [#8254](https://github.com/pulumi/pulumi/pull/8254)

- [sdk/dotnet] - Fix a race condition when detecting exceptions in stack creation
  [#8294](https://github.com/pulumi/pulumi/pull/8294)

- [sdk/go] - Fix regression marshaling assets/archives.
  [#8290](https://github.com/pulumi/pulumi/pull/8290)

### Miscellaneous

- [sdk/python] - Drop support for python 3.6
  [#8161](https://github.com/pulumi/pulumi/pull/8161)
