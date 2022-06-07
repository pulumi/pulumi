### Improvements

- [codegen/go] Handle long and complicated traversals in a nicer way.
  [#9726](https://github.com/pulumi/pulumi/pull/9726)

- [cli] Allow pulumi `destroy -s <stack>` if not in a Pulumi project dir
  [#9613](https://github.com/pulumi/pulumi/pull/9613)

- [cli] Plugins will now shut themselves down if they can't contact the engine that started them.
  [#9735](https://github.com/pulumi/pulumi/pull/9735)

- [cli/engine] The engine will emit a warning of a key given in additional secret outputs doesn't match any of the property keys on the resources.
  [#9750](https://github.com/pulumi/pulumi/pull/9750)
  
- [sdk/go] Add `CompositeInvoke` function, like `Composite` but for `InvokeOption`.
  [#9752](https://github.com/pulumi/pulumi/pull/9752)

### Bug Fixes

- [sdk/nodejs] Fix a crash due to dependency cycles from component resources.
  [#9683](https://github.com/pulumi/pulumi/pull/9683)

- [cli/about] Make `pulumi about` aware of the YAML and Java runtimes.
  [#9745](https://github.com/pulumi/pulumi/pull/9745)

- [cli/engine] Fix a panic deserializing resource plans without goals.
  [#9749](https://github.com/pulumi/pulumi/pull/9749)

- [cli/engine] Provide a sorting for plugins of equivalent version.
  [#9761](https://github.com/pulumi/pulumi/pull/9761)

- [cli/backend] Fix degraded performance in filestate backend
  [#9777](https://github.com/pulumi/pulumi/pull/9777)

