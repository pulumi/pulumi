### Breaking


### Improvements

- [cli] Improve diff displays during `pulumi refresh`
  [#6568](https://github.com/pulumi/pulumi/pull/6568)

- [sdk/go] Cache loaded configuration files.
  [#6576](https://github.com/pulumi/pulumi/pull/6576)

- [sdk/go] Support multiple folders in GOPATH.
  [#6228](https://github.com/pulumi/pulumi/pull/6228
  
- [sdk/nodejs] Allow `Mocks::newResource` to determine whether the created resource is a `CustomResource`.
  [#6551](https://github.com/pulumi/pulumi/pull/6551)

- [automation/*] Implement minimum version checking and add:
  - Go: `LocalWorkspace.PulumiVersion()` - [#6577](https://github.com/pulumi/pulumi/pull/6577)
  - Nodejs: `LocalWorkspace.pulumiVersion` - [#6580](https://github.com/pulumi/pulumi/pull/6580)
  - Python: `LocalWorkspace.pulumi_version` - [#6589](https://github.com/pulumi/pulumi/pull/6589)
  - Dotnet: `LocalWorkspace.PulumiVersion` - [#6590](https://github.com/pulumi/pulumi/pull/6590)

### Bug Fixes

- [sdk/python] Fix automatic venv creation
  [#6599](https://github.com/pulumi/pulumi/pull/6599)

- [automation/python] Fix Settings file save
  [#6605](https://github.com/pulumi/pulumi/pull/6605)

- [sdk/dotnet] Remove MaybeNull from Output/Input.Create to avoid spurious warnings
  [#6600](https://github.com/pulumi/pulumi/pull/6600)
