### Breaking


### Improvements

- [sdk/nodejs] Add support for multiple V8 VM contexts in closure serialization.
  [#6648](https://github.com/pulumi/pulumi/pull/6648)

- [sdk/nodejs] Add provider side caching for dynamic provider deserialization
  [#6657](https://github.com/pulumi/pulumi/pull/6657)

- [automation/dotnet] Expose structured logging
  [#6572](https://github.com/pulumi/pulumi/pull/6572)

- [cli] Support full fidelity YAML round-tripping
  - Strip Byte-order Mark (BOM) from YAML configs during load. - [#6636](https://github.com/pulumi/pulumi/pull/6636)
  - Swap out YAML parser library - [#6642](https://github.com/pulumi/pulumi/pull/6642)

- [sdk/python] Ensure all async tasks are awaited prior to exit.
  [#6606](https://github.com/pulumi/pulumi/pull/6606)

### Bug Fixes

- [sdk/nodejs] Fix error propagation in registerResource and other resource methods.
  [#6644](https://github.com/pulumi/pulumi/pull/6644)

- [automation/python] Fix passing of additional environment variables.
  [#6639](https://github.com/pulumi/pulumi/pull/6639)
  
- [sdk/python] Make exceptions raised by calls to provider functions (e.g. data sources) catchable.
  [#6504](https://github.com/pulumi/pulumi/pull/6504)
