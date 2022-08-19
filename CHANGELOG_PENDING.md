### Improvements

- [cli] Clarify highlighting of confirmation text in `confirmPrompt`.
  [#10413](https://github.com/pulumi/pulumi/pull/10413)

- [provider/python]: Improved exception display. The traceback is now shorter and it always starts with user code.  
  [#10336](https://github.com/pulumi/pulumi/pull/10336)

- [sdk/python] Update PyYAML to 6.0

- [cli/watch] `pulumi watch` now uses relies on a program built on [`watchexec`](https://github.com/watchexec/watchexec)
  to implement recursive file watching, improving performance and cross-platform compatibility.
  This `pulumi-watch` program is now included in releases.
  [#10213](https://github.com/pulumi/pulumi/issues/10213)

- [codegen] Reduce time to execute `pulumi convert` and some YAML programs, depending on providers used, by up to 3 seconds.
  [#10444](https://github.com/pulumi/pulumi/pull/10444)

### Bug Fixes

- [codegen/go] Fix StackReference codegen.
  [#10260](https://github.com/pulumi/pulumi/pull/10260

- [engine/backends]: Fix bug where File state backend failed to apply validation to stack names, resulting in a panic.
  [#10417](https://github.com/pulumi/pulumi/pull/10417)

- [codegen/go] Fix incorrect method call for reading floating point values from configuration.
  [#10445](https://github.com/pulumi/pulumi/pull/10445)

- [engine]: HTML characters are no longer escaped in JSON output.
  [#10440](https://github.com/pulumi/pulumi/pull/10440)

- [codegen/go] Ensure consistency between go docs information and package name
  [#10452](https://github.com/pulumi/pulumi/pull/10452)
