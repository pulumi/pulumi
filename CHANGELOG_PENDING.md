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

### Improvements

- [sdk/go] Add helpers to convert raw Go maps and arrays to Pulumi `Map` and `Array` inputs.
  [#6337](https://github.com/pulumi/pulumi/pull/6337)

- [sdk/go] Return zero values instead of panicing in `Index` and `Elem` methods.
  [#6338](https://github.com/pulumi/pulumi/pull/6338)

- [sdk/go] Support multiple folders in GOPATH.
  [#6228](https://github.com/pulumi/pulumi/pull/6228

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

### Bug Fixes

- [sdk/python] Fix mocks issue when passing a resource more than once.
  [#6479](https://github.com/pulumi/pulumi/pull/6479)
