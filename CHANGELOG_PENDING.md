### Improvements

- [cli] Updated to the latest version of go-git.
  [#10330](https://github.com/pulumi/pulumi/pull/10330)

- [sdk]
   - merge python error message and traceback into single error message.
   [#10348](https://github.com/pulumi/pulumi/pull/10348)

- [sdk/python] Support optional default parameters in pulumi.Config
  [#10344](https://github.com/pulumi/pulumi/pull/10344)

### Bug Fixes

- [cli] Paginate template options
  [#10130](https://github.com/pulumi/pulumi/issues/10130)

- [sdk/dotnet] Fix serialization of non-generic list types.
  [#10277](https://github.com/pulumi/pulumi/pull/10277)

- [codegen/nodejs] Correctly reference external enums.
  [#10286](https://github.com/pulumi/pulumi/pull/10286)

- [sdk/python] Support deeply nested protobuf objects.
  [#10284](https://github.com/pulumi/pulumi/pull/10284)

- Revert [Remove api/renewLease from startup crit path](pulumi/pulumi#10168) to fix #10293.
  [#10294](https://github.com/pulumi/pulumi/pull/10294)

- [codegen/go] Remove superfluous double forward slash from doc.go
  [#10317](https://github.com/pulumi/pulumi/pull/10317)

- [cli/plugins] Don't retry plugin downloads that failed due to local file errors.
  [#10341](https://github.com/pulumi/pulumi/pull/10341)

- [dotnet] Set environment exit code during `Deployment.RunAsync` in case users don't bubble it the program entry point themselves
  [#10217](https://github.com/pulumi/pulumi/pull/10217)
  