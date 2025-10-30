(pcl-binding)=
# PCL binding

*Binding* is the process of verifying the structure of a [PCL](pcl) program's
[*abstract syntax tree*](pcl-lexing-parsing) (AST) (checking that it contains
blocks describing resources, outputs, and so on) before associating it with (or
"binding it to") a set of Pulumi [schema](schema), in the process checking and
resolving references between parts ("nodes") of the program and using this
information to [type check](pcl-type-checking) the program's expressions. For
instance, given the AST for a PCL program such as the following:

```hcl
resource "r" "random:index:RandomString" {
    length = 8
}

output "o" {
    value = r.result
}
```

the binding process would encompass the following tasks:

* Walking the tree to ensure that all its blocks are valid
  [*nodes*](gh-file:pulumi#pkg/codegen/pcl/program.go#L34) in a PCL program. In
  this case, the program contains two nodes: a `resource` node and an `output`
  node. As part of this, the generic notions of e.g. *labels* are refined to
  have semantic meaning:
  * In the case of a `resource` block, for instance, the labels (here `r` and
    `random:index:RandomString`) are interpreted as giving the *name* and *type*
    of the resource being defined.
  * In the case of an `output` block, the label (here `o`) is interpreted as
    giving the name of the output being exported.

* Resolving the `random` package (as referenced by the `random:index:RandomString`
  type in the `r` resource node) and loading its schema.

* Using the resolved schema to construct an object type comprising the
  `RandomString` resource's input properties and using a scope containing this
  type to [type check](pcl-type-checking) the `length` attribute.

* Adding a definition for `r` to the top-level scope so that references to `r`
  can be type checked later on. This definition will refer to an object type
  comprising the `RandomString` resource's output properties, as resolved from
  the loaded schema.

* Type checking the `value` attribute of the `output` node.

The output of binding is a *bound program*, which wraps the original [AST-level
program](pcl-lexing-parsing) with the set of resolved nodes, which in turn
reference [semantic model expressions](pcl-type-checking), and so on.
