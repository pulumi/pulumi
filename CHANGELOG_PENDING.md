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

### Bug Fixes

- [engine/backends]: Fix bug where File state backend failed to apply validation to stack names, resulting in a panic.
  [#10417](https://github.com/pulumi/pulumi/pull/10417)

- [codegen/go] Fix incorrect method call for reading floating point values from configuration.
  [#10445](https://github.com/pulumi/pulumi/pull/10445)

- [codegen/ts] Fix non-pulumi owned provider import alias.
  [#10447](https://github.com/pulumi/pulumi/pull/10447)