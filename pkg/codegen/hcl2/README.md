# PCL syntax and type system

[PCL](pcl)'s syntax is a subset of that expressible using
[HCL](https://github.com/hashicorp/hcl). Its type system is largely an extension
of the [*HCL syntax-agnostic information
model*](https://github.com/hashicorp/hcl/blob/v2.3.0/spec.md). The
`pkg/codegen/hcl2` package exposes code for working with PCL programs at the
level of parsed *syntax* and type-checked *models*.

(pcl-lexing-parsing)=
## Lexing and parsing

The `pkg/codegen/hcl2/syntax` package exports types and functions for parsing
PCL source into an *abstract syntax tree* (AST) using the existing [`hclsyntax`
package](https://pkg.go.dev/github.com/hashicorp/hcl/v2/hclsyntax). At a high
level, a PCL program's syntax tree is structured as follows:

* A *program* consists of a set of
  [*`File`s*](gh-file:pulumi#pkg/codegen/hcl2/syntax/parser.go#L25). Each file
  corresponds to some source file, and has a *body* representing that file's
  content.
* A *body* comprises any number of *attributes* and *blocks*.
* A *block* has a *type* and a (possibly empty) set of *labels*, as well as a
  *body* of its own.
* An *attribute* has a *name* and an associated *expression*.
* An *expression* may be any of a number of supported kinds of expressions
  (literals such as strings and numbers; unary and binary operations over child
  expressions such as negation, addition, etc.; references to other attributes
  or blocks; and so on).

As an example, suppose we have a single file, `main.pp`, with the following
content:

```hcl
resource "r" "pkg:index:Resource" {
    name = "r"
}

x = { r = r }

output "o" {
    value = r.name
}
```

The syntax tree for this program would look as follows:

* A *file* named `main.pp`, with a *body* containing the following children:
  * A *block* of *type* `resource` with a list of *labels* comprising the
    strings `"r"` and `"pkg:index:Resource"`, and a *body* containing the
    following children:
    * An *attribute* named `name` with an *expression* representing the string
      literal `"r"`.
  * An *attribute* named `x` with an *expression* representing an object literal
    with a single key-value pair, mapping the key `r` to the *expression* `r`.
  * A *block* of *type* `output` with a list of *labels* comprising the string
    `"o"`, and a *body* containing the following children:
    * An *attribute* named `value` with the *expression* `r.name`.

At the syntactic level, a PCL program is no more than a piece of HCL syntax.
Notions of "resources", "outputs", and so on, do not exist. In order to, for
instance, assign *semantics* to a piece of PCL syntax (e.g. "the expression
`r.name` refers to the `name` attribute of the `r` resource"), or to decide
whether or not some PCL program is in some sense "well-formed", we need to
[*bind*](pcl-binding) the program to produce a *semantic model*. The part of
binding that can be performed at the level of the syntax tree itself (that is,
without understanding higher-level concepts such as resources or outputs) is
known as [*type checking*](pcl-type-checking).

(pcl-type-checking)=
## Type checking

The `pkg/codegen/hcl2/model` package exports types and functions for *type
checking* parsed HCL syntax to produce a semantic *model*. Specifically, it
provides functionality for type checking *expressions*, since these are the only
parts of the syntax that can be reasoned about without coupling the syntax to
higher-level concepts such as resources and outputs. Type checking thus
comprises a part of the wider [binding](pcl-binding) process, whereby a program
that is being bound is type checked, with the type checker receiving as input a
*scope* that helps it reason about the types of e.g. references it encounters in
expressions. In this way, a reference can be understood to e.g. have an object
type without having to couple the type checker to how a resource's type is an
object consisting of its output properties.

This package's types mirror (and typically wrap) the types of the AST, adding
semantic information such as [data types](pcl-type-system) and resolved
references. So for instance, a `model.LiteralValueExpression`, which corresponds
to a literal value such as the string `"foo"` or the number `42`, contains:

* a reference to the `hclsyntax.LiteralValueExpr` underpinning it in the syntax
  tree;
* an evaluated `cty.Value` representing the value of the literal; and
* a `model.Type` representing the datatype of the literal.

The [`cty` package](https://pkg.go.dev/github.com/zclconf/go-cty/cty) is used to
represent values.

(pcl-type-system)=
### Type system

This section covers the aforementioned extensions PCL's type system makes to the
[*HCL syntax-agnostic information
model*](https://github.com/hashicorp/hcl/blob/v2.3.0/spec.md).

#### Types

##### Primitive types

PCL adds the *int* primitive type. An *int* is an arbitrary-precision integer
value. Implementations *must* make full-precision values available to consumer
applications for interpretation into any suitable integer representation.
Implementations may in practice implement *int*s with limited precision so long
as the following constraints are met:

* Integers are represented with at least 256 bits.
* An error is produced if an integer value given in source cannot be represented
  precisely.

Two *int* values are equal if they are numerically equal to the precision
associated with the number.

Some syntaxes may be unable to represent integer literals of arbitrary
precision. This must be defined in the syntax specification as part of its
description of mapping numeric literals to HCL values.

##### Structural types

PCL adds *union* types as a kind of structural type. A *union* type is
constructed of a set of types, and is assignable from any type that is
assignable to one of its element types.

A *union* type is traversed by traversing each of its element types. The result
of the traversal is the *union* of the results of the traversals that succeed.
When traversing a *union* with an element type of *none*, the traversal of
*none* successfully results in *none*; this allows a traversal of an optional
value to return an optional value of the appropriate type.

##### Eventual types

PCL adds two *eventual type kinds*, *promise* and *output*. These types represent
values that are only available asynchronously, and can be used by applications
that produce such values to more accurately track which values are available
promptly and which are not.

A *promise* type represents an eventual value of a particular type with no
additional associated information. A *promise* type is assignable from itself or
from its element type. Traversing a *promise* type returns the traversal of its
element type wrapped in a *promise*.

An *output* type represents an eventual value of a particular type that carries
additional application-specific information. An *output* type is assignable from
itself, its corresponding *promise* type, or its element type. Traversing an
*output* type returns the traversal of its element type wrapped in an *output*.

##### Null values and *none*

PCL includes a first-class representation for the null value, the *none* type.
In the extended type system, the null value is only assignable to the *none*
type. Optional values of type *T* are represented by a *union* of *T* and
*none*.

#### Conversions

##### Primitive type conversions

Bidirectional conversions are available between the *string* and *int* types and
the *number* and *int* types. Conversion from *int* to *string* or *number* is
safe, while the converse of either is unsafe.

##### Collection and structural type conversions

Conversion from a type *T* to a *union* type is permitted if there is a
conversion from *T* to at least one of the *union*'s element types. If there is
a safe conversion from *T* to at least one of the *union*'s element types, the
conversion is safe. Otherwise, the conversion is unsafe.

##### Eventual type conversions

Conversion from a type *T* to a promise with element type *U* is permitted if
*T* is a promise with element type *V* where *V* is convertible to *U* or if *T*
is convertible to *U*. The safety of this conversion depends on the safety of
the conversion from *V* or *T* to *U*.

Conversion from a type *T* to an output with element type *U* is permitted if
*T* is an output or promise with element type *V* where *V* is convertible to
*U* or if T is convertible to *U*. The safety of this conversion depends on the
safety of the conversion from *V* or *T* to *U*.

#### Unification

The *int* type unifies with *number* by preferring *number*, and unifies with
*string* by preferring *string*.

Two *union* types unify by producing a new *union* type whose elements are the
concatenation of those of the two input types.

A *union* type unifies with another type by producing a new *union* whose element
types are the unification of the other type with each of the input *union*'s
element types.

A *promise* type unifies with an *output* type by producing a new *output* type
whose element type is the unification of the *output* type's element type and
the *promise* type's element types.

Two *promise* types unify by producing a new *promise* type whose element type
is the unification of the element types of the two *promise* types.

Two *output* types unify by producing a new *output* type whose element type is
the unification of the element types of the two *output* types.
