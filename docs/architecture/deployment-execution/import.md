(importing-resources)=
# Importing resources

There are a number of scenarios in which it is necessary to import existing
resources for management by Pulumi:

* Migrating manually managed resources to an infrastructure-as-code solution;
* Migrating from other infrastructure-as-code platforms to Pulumi;
* Moving resources between Pulumi stacks in cases where e.g. `pulumi state move`
  is not sufficient.

Pulumi offers two approaches for importing resources: the `import` resource
option and the `pulumi import` command. At a minimum, importing a resource
involves adding the resource's state to the destination stack's
[state](state-snapshots). Once the resource has been added to the stack, the
Pulumi CLI is able to manage the resource like any other.

## The `import` resource option

The [`import` resource
option](https://www.pulumi.com/docs/iac/concepts/options/import/) accepts the
[ID](resource-ids) of an existing resource whose state should be
[](pulumirpc.ResourceProvider.Read) and imported into the stack.

Importing a resource using the `import` resource option requires that the
desired state described by the Pulumi program for a resource being imported
matches the actual state of the resource as returned by the provider. More
precisely, given a resource `R` of type `T` with import ID `X` and set of inputs
(as specified in the program) `Iₚ`, the engine performs the following sequence
of operations:

1. Fetch the current inputs `Iₐ` and state `Sₐ` for the resource of type `T`
   with ID `X` from its provider by calling the provider's
   [](pulumirpc.ResourceProvider.Read) method. If the provider does not return a
   value for `Iₐ`, the provider does not support importing resources and the
   import fails.
2. Process the [`ignoreChanges` resource
   option](https://www.pulumi.com/docs/iac/concepts/options/ignorechanges/) by
   copying the value for any ignored input property from `Iₐ` to `Iₚ`.
3. Validate the resource's inputs and apply any programmatic defaults by passing
   `Iₚ` and `Iₐ` to the provider's [](pulumirpc.ResourceProvider.Check) method.
   Let `Iₖ` be the checked inputs; these inputs form the resource's desired
   state.
4. Check for differences between `Iₖ` and `Sₐ` by calling the provider's
   [](pulumirpc.ResourceProvider.Diff) method. If the provider reports any
   differences, the import either succeeds with a warning (in the case of a
   preview) or fails with an error (in the case of an update).

If all of these steps succeed, the user is left with a definition for `R` in
their Pulumi program that matches that in the stack's state exactly.

```mermaid
:caption: Importing a resource using the `import` resource option
:zoom:

sequenceDiagram
    participant LH as Language host
    box Engine
        participant RM as Resource monitor
        participant SG as Step generator
        participant SE as Step executor
    end
    participant P as Provider

    LH->>+RM: RegisterResourceRequest(type, name, inputs, options)
    RM->>+SG: RegisterResourceEvent(type, name, inputs, options)
    SG->>+SE: ImportStep(inputs, options)
    SE->>+P: ReadRequest(type, id)
    P->>-SE: ReadResponse(current inputs, current state)
    SE->>+P: CheckRequest(type, inputs, current inputs)
    P->>-SE: CheckResponse(inputs', failures)
    SE->>+P: DiffRequest(type, inputs', current state, options)
    P->>-SE: DiffResponse(diff)
    SE->>-RM: Done(current state)
    RM->>-LH: RegisterResourceResponse(URN, ID, current state)
```

## `pulumi import`

[`pulumi import`](https://www.pulumi.com/docs/cli/commands/pulumi_import/) is a
newer method of importing resources into a stack that also generates program
code for imported resources. `pulumi import` accepts a list of *import specs*,
where each spec comprises at minimum a [type token](urns), name, and
[ID](resource-ids), but may also specify a parent URN, provider reference, and
package version. Unlike the `import` resource option, `pulumi import` does not
insist that the desired state of the resource in the Pulumi program matches the
actual state of the resource as returned by the provider, since it is capable of
generating code to match the actual state. Given a resource `R` of type `T` with
import ID `X` and an (initiall empty) set of input properties `Iₚ`, the engine
performs the following sequence of operations:

1. Fetch the current inputs `Iₐ` and state `Sₐ` for the resource of type `T`
   with ID `X` from its provider by calling the provider's
   [](pulumirpc.ResourceProvider.Read) method. If the provider does not return a
   value for `Iₐ`, the provider does not support importing resources and the
   import fails.
2. Fetch the schema for resources of type `T` from the provider. If the provider
   is not schematized or if `T` has no schema, the import fails.
3. Copy the value of each required input property defined in the schema for `T`
   from `Iₐ` to `Iₚ`.
4. Validate the resource's inputs and apply any programmatic defaults by passing
   `Iₚ` and `Iₐ` to the provider's [](pulumirpc.ResourceProvider.Check) method.
   Let `Iₖ` be the checked inputs; these inputs form the resource's desired
   state.
5. Check for differences between `Iₖ` and `Sₐ` by calling the provider's
   [](pulumirpc.ResourceProvider.Diff) method. If the provider reports any
   differences, the values of the differing properties are copied from `Sₐ` to
   `Iₚ`. This is intended to produce the smallest valid set of inputs necessary
   to avoid diffs. This does not use a fixed-point algorithm because there is no
   guarantee that the values copied from `Sₐ` are in fact valid (state and
   inputs with the same property paths may have different types and validation
   rules) and there is no guarantee that such an algorithm would terminate (
   bridged Terraform providers, for example, have had bugs that cause persistent
   diffs, which can only be worked around with `ignoreChanges`).

If all of these steps succeed, the user is left with a definition for `R` in
their state. The Pulumi CLI then passes the inputs `Iₚ` stored in the state to
the import code generator. The import code generator converts the values present
in `Iₚ` into an equivalent [PCL](pcl) representation of `R`'s desired state,
then passes the PCL to a language-specific code generator to emit a
representation of `R`'s desired state in the language used by the destination
stack's Pulumi program. The user can then copy the generated definition into
their Pulumi program.

```mermaid
:caption: Importing a resource using the `pulumi import` CLI
:zoom:

sequenceDiagram
    participant PI as pulumi import
    participant CV as PCL converter
    participant LH as Language host
    box Engine
        participant ID as Import driver
        participant SE as Step executor
    end
    participant P as Provider

    PI->>+ID: Import(specs)
    ID->>+SE: ImportStep(inputs, options)
    SE->>+P: ReadRequest(type, id)
    P->>-SE: ReadResponse(current inputs, current state)
    SE->>+P: CheckRequest(type, inputs, current inputs)
    P->>-SE: CheckResponse(inputs', failures)
    SE->>+P: DiffRequest(type, inputs', current state, options)
    P->>-SE: DiffResponse(diff)
    SE->>-ID: Done(current state)
    ID->>-PI: Done(current state)

    PI->>+CV: Convert(current state)
    CV->>-PI: Done(PCL definitions)

    PI->>+LH: GenerateProgram(PCL definitions)
    LH->>-PI: Done(generated resource definitions)
```

### Limitations and challenges

The primary challenge in generating appropriate code for `pulumi import` lies in
determining exactly what the input values for a particular resource should be.
In many providers, it is not necessarily possible to accurately recover a
resource's inputs from its state. This observation led to the diff-oriented
approach described above, where the importer begins with an extremely minimal
set of inputs and attempts to derive the actual inputs from the results of a
call to the provider's [](pulumirpc.ResourceProvider.Diff) method.
Unfortunately, the results are not always satisfactory, and the relatively small
set of inputs present in the generated code can make it difficult for users to
determine what inputs they *actually* need to pass to the resource to describe
its current state.

A few other approaches might be:

* Emit no properties at all and instead just output appropriate resource
  constructor calls. This will almost always emit code that does not compile or
  run, as nearly every resource has at least one required property.
* Copy the value for every input property present in a resource's schema from
  its state. This risks emitting code that does not compile due to differences
  in types between inputs and outputs, and also risks emitting code that does
  not work at runtime due to conflicts between mutually-exclusive properties
  (which are unfortunately common for Terraform-based resources, for example).

It is likely that some mix of approaches is necessary in order to arrive at a
satisfactory solution, as none of the above solutions seems universally
"correct".
