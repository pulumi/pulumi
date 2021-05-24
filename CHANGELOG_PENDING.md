### Improvements

- [dotnet/sdk] - Use source context with serilog
  [#7095](https://github.com/pulumi/pulumi/pull/7095)

- [auto/dotnet] - Make StackDeployment.FromJsonString public
  [#7067](https://github.com/pulumi/pulumi/pull/7067)

- [sdk/python] - Generated SDKs may now be installed from in-tree source.
  [#7097](https://github.com/pulumi/pulumi/pull/7097)

### Bug Fixes

- [auto/nodejs] - Fix an intermittent bug in parsing JSON events 
  [#7032](https://github.com/pulumi/pulumi/pull/7032) 

- [auto/dotnet] - Fix deserialization of CancelEvent in .NET 5
  [#7051](https://github.com/pulumi/pulumi/pull/7051)

- Temporarily disable warning when a secret config is read as a non-secret
  [#7129](https://github.com/pulumi/pulumi/pull/7129)
