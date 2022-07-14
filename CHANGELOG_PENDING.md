### Improvements

- [cli] Display outputs during the very first preview.
  [#10031](https://github.com/pulumi/pulumi/pull/10031)

- [cli] Add Last Status to `pulumi stack ls` output.
  [#6148](https://github.com/pulumi/pulumi/pull/6148)
  
- [cli] Allow pulumi `destroy -s <stack>` if not in a Pulumi project dir
  [#9918](https://github.com/pulumi/pulumi/pull/9918)
  
- [protobuf] Pulumi protobuf messages are now namespaced under "pulumi".
  [#10074](https://github.com/pulumi/pulumi/pull/10074)

- [cli] Truncate long stack outputs
  [#9905](https://github.com/pulumi/pulumi/issues/9905)
  
- [sdk/go] Add `As*Output` methods to `AnyOutput`
  [#10085](https://github.com/pulumi/pulumi/pull/10085)

- [yaml] [Updates Pulumi YAML to v0.5.3](https://github.com/pulumi/pulumi-yaml/releases/tag/v0.5.3)

- [dotnet/codegen] code generation for csharp Pulumi programs now targets .NET 6
  [#10143](https://github.com/pulumi/pulumi/pull/10143)

### Bug Fixes

- [cli] `pulumi convert` help text is wrong
  [#9892](https://github.com/pulumi/pulumi/issues/9892)

- [go/codegen] fix error assignment when creating a new resource in generated go code
  [#10049](https://github.com/pulumi/pulumi/pull/10049)

- [cli] `pulumi convert` generates incorrect input parameter names for C#
  [#10042](https://github.com/pulumi/pulumi/issues/10042)

- [engine] Un-parent child resource when a resource is deleted during a refresh.
  [#10073](https://github.com/pulumi/pulumi/pull/10073)

- [cli] `pulumi state change-secrets-provider` now takes `--stack` into account
  [#10075](https://github.com/pulumi/pulumi/pull/10075)

- [nodejs/sdkgen] Default set `pulumi.name` in package.json to the pulumi package name.
  [#10088](https://github.com/pulumi/pulumi/pull/10088)

- [sdk/python] update protobuf library to v4 which speeds up pulumi CLI dramatically on M1 machines
  [#10063](https://github.com/pulumi/pulumi/pull/10063)

- [go] Fix panic when returning pulumi.Bool, .String, .Int, and .Float64 in the argument to
  ApplyT and casting the result to the corresponding output, e.g.: BoolOutput.
  [#10103](https://github.com/pulumi/pulumi/pull/10103)

- [engine] Fix data races discovered in CLI and Go SDK that could cause nondeterministic behavior
  or a panic.
  [#10081](https://github.com/pulumi/pulumi/pull/10081),
  [#10100](https://github.com/pulumi/pulumi/pull/10100)
