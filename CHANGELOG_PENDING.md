### Breaking

- [automation/go] - Expose structured logging for Stack.Up/Preview/Refresh/Destroy.
  [#6436](https://github.com/pulumi/pulumi/pull/6436)

This change is marked breaking because it changes the shape of the `PreviewResult` struct.

**Before**

```go
type PreviewResult struct {
  Steps         []PreviewStep  `json:"steps"`
  ChangeSummary map[string]int `json:"changeSummary"`
}
```

**After**

```go
type PreviewResult struct {
  StdOut        string
  StdErr        string
  ChangeSummary map[apitype.OpType]int
}
```

- [automation/dotnet] Add ability to capture stderr
  [#6513](https://github.com/pulumi/pulumi/pull/6513)

This change is marked breaking because it also renames `OnOutput` to `OnStandardOutput`.

### Improvements

- [sdk/go] Add helpers to convert raw Go maps and arrays to Pulumi `Map` and `Array` inputs.
  [#6337](https://github.com/pulumi/pulumi/pull/6337)

- [sdk/go] Return zero values instead of panicing in `Index` and `Elem` methods.
  [#6338](https://github.com/pulumi/pulumi/pull/6338)

- [sdk/go] Support multiple folders in GOPATH.
  [#6228](https://github.com/pulumi/pulumi/pull/6228
  
- [sdk/nodejs] Allow `Mocks::newResource` to determine whether the created resource is a `CustomResource`.
  [#6551](https://github.com/pulumi/pulumi/pull/6551)

- [cli] Add ability to download arm64 provider plugins
  [#6492](https://github.com/pulumi/pulumi/pull/6492)

- [build] Updating Pulumi to use Go 1.16
  [#6470](https://github.com/pulumi/pulumi/pull/6470)

- [build] Adding a Pulumi arm64 binary for use on new macOS hardware.
  Please note that `pulumi watch` will not be supported on darwin/arm64 builds.
  [#6492](https://github.com/pulumi/pulumi/pull/6492)

- [automation/nodejs] - Expose structured logging for Stack.up/preview/refresh/destroy.
  [#6454](https://github.com/pulumi/pulumi/pull/6454)

- [automation/nodejs] - Add `onOutput` event handler to `PreviewOptions`.
  [#6507](https://github.com/pulumi/pulumi/pull/6507)

- [cli] Add locking support to the self-managed backends using the `PULUMI_SELF_MANAGED_STATE_LOCKING=1` environment variable.
  [#2697](https://github.com/pulumi/pulumi/pull/2697)

### Bug Fixes

- [sdk/python] Fix mocks issue when passing a resource more than once.
  [#6479](https://github.com/pulumi/pulumi/pull/6479)

- [automation/dotnet] Add ReadDiscard OperationType
  [#6493](https://github.com/pulumi/pulumi/pull/6493)

- [cli] Ensure the user has the correct access to the secrets manager before using it as part of
  `pulumi stack export --show-secrets`.
  [#6215](https://github.com/pulumi/pulumi/pull/6210)

- [sdk/go] Implement getResource in the mock monitor.
  [#5923](https://github.com/pulumi/pulumi/pull/5923)
