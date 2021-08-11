### Improvements

- [cli] - Fix the preview experience for unconfigured providers. Rather than returning the
  inputs of a resource managed by an unconfigured provider as its outputs, the engine will treat all outputs as unknown. Most
  programs will not be affected by these changes: in general, the only programs that will
  see differences are programs that:

      1. pass unknown values to provider instances
      2. use these provider instances to manage resources
      3. pass values from these resources to resources that are managed by other providers

  These kinds of programs are most common in scenarios that deploy managed Kubernetes
  clusters and Kubernetes apps within the same program, then flow values from those apps
  into other resources.
   
  The legacy behavior can be re-enabled by setting the `PULUMI_LEGACY_PROVIDER_PREVIEW` to
  a truthy value (e.g. `1`, `true`, etc.).

  [#7560](https://github.com/pulumi/pulumi/pull/7560)

- [automation] - Add force flag for RemoveStack in workspace
  [#7523](https://github.com/pulumi/pulumi/pull/7523)

### Bug Fixes

- [cli] - Properly parse Git remotes with periods or hyphens.
  [#7386](https://github.com/pulumi/pulumi/pull/7386)

- [codegen/python] - Recover good IDE completion experience over
  module imports that was compromised when introducing the lazy import
  optimization.
  [#7487](https://github.com/pulumi/pulumi/pull/7487)

- [sdk/python] - Use `Sequence[T]` instead of `List[T]` for several `Resource`
  parameters.
  [#7698](https://github.com/pulumi/pulumi/pull/7698)

- [auto/nodejs] - Fix a case where inline programs could exit with outstanding async work.
  [#7704](https://github.com/pulumi/pulumi/pull/7704)

- [sdk/nodejs] - Use ESlint instead of TSlint
  [#7719](https://github.com/pulumi/pulumi/pull/7719)