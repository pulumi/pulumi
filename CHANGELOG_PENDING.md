
### Improvements
  
- [codegen/python,nodejs] - Emit dynamic config-getters.
  [#7447](https://github.com/pulumi/pulumi/pull/7447),  [#7530](https://github.com/pulumi/pulumi/pull/7530)

### Bug Fixes

- [sdk/dotnet] - Fix for race conditions in .NET SDK that used to
  manifest as a `KeyNotFoundException` from `WhileRunningAsync`
  [#7529](https://github.com/pulumi/pulumi/pull/7529)
- [sdk/nodejs] Fix a bug in closure serialization. 
  [#6999](https://github.com/pulumi/pulumi/pull/6999)
- Normalize cloud URL during login
  [#7544](https://github.com/pulumi/pulumi/pull/7544)
