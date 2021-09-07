### Improvements

- [build] - make lint returns an accurate status code
  [#7844](https://github.com/pulumi/pulumi/pull/7844)

- [codegen/python] - Add helper function forms `$fn_output` that
  accept `Input`s, return an `Output`, and wrap the underlying `$fn`
  call. This change addreses
  [#5758](https://github.com/pulumi/pulumi/issues/) for Python,
  making it easier to compose functions/datasources with Pulumi
  resources. [#7784](https://github.com/pulumi/pulumi/pull/7784)

- [cli/about] - Add comand for debug information
  [#7817](https://github.com/pulumi/pulumi/pull/7817)

- [codegen/schema] Add a `pulumi schema check` command to validate package schemas.
  [#7865](https://github.com/pulumi/pulumi/pull/7865)

### Bug Fixes

- [sdk/python] - Fix Pulumi programs hanging when dependency graph
  forms a cycle, as when `eks.NodeGroup` declaring `eks.Cluster` as a
  parent while also depending on it indirectly via properties
  [#7887](https://github.com/pulumi/pulumi/pull/7887)

- [sdk/python] Fix a regression in Python dynamic providers introduced in #7755.

- [automation/go] Fix loading of stack settings/configs from yaml files.
  [#pulumi-kubernetes-operator/183](https://github.com/pulumi/pulumi-kubernetes-operator/issues/183)
