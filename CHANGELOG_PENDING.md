### Improvements

- [cli] Allow `pulumi refresh` to interactively resolve pending creates.
  [#10394](https://github.com/pulumi/pulumi/pull/10394)

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

- [sdk/nodejs] Added stack truncation to `SyntaxError` in nodejs.
  [#10465](https://github.com/pulumi/pulumi/pull/10465)

- [sdk/python] Makes global SETTINGS values context-aware to not leak state between Pulumi programs running in parallel
  [#10402](https://github.com/pulumi/pulumi/pull/10402)

- [sdk/python] Makes global ROOT, CONFIG, _SECRET_KEYS ContextVars to not leak state between parallel inline Pulumi programs
  [#10472](https://github.com/pulumi/pulumi/pull/10472)

- [sdk/go] Improve error messages for `StackReference`s
  [#10477](https://github.com/pulumi/pulumi/pull/10477)
  
- [sdk/dotnet] Added `Output.CreateSecret<T>(Output<T> value)` to set the secret bit on an output value.
  [#10467](https://github.com/pulumi/pulumi/pull/10467)

- [policy] `pulumi policy publish` now takes into account `.gitignore` files higher in the file tree.
  [#10493](https://github.com/pulumi/pulumi/pull/10493)

- [sdk/go] enable direct compilation via `go build`(set `PULUMI_GO_USE_RUN=true` to opt out)
  [#10375](https://github.com/pulumi/pulumi/pull/10375)

- [sdk/go] Go SDK now properly outputs concise diagnostic error logs
  [#10347](https://github.com/pulumi/pulumi/pull/10347)

### Bug Fixes

- [codegen/go] Fix StackReference codegen.
  [#10260](https://github.com/pulumi/pulumi/pull/10260

- [engine/backends]: Fix bug where File state backend failed to apply validation to stack names, resulting in a panic.
  [#10417](https://github.com/pulumi/pulumi/pull/10417)

- [cli] Fix VCS detection for domains other than .com and .org.
  [#10415](https://github.com/pulumi/pulumi/pull/10415)

- [codegen/go] Fix incorrect method call for reading floating point values from configuration.
  [#10445](https://github.com/pulumi/pulumi/pull/10445)

- [engine]: HTML characters are no longer escaped in JSON output.
  [#10440](https://github.com/pulumi/pulumi/pull/10440)

- [codegen/go] Ensure consistency between go docs information and package name
  [#10452](https://github.com/pulumi/pulumi/pull/10452)

- [auto/go] Clone non-default branches (and tags).
  [#10285](https://github.com/pulumi/pulumi/pull/10285)

- [cli] Fixes `survey.v1` panics in Terminal UI introduced in
  [#10130](https://github.com/pulumi/pulumi/issues/10130) in v3.38.0.
  [#10475](https://github.com/pulumi/pulumi/pull/10475)
  
- [codegen/ts] Fix non-pulumi owned provider import alias.
  [#10447](https://github.com/pulumi/pulumi/pull/10447)

- [cli] Fixes panics on repeat Ctrl+C invocation during long-running updates
  [#10489](https://github.com/pulumi/pulumi/pull/10489)

- [cli] Improve Windows reliability with dependency update to ssh-agent
  [#10486](https://github.com/pulumi/pulumi/pull/10486)

- [sdk/{dotnet,nodejs,python}] Dynamic providers and automation API will not trigger a firewall
  permission prompt, will only accept network requests via loopback address.
  [#10498](https://github.com/pulumi/pulumi/pull/10498)
  [#10502](https://github.com/pulumi/pulumi/pull/10502)
  [#10503](https://github.com/pulumi/pulumi/pull/10503)

- [cli] Fix `pulumi console` command to follow documented behavior in help message/docs.
  [#10509](https://github.com/pulumi/pulumi/pull/10509)
